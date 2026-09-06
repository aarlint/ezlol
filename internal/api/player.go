package api

import (
	"context"
	"sync"
	"time"

	"github.com/aarlint/ezlol/internal/builds"
	"github.com/aarlint/ezlol/internal/lcu"
)

// Record is the local player's history with one champion in one queue family.
type Record struct {
	Games   int `json:"games"`
	Wins    int `json:"wins"`
	Kills   int `json:"kills"`
	Deaths  int `json:"deaths"`
	Assists int `json:"assists"`
}

// SessionGame is one game from today, newest first.
type SessionGame struct {
	ChampionID int   `json:"championId"`
	Win        bool  `json:"win"`
	Kills      int   `json:"kills"`
	Deaths     int   `json:"deaths"`
	Assists    int   `json:"assists"`
	QueueID    int   `json:"queueId"`
	Duration   int   `json:"duration"`
	Created    int64 `json:"created"`
}

// playerCache holds the local player's mastery and recent history, refreshed lazily.
type playerCache struct {
	mu      sync.Mutex
	at      time.Time
	mastery map[int]lcu.Mastery
	records map[string]map[int]*Record // queue tag -> champion -> record
	today   []SessionGame
}

func (pc *playerCache) refresh(ctx context.Context, c *lcu.Client) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if time.Since(pc.at) < 30*time.Second && pc.mastery != nil {
		return
	}
	mastery := map[int]lcu.Mastery{}
	if list, err := c.ChampionMastery(ctx); err == nil {
		for _, m := range list {
			mastery[m.ChampionID] = m
		}
	}
	records := map[string]map[int]*Record{}
	var today []SessionGame
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if me, err := c.CurrentSummoner(ctx); err == nil {
		if games, err := c.MatchHistory(ctx, me.PUUID, 100); err == nil {
			for _, g := range games {
				if len(g.Participants) == 0 || g.GameDuration < 240 {
					continue
				}
				if time.UnixMilli(g.GameCreation).After(dayStart) {
					p := g.Participants[0]
					today = append(today, SessionGame{ChampionID: p.ChampionID, Win: p.Stats.Win, Kills: p.Stats.Kills, Deaths: p.Stats.Deaths,
						Assists: p.Stats.Assists, QueueID: g.QueueID, Duration: g.GameDuration, Created: g.GameCreation})
				}
				tag := builds.QueueTag(g.QueueID)
				p := g.Participants[0]
				byChamp := records[tag]
				if byChamp == nil {
					byChamp = map[int]*Record{}
					records[tag] = byChamp
				}
				r := byChamp[p.ChampionID]
				if r == nil {
					r = &Record{}
					byChamp[p.ChampionID] = r
				}
				r.Games++
				if p.Stats.Win {
					r.Wins++
				}
				r.Kills += p.Stats.Kills
				r.Deaths += p.Stats.Deaths
				r.Assists += p.Stats.Assists
			}
		}
	}
	if len(mastery) > 0 || len(records) > 0 {
		pc.mastery, pc.records, pc.today, pc.at = mastery, records, today, time.Now()
	}
}

func (pc *playerCache) lookup(tag string, champ int) (*lcu.Mastery, *Record) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	var m *lcu.Mastery
	if v, ok := pc.mastery[champ]; ok {
		m = &v
	}
	var r *Record
	if v, ok := pc.records[tag][champ]; ok {
		cp := *v
		r = &cp
	}
	return m, r
}

// Today returns today's games, newest first.
func (pc *playerCache) Today() []SessionGame {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	out := make([]SessionGame, len(pc.today))
	copy(out, pc.today)
	return out
}

// AllMastery returns the full mastery map (champion id -> mastery).
func (pc *playerCache) AllMastery() map[int]lcu.Mastery {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	out := make(map[int]lcu.Mastery, len(pc.mastery))
	for k, v := range pc.mastery {
		out[k] = v
	}
	return out
}
