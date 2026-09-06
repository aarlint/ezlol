// Package update checks GitHub releases for a newer ezlol.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Info is the result of the last check.
type Info struct {
	Current   string    `json:"current"`
	Latest    string    `json:"latest"`
	HasUpdate bool      `json:"hasUpdate"`
	URL       string    `json:"url"`
	Notes     string    `json:"notes"`
	CheckedAt time.Time `json:"checkedAt"`
	Error     string    `json:"error,omitempty"`
}

// Checker polls the releases API.
type Checker struct {
	repo    string
	current string
	http    *http.Client
	mu      sync.RWMutex
	info    Info
}

// New creates a checker for owner/repo.
func New(repo, current string) *Checker {
	return &Checker{repo: repo, current: current, http: &http.Client{Timeout: 15 * time.Second}, info: Info{Current: current}}
}

// Info returns the last result.
func (c *Checker) Info() Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info
}

// Check queries GitHub once.
func (c *Checker) Check(ctx context.Context) Info {
	info := Info{Current: c.current, CheckedAt: time.Now()}
	err := c.fetch(ctx, &info)
	if err != nil {
		info.Error = err.Error()
	}
	c.mu.Lock()
	c.info = info
	c.mu.Unlock()
	return info
}

func (c *Checker) fetch(ctx context.Context, info *Info) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+c.repo+"/releases/latest", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ezlol/"+c.current)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("github: status %d", res.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(res.Body).Decode(&rel); err != nil {
		return err
	}
	info.Latest = strings.TrimPrefix(rel.TagName, "v")
	info.URL = rel.HTMLURL
	info.Notes = rel.Body
	info.HasUpdate = Newer(info.Latest, c.current)
	return nil
}

// Run checks now and then every interval until ctx ends.
func (c *Checker) Run(ctx context.Context, interval time.Duration, enabled func() bool) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if enabled() {
			c.Check(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Newer reports whether a > b for dotted numeric versions ("1.0.10" > "1.0.9").
// Non-numeric versions (e.g. "dev") never count as older than a release.
func Newer(a, b string) bool {
	pa, pb := parts(a), parts(b)
	if pa == nil || pb == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	fs := strings.Split(v, ".")
	out := []int{0, 0, 0}
	for i := 0; i < len(fs) && i < 3; i++ {
		n, err := strconv.Atoi(fs[i])
		if err != nil {
			return nil
		}
		out[i] = n
	}
	return out
}
