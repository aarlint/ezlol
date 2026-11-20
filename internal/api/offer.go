package api

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aarlint/ezlol/internal/builds"
	"github.com/aarlint/ezlol/internal/screen"
)

// augmentOffer is the current augment selection screen, when detected.
type augmentOffer struct {
	Active    bool             `json:"active"`
	Pending   bool             `json:"pending"` // a pick is due (level threshold crossed) but not yet seen on screen
	Level     int              `json:"level"`
	Bonus     bool             `json:"bonus,omitempty"` // an offer seen with no level pick due (K/DA photocards, queued screens)
	Offered   []builds.Augment `json:"offered"`         // ranked best first
	Best      string           `json:"best,omitempty"`
	Why       string           `json:"why,omitempty"`
	ScannedAt time.Time        `json:"scannedAt"`
	Lines     []string         `json:"lines,omitempty"` // diagnostics: what OCR last read
}

// Mayhem grants augment picks at game start and at these champion levels; the
// Headliner can add more (Pentakill 18, Superfans 21, K/DA on photocards).
var pickLevels = []int{1, 7, 11, 15}

type offerState struct {
	mu        sync.Mutex
	gameStart time.Time
	seenLevel map[int]bool // levels whose pick has been completed (offer seen then gone)
	pending   int          // level with a pick outstanding, 0 = none
	pendingAt time.Time
	last      *augmentOffer
	lastScan  time.Time
	lastGT    float64
	lastLines []string
	picks     int // Arena: completed picks this game
}

// isMayhem: augments exist in ARAM: Mayhem (game mode KIWI), not plain ARAM.
func (s *Server) isMayhem(live *liveResponse) bool {
	return live.GameMode == "KIWI" || builds.IsMayhem(s.watcher.Status().QueueID)
}

// detectOffer decides whether an augment pick is likely on screen and, if so,
// OCRs the display to identify the offered augments (three, or four under
// Maestro) and rank them.
func (s *Server) detectOffer(ctx context.Context, live *liveResponse) (*augmentOffer, string) {
	st := &s.offer
	st.mu.Lock()
	defer st.mu.Unlock()
	if !s.ocr.Available() {
		return nil, "unavailable"
	}
	if !s.settings.Get().OCR {
		return nil, "disabled"
	}
	tag := builds.QueueTag(s.watcher.Status().QueueID)
	if tag == "arena" {
		return s.detectArenaOffer(ctx, live)
	}
	if !s.isMayhem(live) {
		return nil, "available"
	}
	// New game: reset.
	fresh := live.GameTime < st.lastGT-30 || st.seenLevel == nil
	if fresh {
		st.seenLevel = map[int]bool{}
		st.pending = 0
		st.last = nil
	}
	st.lastGT = live.GameTime
	var me *livePlayer
	for i := range live.Players {
		if live.Players[i].IsMe {
			me = &live.Players[i]
		}
	}
	if me == nil {
		return nil, "available"
	}
	levels := append(append([]int{}, pickLevels...), s.headliner.extraPickLevels()...)
	// Joined mid-game: picks below the current level already happened.
	if fresh && live.GameTime > 120 {
		for _, lv := range levels {
			if me.Level > lv {
				st.seenLevel[lv] = true
			}
		}
	}
	arm := func() {
		for _, lv := range levels {
			if me.Level >= lv && !st.seenLevel[lv] {
				st.pending, st.pendingAt = lv, time.Now()
				return
			}
		}
	}
	// Mark a pick pending when a threshold is crossed (start counts as level 1).
	if st.pending == 0 {
		arm()
	}
	active := st.last != nil && st.last.Active
	// Picks show while dead or at the fountain. With a pick due we scan on death,
	// early on and right after the level-up; with none due we still scan while
	// dead (slower), because K/DA photocards and missed screens grant picks that
	// no level marks.
	interval := 1500 * time.Millisecond
	var shouldScan bool
	if st.pending != 0 {
		shouldScan = me.IsDead || live.GameTime < 120 || time.Since(st.pendingAt) < 45*time.Second || active
	} else {
		shouldScan = me.IsDead || active
		if !active {
			interval = 2500 * time.Millisecond
		}
	}
	if !shouldScan || time.Since(st.lastScan) < interval {
		if active && time.Since(st.lastScan) < 6*time.Second {
			return st.last, "available"
		}
		if st.pending == 0 {
			return nil, "available"
		}
		return &augmentOffer{Pending: true, Level: st.pending, Lines: st.lastLines}, "available"
	}
	st.lastScan = time.Now()
	lines, err := s.ocr.Read(ctx)
	if err != nil {
		if st.pending == 0 {
			return nil, err.Error()
		}
		return &augmentOffer{Pending: true, Level: st.pending}, err.Error()
	}
	s.headliner.observe(lines, live.GameTime)
	patch := s.watcher.Status().Patch
	augs, _, err := s.community.Augments(ctx, patch, me.Champion.ID)
	if err != nil {
		if st.pending == 0 {
			return nil, "available"
		}
		return &augmentOffer{Pending: true, Level: st.pending}, "available"
	}
	byID := map[int]builds.Augment{}
	cands := map[string]string{}
	for _, a := range augs {
		byID[a.ID] = a
		if n := screen.Normalize(a.Name); len(n) >= 4 {
			cands[n] = strconv.Itoa(a.ID)
		}
	}
	var diag []string
	for _, l := range lines {
		if l.Conf >= 0.3 && len(l.Text) >= 3 && len(diag) < 40 {
			diag = append(diag, l.Text)
		}
	}
	st.lastLines = diag
	keys := screen.OfferLayout(screen.Match(lines, cands))
	if len(keys) < 2 {
		// Offer disappeared after being seen: the pick is done.
		if active && time.Since(st.last.ScannedAt) > 4*time.Second {
			if st.pending != 0 {
				st.seenLevel[st.pending] = true
			}
			st.pending = 0
			st.last = nil
			return nil, "available"
		}
		if st.pending == 0 {
			if active {
				return st.last, "available"
			}
			return nil, "available"
		}
		// Give up waiting after a long time so the next threshold can arm.
		if time.Since(st.pendingAt) > 8*time.Minute {
			st.seenLevel[st.pending] = true
			st.pending = 0
		}
		return &augmentOffer{Pending: st.pending != 0, Level: st.pending, Lines: st.lastLines}, "available"
	}
	var offered []builds.Augment
	for _, k := range keys {
		id, _ := strconv.Atoi(k.Key)
		if a, ok := byID[id]; ok {
			offered = append(offered, a)
		}
	}
	// A wholly different set of cards right after a pick is the next queued
	// screen (several level picks banked while alive): count the previous pick.
	if active && !overlaps(st.last.Offered, offered) {
		if st.pending != 0 {
			st.seenLevel[st.pending] = true
		}
		st.pending = 0
		arm()
	}
	off := &augmentOffer{Active: true, Level: st.pending, Bonus: st.pending == 0, ScannedAt: time.Now(), Lines: diag, Offered: offered}
	s.rankOffer(ctx, off, me)
	s.headliner.inferFromOffer(off)
	if !active {
		s.log.Info("augment offer detected", "level", off.Level, "bonus", off.Bonus, "offered", len(off.Offered), "best", off.Best)
	}
	st.last = off
	return off, "available"
}

