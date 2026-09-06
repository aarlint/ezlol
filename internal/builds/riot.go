package builds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/aarlint/ezlol/internal/ddragon"
)

// platformToRegion maps platform routing (na1) to regional routing (americas).
var platformToRegion = map[string]string{
	"na1": "americas", "br1": "americas", "la1": "americas", "la2": "americas",
	"euw1": "europe", "eun1": "europe", "tr1": "europe", "ru": "europe", "me1": "europe",
	"kr": "asia", "jp1": "asia",
	"oc1": "sea", "ph2": "sea", "sg2": "sea", "th2": "sea", "tw2": "sea", "vn2": "sea",
}

// Progress is the compiler's observable state.
type Progress struct {
	Running       bool       `json:"running"`
	Phase         string     `json:"phase"`
	Queue         int        `json:"queue"`
	Players       int        `json:"players"`
	MatchesQueued int        `json:"matchesQueued"`
	MatchesDone   int        `json:"matchesDone"`
	Target        int        `json:"target"`
	Requests      int        `json:"requests"`
	Errors        int        `json:"errors"`
	LastError     string     `json:"lastError,omitempty"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
}

// CompilerConfig configures the Riot match compiler.
type CompilerConfig struct {
	APIKey        string
	Platform      string   // na1
	MatchesPerRun int      // stop after this many new matches
	Timeline      bool     // fetch timelines (2x requests, needed for build order and skills)
	Tiers         []string // challenger, grandmaster, master
	Queue         int      // 420 ranked solo, 450 ARAM
}

// Compiler pulls high-elo ranked matches and folds them into a Store.
type Compiler struct {
	cfg    CompilerConfig
	store  *Store
	dd     *ddragon.Store
	log    *slog.Logger
	http   *http.Client
	limit  *limiter
	mu     sync.Mutex
	prog   Progress
	cancel context.CancelFunc
}

// NewCompiler wires a compiler. It does nothing until Start.
func NewCompiler(cfg CompilerConfig, store *Store, dd *ddragon.Store, log *slog.Logger) *Compiler {
	if cfg.MatchesPerRun <= 0 {
		cfg.MatchesPerRun = 200
	}
	if cfg.Queue == 0 {
		cfg.Queue = 420
	}
	if len(cfg.Tiers) == 0 {
		cfg.Tiers = []string{"challenger", "grandmaster"}
	}
	return &Compiler{
		cfg:   cfg,
		store: store,
		dd:    dd,
		log:   log,
		http:  &http.Client{Timeout: 30 * time.Second},
		// Development key limits: 20 req / 1 s and 100 req / 2 min.
		limit: newLimiter(20, time.Second, 100, 2*time.Minute),
	}
}

// HasKey reports whether a Riot API key is configured.
func (c *Compiler) HasKey() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg.APIKey != ""
}

// Platform returns the configured platform.
func (c *Compiler) Platform() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg.Platform
}

// Configure updates key, platform and batch size for the next run.
func (c *Compiler) Configure(apiKey, platform string, matchesPerRun int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg.APIKey = apiKey
	if platform != "" {
		c.cfg.Platform = platform
	}
	if matchesPerRun > 0 {
		c.cfg.MatchesPerRun = matchesPerRun
	}
}

// Progress returns a snapshot.
func (c *Compiler) Progress() Progress {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.prog
}

func (c *Compiler) update(f func(p *Progress)) {
	c.mu.Lock()
	f(&c.prog)
	c.mu.Unlock()
}

// Start begins a compile run in the background for the given queue (0 = default).
// Returns an error if one is running.
func (c *Compiler) Start(parent context.Context, queue int) error {
	if !c.HasKey() {
		return errors.New("Riot API key not set (Settings)")
	}
	c.mu.Lock()
	if c.prog.Running {
		c.mu.Unlock()
		return errors.New("compile already running")
	}
	if queue != 0 {
		c.cfg.Queue = queue
	}
	ctx, cancel := context.WithCancel(parent)
	c.cancel = cancel
	now := time.Now()
	c.prog = Progress{Running: true, Phase: "starting", Queue: c.cfg.Queue, Target: c.cfg.MatchesPerRun, StartedAt: &now}
	c.mu.Unlock()
	go c.run(ctx)
	return nil
}

// Stop cancels a running compile.
func (c *Compiler) Stop() {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Unlock()
}

func (c *Compiler) run(ctx context.Context) {
	defer func() {
		now := time.Now()
		c.update(func(p *Progress) { p.Running = false; p.FinishedAt = &now; p.Phase = "idle" })
		if err := c.store.Save(); err != nil {
			c.log.Error("builds: save failed", "err", err)
		}
	}()
	d := c.dd.Data()
	if d == nil {
		c.fail("data dragon not loaded")
		return
	}

	c.update(func(p *Progress) { p.Phase = "fetching league players" })
	puuids, err := c.leaguePUUIDs(ctx)
	if err != nil {
		c.fail(err.Error())
		return
	}
	rand.Shuffle(len(puuids), func(i, j int) { puuids[i], puuids[j] = puuids[j], puuids[i] })
	c.update(func(p *Progress) { p.Players = len(puuids) })

	c.update(func(p *Progress) { p.Phase = "collecting matches" })
	done := 0
	lastSave := time.Now()
	for _, puuid := range puuids {
		if ctx.Err() != nil || done >= c.cfg.MatchesPerRun {
			break
		}
		ids, err := c.matchIDs(ctx, puuid)
		if err != nil {
			c.recordErr(err)
			if errors.Is(err, errFatal) {
				return
			}
			continue
		}
		for _, id := range ids {
			if ctx.Err() != nil || done >= c.cfg.MatchesPerRun {
				break
			}
			if c.store.Seen(id) {
				continue
			}
			var m Match
			if err := c.get(ctx, c.regionURL("/lol/match/v5/matches/"+id), &m); err != nil {
				c.recordErr(err)
				if errors.Is(err, errFatal) {
					return
				}
				continue
			}
			if m.Info.QueueID != c.cfg.Queue {
				continue
			}
			var tl *Timeline
			if c.cfg.Timeline {
				var t Timeline
				if err := c.get(ctx, c.regionURL("/lol/match/v5/matches/"+id+"/timeline"), &t); err == nil {
					tl = &t
				} else {
					c.recordErr(err)
					if errors.Is(err, errFatal) {
						return
					}
				}
			}
			c.store.Ingest(d, c.cfg.Platform, &m, tl)
			done++
			c.update(func(p *Progress) { p.MatchesDone = done })
			if time.Since(lastSave) > 30*time.Second {
				if err := c.store.Save(); err != nil {
					c.log.Error("builds: save failed", "err", err)
				}
				lastSave = time.Now()
			}
		}
	}
	c.log.Info("builds: compile finished", "matches", done)
}

func (c *Compiler) fail(msg string) {
	c.log.Error("builds: compile failed", "err", msg)
	c.update(func(p *Progress) { p.Errors++; p.LastError = msg })
}

func (c *Compiler) recordErr(err error) {
	c.log.Warn("builds: request failed", "err", err)
	c.update(func(p *Progress) { p.Errors++; p.LastError = err.Error() })
}

func (c *Compiler) platformURL(path string) string {
	return "https://" + c.cfg.Platform + ".api.riotgames.com" + path
}

func (c *Compiler) regionURL(path string) string {
	region := platformToRegion[c.cfg.Platform]
	if region == "" {
		region = "americas"
	}
	return "https://" + region + ".api.riotgames.com" + path
}

// leaguePUUIDs returns the puuids of every player in the configured tiers.
func (c *Compiler) leaguePUUIDs(ctx context.Context) ([]string, error) {
	var out []string
	for _, tier := range c.cfg.Tiers {
		var league struct {
			Entries []struct {
				PUUID string `json:"puuid"`
			} `json:"entries"`
		}
		url := c.platformURL("/lol/league/v4/" + tier + "leagues/by-queue/RANKED_SOLO_5x5")
		if err := c.get(ctx, url, &league); err != nil {
			return nil, err
		}
		for _, e := range league.Entries {
			if e.PUUID != "" {
				out = append(out, e.PUUID)
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("league-v4 returned no players")
	}
	return out, nil
}

func (c *Compiler) matchIDs(ctx context.Context, puuid string) ([]string, error) {
	var ids []string
	url := c.regionURL(fmt.Sprintf("/lol/match/v5/matches/by-puuid/%s/ids?queue=%d&count=10", puuid, c.cfg.Queue))
	err := c.get(ctx, url, &ids)
	return ids, err
}

// errFatal marks errors that should end the run (bad/expired key).
var errFatal = errors.New("fatal")

func (c *Compiler) get(ctx context.Context, url string, out any) error {
	for attempt := 0; attempt < 3; attempt++ {
		if err := c.limit.Wait(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-Riot-Token", c.cfg.APIKey)
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		c.update(func(p *Progress) { p.Requests++ })
		body, _ := io.ReadAll(io.LimitReader(res.Body, 32<<20))
		res.Body.Close()
		switch {
		case res.StatusCode == http.StatusOK:
			return json.Unmarshal(body, out)
		case res.StatusCode == http.StatusTooManyRequests:
			wait := 10 * time.Second
			if ra, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil {
				wait = time.Duration(ra+1) * time.Second
			}
			c.log.Warn("builds: rate limited", "retryAfter", wait)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
			return fmt.Errorf("%w: riot api %d (key invalid or expired)", errFatal, res.StatusCode)
		case res.StatusCode >= 500:
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
		default:
			return fmt.Errorf("riot api %d for %s", res.StatusCode, url)
		}
	}
	return fmt.Errorf("riot api: gave up on %s", url)
}

// limiter enforces two sliding windows.
type limiter struct {
	mu    sync.Mutex
	short window
	long  window
}

type window struct {
	n     int
	per   time.Duration
	times []time.Time
}

func newLimiter(n1 int, d1 time.Duration, n2 int, d2 time.Duration) *limiter {
	return &limiter{short: window{n: n1, per: d1}, long: window{n: n2, per: d2}}
}

func (w *window) wait(now time.Time) time.Duration {
	cut := now.Add(-w.per)
	i := 0
	for i < len(w.times) && w.times[i].Before(cut) {
		i++
	}
	w.times = w.times[i:]
	if len(w.times) < w.n {
		return 0
	}
	return w.times[0].Add(w.per).Sub(now)
}

func (l *limiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		now := time.Now()
		d := l.short.wait(now)
		if d2 := l.long.wait(now); d2 > d {
			d = d2
		}
		if d <= 0 {
			l.short.times = append(l.short.times, now)
			l.long.times = append(l.long.times, now)
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d + 50*time.Millisecond):
		}
	}
}
