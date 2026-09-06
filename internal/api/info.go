package api

import (
	"context"
	"net/http"
	"strconv"
	"sync"

	"github.com/aarlint/ezlol/internal/ddragon"
	"github.com/aarlint/ezlol/internal/lcu"
)

// infoCache memoises per-champion tactical info from the client for the process lifetime.
type infoCache struct {
	mu sync.Mutex
	m  map[int]*lcu.ChampionInfo
}

func (ic *infoCache) get(ctx context.Context, c *lcu.Client, id int) *lcu.ChampionInfo {
	if c == nil || id == 0 {
		return nil
	}
	ic.mu.Lock()
	if ic.m == nil {
		ic.m = map[int]*lcu.ChampionInfo{}
	}
	if v, ok := ic.m[id]; ok {
		ic.mu.Unlock()
		return v
	}
	ic.mu.Unlock()
	v, err := c.ChampionInfo(ctx, id)
	if err != nil {
		return nil
	}
	ic.mu.Lock()
	ic.m[id] = v
	ic.mu.Unlock()
	return v
}

func (s *Server) championInfo(w http.ResponseWriter, r *http.Request) {
	d := s.dd.Data()
	key := r.PathValue("key")
	var champ ddragon.Champion
	var ok bool
	if id, err := strconv.Atoi(key); err == nil {
		champ, ok = d.Champions[id]
	} else {
		champ, ok = d.ChampionByKey(key)
	}
	if !ok {
		writeErr(w, 404, "unknown champion")
		return
	}
	info := s.info.get(r.Context(), s.watcher.Client(), champ.ID)
	if info == nil {
		writeJSON(w, 200, map[string]any{"champion": champ})
		return
	}
	writeJSON(w, 200, map[string]any{"champion": champ, "info": info})
}

// profileWithInfo builds a damage profile using the client's damageType when
// available, falling back to Data Dragon's 0-10 hints.
func (s *Server) profileWithInfo(ctx context.Context, champs []ddragon.Champion) damageProfile {
	c := s.watcher.Client()
	var dp damageProfile
	for _, ch := range champs {
		if info := s.info.get(ctx, c, ch.ID); info != nil {
			switch info.TacticalInfo.DamageType {
			case "kMagic":
				dp.Magic += 10
			case "kPhysical":
				dp.Attack += 10
			default:
				dp.Attack += 5
				dp.Magic += 5
			}
			continue
		}
		dp.Attack += ch.Attack
		dp.Magic += ch.Magic
	}
	total := dp.Attack + dp.Magic
	switch {
	case total == 0:
	case dp.Attack*100/total >= 62:
		dp.Hint = "Heavy AD — armor beats magic resist"
	case dp.Magic*100/total >= 62:
		dp.Hint = "Heavy AP — magic resist beats armor"
	default:
		dp.Hint = "Mixed damage — balance resistances"
	}
	return dp
}

func (s *Server) endOfGame(w http.ResponseWriter, r *http.Request) {
	c := s.watcher.Client()
	if c == nil {
		writeJSON(w, 200, map[string]any{"available": false})
		return
	}
	eog, err := c.EndOfGameStats(r.Context())
	if err != nil || eog == nil {
		writeJSON(w, 200, map[string]any{"available": false})
		return
	}
	d := s.dd.Data()
	type row struct {
		lcu.EOGPlayer
		Champion ddragon.Champion `json:"champion"`
		ItemRefs []ItemRefLite    `json:"itemRefs"`
	}
	type team struct {
		IsWinningTeam bool  `json:"isWinningTeam"`
		TeamID        int   `json:"teamId"`
		Players       []row `json:"players"`
	}
	var teams []team
	for _, t := range eog.Teams {
		tt := team{IsWinningTeam: t.IsWinningTeam, TeamID: t.TeamID}
		for _, p := range t.Players {
			rr := row{EOGPlayer: p}
			if d != nil {
				rr.Champion = d.Champions[p.ChampionID]
				for _, id := range p.Items {
					if it, ok := d.Items[id]; ok && id != 0 {
						rr.ItemRefs = append(rr.ItemRefs, ItemRefLite{ID: id, Name: it.Name, Image: it.Image})
					}
				}
			}
			tt.Players = append(tt.Players, rr)
		}
		teams = append(teams, tt)
	}
	writeJSON(w, 200, map[string]any{"available": true, "gameLength": eog.GameLength, "gameMode": eog.GameMode, "queueId": eog.QueueID, "teams": teams})
}

// ItemRefLite is an item with display data.
type ItemRefLite struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
}