func overlaps(a, b []builds.Augment) bool {
	for _, x := range a {
		for _, y := range b {
			if x.ID == y.ID {
				return true
			}
		}
	}
	return false
}

// rankOffer orders a Mayhem offer best first and explains the top card.
// Pentakill's exclusive pick (HEAVY/POWER/THRASH, DOOM at 18) follows the
// champion's skill max order; everything else win rate on a real sample, then
// tier, then the all-champion win rate.
func (s *Server) rankOffer(ctx context.Context, off *augmentOffer, me *livePlayer) {
	var prio string
	for _, a := range off.Offered {
		if a.Skill != "" {
			prio = s.skillPriority(ctx, me)
			break
		}
	}
	skillRank := func(a builds.Augment) int {
		if a.Skill == "" {
			return 9
		}
		if i := strings.Index(prio, a.Skill); i >= 0 {
			return i
		}
		return 8
	}
	scopeRank := func(a builds.Augment) int {
		switch a.Scope {
		case "champion":
			return 0
		case "global":
			return 1
		}
		return 2
	}
	sort.SliceStable(off.Offered, func(i, j int) bool {
		a, b := off.Offered[i], off.Offered[j]
		if ra, rb := skillRank(a), skillRank(b); ra != rb {
			return ra < rb
		}
		ta, tb := a.Tier, b.Tier
		if ta == 0 {
			ta = 9
		}
		if tb == 0 {
			tb = 9
		}
		// Prefer win rate when both have a meaningful sample; tier breaks ties.
		if a.Games >= 200 && b.Games >= 200 && a.WinRate != b.WinRate {
			return a.WinRate > b.WinRate
		}
		if ta != tb {
			return ta < tb
		}
		if sa, sb := scopeRank(a), scopeRank(b); sa != sb {
			return sa < sb
		}
		return a.WinRate > b.WinRate
	})
	if len(off.Offered) == 0 {
		return
	}
	b := off.Offered[0]
	off.Best = b.Name
	switch {
	case b.Skill != "" && prio != "":
		off.Why = "Pentakill: empowers " + b.Skill + ", first in your skill order (" + strings.Join(strings.Split(prio, ""), " > ") + ")"
	case b.Skill != "":
		off.Why = "Pentakill: empowers " + b.Skill
	case b.Games >= 200:
		off.Why = strconv.Itoa(int(b.WinRate*100+0.5)) + "% win rate on " + me.Champion.Name + " over " + strconv.Itoa(b.Games) + " games"
	case b.Tier > 0:
		off.Why = "tier " + strconv.Itoa(b.Tier) + " for " + me.Champion.Name
	case b.Scope == "global" && b.WinRate > 0:
		off.Why = strconv.Itoa(int(b.WinRate*100+0.5)) + "% win rate across all champions (little data on " + me.Champion.Name + ")"
	case b.New:
		off.Why = "new this patch, no stats yet"
	default:
		off.Why = "best available"
	}
}

