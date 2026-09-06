// Package builds compiles champion builds from Riot match data and serves them.
package builds

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Count is a games/wins tally.
type Count struct {
	Games int `json:"games"`
	Wins  int `json:"wins"`
}

func (c *Count) add(win bool) {
	c.Games++
	if win {
		c.Wins++
	}
}

// Agg is the raw aggregate for one champion in one role.
type Agg struct {
	Total      Count             `json:"total"`
	Starting   map[string]*Count `json:"starting"`   // "1055,2003,2003" sorted
	Core       map[string]*Count `json:"core"`       // "3157,3089,4645" in purchase order
	Boots      map[string]*Count `json:"boots"`      // item id
	Items      map[string]*Count `json:"items"`      // any completed item id
	Late       map[string]*Count `json:"late"`       // completed items bought 4th+
	Runes      map[string]*Count `json:"runes"`      // "8100|8300|8112,8139,8140,8106,8345,8347|5005,5008,5001"
	Spells     map[string]*Count `json:"spells"`     // "4,14" sorted
	SkillOrder map[string]*Count `json:"skillOrder"` // "Q>W>E"
	SkillStart map[string]*Count `json:"skillStart"` // "QWE" first three level-ups
}

func newAgg() *Agg {
	return &Agg{
		Starting: map[string]*Count{}, Core: map[string]*Count{}, Boots: map[string]*Count{},
		Items: map[string]*Count{}, Late: map[string]*Count{}, Runes: map[string]*Count{},
		Spells: map[string]*Count{}, SkillOrder: map[string]*Count{}, SkillStart: map[string]*Count{},
	}
}

func bump(m map[string]*Count, key string, win bool) {
	if key == "" {
		return
	}
	c := m[key]
	if c == nil {
		c = &Count{}
		m[key] = c
	}
	c.add(win)
}

// PatchData is everything compiled for one patch.
type PatchData struct {
	Patch     string                  `json:"patch"`
	Platform  string                  `json:"platform"`
	Matches   int                     `json:"matches"`
	Seen      map[string]bool         `json:"seen"`      // match ids already ingested
	Champions map[int]map[string]*Agg `json:"champions"` // champ id -> role -> agg
}

// Store keeps per-patch aggregates on disk.
type Store struct {
	dir     string
	mu      sync.RWMutex
	patches map[string]*PatchData
}

// NewStore creates a store rooted at dir and loads existing patch files.
func NewStore(dir string) (*Store, error) {
	s := &Store{dir: dir, patches: map[string]*PatchData{}}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var pd PatchData
		if err := json.Unmarshal(b, &pd); err != nil {
			return nil, fmt.Errorf("builds: corrupt %s: %w", e.Name(), err)
		}
		if pd.Seen == nil {
			pd.Seen = map[string]bool{}
		}
		if pd.Champions == nil {
			pd.Champions = map[int]map[string]*Agg{}
		}
		s.patches[pd.Patch] = &pd
	}
	return s, nil
}

func (s *Store) patch(p, platform string) *PatchData {
	pd := s.patches[p]
	if pd == nil {
		pd = &PatchData{Patch: p, Platform: platform, Seen: map[string]bool{}, Champions: map[int]map[string]*Agg{}}
		s.patches[p] = pd
	}
	return pd
}

// Seen reports whether a match id was already ingested for this patch (any patch, since ids are global).
func (s *Store) Seen(matchID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, pd := range s.patches {
		if pd.Seen[matchID] {
			return true
		}
	}
	return false
}

// Save writes every patch file to disk.
func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for p, pd := range s.patches {
		b, err := json.Marshal(pd)
		if err != nil {
			return err
		}
		tmp := filepath.Join(s.dir, p+".json.tmp")
		if err := os.WriteFile(tmp, b, 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmp, filepath.Join(s.dir, p+".json")); err != nil {
			return err
		}
	}
	return nil
}

// Patches lists compiled patches, newest first, with match counts.
func (s *Store) Patches() []PatchSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []PatchSummary
	for _, pd := range s.patches {
		out = append(out, PatchSummary{Patch: pd.Patch, Matches: pd.Matches, Champions: len(pd.Champions)})
	}
	sort.Slice(out, func(i, j int) bool { return patchLess(out[j].Patch, out[i].Patch) })
	return out
}

// PatchSummary is a row in the patches list.
type PatchSummary struct {
	Patch     string `json:"patch"`
	Matches   int    `json:"matches"`
	Champions int    `json:"champions"`
}

// patchLess orders "16.9" < "16.17" numerically.
func patchLess(a, b string) bool {
	var a1, a2, b1, b2 int
	fmt.Sscanf(a, "%d.%d", &a1, &a2)
	fmt.Sscanf(b, "%d.%d", &b1, &b2)
	if a1 != b1 {
		return a1 < b1
	}
	return a2 < b2
}

// ErrNoData means nothing has been compiled for that champion.
var ErrNoData = errors.New("no compiled data")

// Roles returns the roles with data for a champion on a patch, most played first.
func (s *Store) Roles(patch string, champ int) []RoleCount {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pd := s.patches[patch]
	if pd == nil {
		return nil
	}
	var out []RoleCount
	for role, agg := range pd.Champions[champ] {
		out = append(out, RoleCount{Role: role, Count: agg.Total})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Games > out[j].Games })
	return out
}

// RoleCount is a role with its tally.
type RoleCount struct {
	Role string `json:"role"`
	Count
}

// Agg returns a deep copy of the aggregate for champ/role, or ErrNoData.
func (s *Store) Agg(patch string, champ int, role string) (*Agg, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pd := s.patches[patch]
	if pd == nil {
		return nil, ErrNoData
	}
	agg := pd.Champions[champ][role]
	if agg == nil {
		return nil, ErrNoData
	}
	b, _ := json.Marshal(agg)
	var cp Agg
	_ = json.Unmarshal(b, &cp)
	return &cp, nil
}

// Top returns the top-n keys of a tally map by games, with a minimum share filter.
func Top(m map[string]*Count, n int) []KeyCount {
	out := make([]KeyCount, 0, len(m))
	for k, c := range m {
		out = append(out, KeyCount{Key: k, Count: *c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Games != out[j].Games {
			return out[i].Games > out[j].Games
		}
		return out[i].Key < out[j].Key
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// KeyCount is a tally row.
type KeyCount struct {
	Key string `json:"key"`
	Count
}
