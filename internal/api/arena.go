package api

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/aarlint/ezlol/internal/lcu"
	"github.com/aarlint/ezlol/internal/live"
)

// arenaTeam is one sub-team in Arena (duos or trios).
type arenaTeam struct {
	ID      int      `json:"id"`
	Mine    bool     `json:"mine"`
	Players []string `json:"players"` // player names, matching livePlayer.Name
	Kills   int      `json:"kills"`
	Deaths  int      `json:"deaths"`
	Alive   int      `json:"alive"`
	ItemAD  int      `json:"itemAD"`
	ItemAP  int      `json:"itemAP"`
	Threat  int      `json:"threat"` // 0-100 rough: kills + items
}

type arenaInfo struct {
	Teams      []arenaTeam `json:"teams"`
	TeamSize   int         `json:"teamSize"`
	Unassigned []string    `json:"unassigned,omitempty"` // players we could not place
}

// arenaCache derives Arena sub-teams from the event feed: a kill's killer and
// assisters are teammates, so union-find over ChampionKill events clusters the
// lobby into its teams after the first fights. The Live Client Data API reports
// every Arena player as "ORDER" and the client's gameflow session does not carry
// the sub-team, so this is the only reliable signal in game.
type arenaCache struct {
	mu     sync.Mutex
	key    string
	byName map[string]int
	order  []string // cluster roots in first-seen order, so team ids stay stable
}

func playerFingerprint(players []livePlayer) string {
	names := make([]string, 0, len(players))
	for _, p := range players {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return fmt.Sprint(len(names), names)
}

// resolve returns name -> team id (1..N) for players seen in at least one kill.
func (ac *arenaCache) resolve(_ context.Context, _ *lcu.Client, players []livePlayer, gd *live.GameData) map[string]int {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	fp := playerFingerprint(players)
	if ac.key != fp {
		ac.key = fp
		ac.order = nil
	}
	parent := map[string]string{}
	for _, p := range players {
		parent[p.Name] = p.Name
	}
	var find func(string) string
	find = func(x string) string {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b string) {
		if _, ok := parent[a]; !ok {
			return
		}
		if _, ok := parent[b]; !ok {
			return
		}
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	involved := map[string]bool{}
	for _, e := range gd.Events.Events {
		if e.Name != "ChampionKill" {
			continue
		}
		if _, ok := parent[e.KillerName]; ok {
			involved[e.KillerName] = true
		}
		for _, a := range e.Assisters {
			union(e.KillerName, a)
			involved[a] = true
		}
	}
	// Stable team numbering: keep previously assigned roots first.
	rootID := map[string]int{}
	for i, r := range ac.order {
		rootID[find(r)] = i + 1
	}
	byName := map[string]int{}
	for _, p := range players {
		if !involved[p.Name] {
			continue
		}
		r := find(p.Name)
		id, ok := rootID[r]
		if !ok {
			ac.order = append(ac.order, r)
			id = len(ac.order)
			rootID[r] = id
		}
		byName[p.Name] = id
	}
	ac.byName = byName
	return byName
}

// applyArena rewrites player teams to their Arena sub-team and summarises teams.
func (s *Server) applyArena(ctx context.Context, resp *liveResponse, gd *live.GameData) {
	teams := s.arena.resolve(ctx, s.watcher.Client(), resp.Players, gd)
	myTeam := 0
	for i := range resp.Players {
		p := &resp.Players[i]
		if t, ok := teams[p.Name]; ok {
			p.Team = fmt.Sprintf("T%d", t)
			if p.IsMe {
				myTeam = t
			}
		} else {
			p.Team = "T?"
		}
	}
	resp.MyTeam = fmt.Sprintf("T%d", myTeam)
	info := &arenaInfo{}
	byID := map[int]*arenaTeam{}
	for _, p := range resp.Players {
		t, ok := teams[p.Name]
		if !ok {
			info.Unassigned = append(info.Unassigned, p.Name)
			continue
		}
		at := byID[t]
		if at == nil {
			at = &arenaTeam{ID: t, Mine: t == myTeam}
			byID[t] = at
		}
		at.Players = append(at.Players, p.Name)
		at.Kills += p.Kills
		at.Deaths += p.Deaths
		if !p.IsDead {
			at.Alive++
		}
		at.ItemAD += p.ItemAD
		at.ItemAP += p.ItemAP
	}
	size := 0
	for _, at := range byID {
		if len(at.Players) > size {
			size = len(at.Players)
		}
	}
	info.TeamSize = size
	for _, at := range byID {
		// Threat: kills weigh most, then bought damage.
		at.Threat = min(100, at.Kills*6+(at.ItemAD+at.ItemAP)/12)
		info.Teams = append(info.Teams, *at)
	}
	sort.Slice(info.Teams, func(i, j int) bool {
		if info.Teams[i].Mine != info.Teams[j].Mine {
			return info.Teams[i].Mine
		}
		return info.Teams[i].Threat > info.Teams[j].Threat
	})
	resp.Arena = info
}