// skillPriority is the champion's ability max order with the ultimate first,
// e.g. "RQEW", from the op.gg ARAM build (cached); "" when unknown.
func (s *Server) skillPriority(ctx context.Context, me *livePlayer) string {
	d := s.dd.Data()
	if d == nil || s.community == nil || me.Champion.ID == 0 {
		return ""
	}
	champ, ok := d.Champions[me.Champion.ID]
	if !ok {
		return ""
	}
	b, err := s.community.ARAMBuild(ctx, d, champ)
	if err != nil || len(b.SkillOrder) == 0 {
		return ""
	}
	order := "R"
	for _, k := range strings.Split(b.SkillOrder[0].Order, ">") {
		k = strings.TrimSpace(k)
		if len(k) == 1 && strings.Contains("QWE", k) && !strings.Contains(order, k) {
			order += k
		}
	}
	return order
}

// detectArenaOffer scans continuously (every 3s, 1.5s while an offer is up):
// Arena offers augments between rounds, and nothing in the live data marks it.
func (s *Server) detectArenaOffer(ctx context.Context, live *liveResponse) (*augmentOffer, string) {
	st := &s.offer
	var me *livePlayer
	for i := range live.Players {
		if live.Players[i].IsMe {
			me = &live.Players[i]
		}
	}
	if me == nil {
		return nil, "available"
	}
	interval := 3 * time.Second
	if st.last != nil && st.last.Active {
		interval = 1500 * time.Millisecond
	}
	if time.Since(st.lastScan) < interval {
		if st.last != nil && st.last.Active && time.Since(st.lastScan) < 6*time.Second {
			return st.last, "available"
		}
		return nil, "available"
	}
	st.lastScan = time.Now()
	lines, err := s.ocr.Read(ctx)
	if err != nil {
		return nil, err.Error()
	}
	augs, _, err := s.community.ArenaAugments(ctx, s.watcher.Status().Patch, me.Champion.ID)
	if err != nil {
		return nil, "available"
	}
	byID := map[int]builds.Augment{}
	cands := map[string]string{}
	for _, a := range augs {
		byID[a.ID] = a
		if n := screen.Normalize(a.Name); len(n) >= 4 {
			cands[n] = strconv.Itoa(a.ID)
		}
	}
	keys := screen.OfferLayout(screen.Match(lines, cands))
	if len(keys) < 2 {
		if st.last != nil && st.last.Active && time.Since(st.last.ScannedAt) > 4*time.Second {
			st.picks++
			st.last = nil
		}
		return nil, "available"
	}
	off := &augmentOffer{Active: true, Level: st.picks + 1, ScannedAt: time.Now()}
	for _, k := range keys {
		id, _ := strconv.Atoi(k.Key)
		if a, ok := byID[id]; ok {
			off.Offered = append(off.Offered, a)
		}
	}
	// Lower average placement wins; tier breaks ties; then sample size.
	sort.SliceStable(off.Offered, func(i, j int) bool {
		a, b := off.Offered[i], off.Offered[j]
		ap, bp := a.AvgPlace, b.AvgPlace
		if ap == 0 {
			ap = 9
		}
		if bp == 0 {
			bp = 9
		}
		if a.Games >= 100 && b.Games >= 100 && ap != bp {
			return ap < bp
		}
		ta, tb := a.Tier, b.Tier
		if ta == 0 {
			ta = 9
		}
		if tb == 0 {
			tb = 9
		}
		if ta != tb {
			return ta < tb
		}
		return ap < bp
	})
	if len(off.Offered) > 0 {
		b := off.Offered[0]
		off.Best = b.Name
		switch {
		case b.Games >= 100 && b.AvgPlace > 0:
			off.Why = fmt.Sprintf("avg place %.2f on %s over %d games", b.AvgPlace, me.Champion.Name, b.Games)
		case b.Tier > 0:
			off.Why = "tier " + strconv.Itoa(b.Tier) + " across all champions"
		default:
			off.Why = "best available"
		}
	}
	if st.last == nil || !st.last.Active {
		s.log.Info("arena augment offer detected", "pick", off.Level, "offered", len(off.Offered), "best", off.Best)
	}
	st.last = off
	return off, "available"
}
