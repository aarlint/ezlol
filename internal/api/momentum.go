package api

import (
	"fmt"
	"sync"

	"github.com/aarlint/ezlol/internal/live"
)

// Momentum: the game API only reveals your own gold, but every player's items
// are visible, so the value of the items each team carries is a fair stand-in
// for the gold lead. ezlol samples it every few seconds of game time, keeps the
// series for the whole game (a reload mid-game keeps the graph) and marks the
// moments that swung it.

type tlPoint struct {
	T     float64 `json:"t"`
	Ally  int     `json:"ally"`  // your team's item gold
	Enemy int     `json:"enemy"` // the enemy team's item gold
	AK    int     `json:"ak"`    // your team's kills
	EK    int     `json:"ek"`    // the enemy team's kills
}

type tlMark struct {
	T     float64 `json:"t"`
	Kind  string  `json:"kind"` // ace | dragon | baron | herald | multikill | firstblood
	Ally  bool    `json:"ally"`
	Label string  `json:"label"`
}

type momentum struct {
	Points []tlPoint `json:"points"`
	Marks  []tlMark  `json:"marks"`
}

const (
	tlStep = 10.0 // seconds of game time between kept samples
	tlMax  = 600  // thinned to half beyond this
)

type timelineState struct {
	mu  sync.Mutex
	pts []tlPoint
}

// record adds a sample. The newest point always reflects now; older points are
// at least tlStep apart. A game time jump backwards means a new game.
func (tl *timelineState) record(p tlPoint) []tlPoint {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	n := len(tl.pts)
	if n > 0 && p.T < tl.pts[n-1].T-30 {
		tl.pts, n = nil, 0
	}
	switch {
	case n >= 1 && p.T <= tl.pts[n-1].T:
		tl.pts[n-1] = p
	case n >= 2 && tl.pts[n-1].T-tl.pts[n-2].T < tlStep:
		// The live point is not tlStep past the last kept one yet: move it.
		tl.pts[n-1] = p
	default:
		tl.pts = append(tl.pts, p)
	}
	if len(tl.pts) > tlMax {
		thin := make([]tlPoint, 0, tlMax/2+1)
		for i := 0; i < len(tl.pts)-1; i += 2 {
			thin = append(thin, tl.pts[i])
		}
		tl.pts = append(thin, tl.pts[len(tl.pts)-1])
	}
	return append([]tlPoint(nil), tl.pts...)
}

func (tl *timelineState) reset() {
	tl.mu.Lock()
	tl.pts = nil
	tl.mu.Unlock()
}

// teamSample sums item gold and kills per side.
func teamSample(t float64, players []livePlayer, myTeam string) tlPoint {
	p := tlPoint{T: t}
	for _, pl := range players {
		if pl.Team == myTeam {
			p.Ally += pl.ItemGold
			p.AK += pl.Kills
		} else {
			p.Enemy += pl.ItemGold
			p.EK += pl.Kills
		}
	}
	return p
}

var multikillName = map[int]string{3: "Triple kill", 4: "Quadra kill", 5: "PENTAKILL"}

// momentumMarks picks the swing moments out of the game's event list.
func momentumMarks(evs []live.Event, players []livePlayer, myTeam string) []tlMark {
	team := map[string]string{}
	champ := map[string]string{}
	for _, p := range players {
		team[p.Name] = p.Team
		champ[p.Name] = p.Champion.Name
	}
	who := func(n string) string {
		if c := champ[n]; c != "" {
			return c
		}
		return n
	}
	var out []tlMark
	for _, e := range evs {
		m := tlMark{T: e.Time, Ally: team[e.KillerName] == myTeam}
		switch e.Name {
		case "Ace":
			m.Kind, m.Ally, m.Label = "ace", e.AcingTeam == myTeam, "Ace"
		case "DragonKill":
			m.Kind, m.Label = "dragon", e.DragonType+" drake"
			if e.DragonType == "Elder" {
				m.Label = "Elder dragon"
			}
		case "BaronKill":
			m.Kind, m.Label = "baron", "Baron"
		case "HeraldKill":
			m.Kind, m.Label = "herald", "Herald"
		case "FirstBlood":
			// FirstBlood names the killer as Recipient, not KillerName.
			m.Kind, m.Ally, m.Label = "firstblood", team[e.Recipient] == myTeam, "First blood · "+who(e.Recipient)
		case "Multikill":
			name, ok := multikillName[e.KillStreak]
			if !ok {
				continue
			}
			m.Kind, m.Label = "multikill", fmt.Sprintf("%s %s", who(e.KillerName), name)
		default:
			continue
		}
		if e.Stolen == "True" {
			m.Label += " (stolen)"
		}
		out = append(out, m)
	}
	if len(out) > 40 {
		out = out[len(out)-40:]
	}
	return out
}
