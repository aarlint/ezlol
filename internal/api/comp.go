package api

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aarlint/ezlol/internal/ddragon"
)

// compSummary describes a team's shape for ARAM-style comp checks.
type compSummary struct {
	Tanks      int      `json:"tanks"`
	Ranged     int      `json:"ranged"`
	Melee      int      `json:"melee"`
	CC         int      `json:"cc"`         // sum of crowdControl pips (1-3 each)
	Durability int      `json:"durability"` // sum of durability pips
	Damage     int      `json:"damage"`
	Utility    int      `json:"utility"`
	Needs      []string `json:"needs"` // what the comp is missing
}

func (s *Server) summarize(ctx context.Context, champs []ddragon.Champion) compSummary {
	c := s.watcher.Client()
	var cs compSummary
	n := 0
	for _, ch := range champs {
		info := s.info.get(ctx, c, ch.ID)
		if info == nil {
			continue
		}
		n++
		if info.TacticalInfo.AttackType == "ranged" {
			cs.Ranged++
		} else {
			cs.Melee++
		}
		for _, r := range info.Roles {
			if r == "tank" {
				cs.Tanks++
				break
			}
		}
		cs.CC += info.PlaystyleInfo.CrowdControl
		cs.Durability += info.PlaystyleInfo.Durability
		cs.Damage += info.PlaystyleInfo.Damage
		cs.Utility += info.PlaystyleInfo.Utility
	}
	if n < 3 {
		return cs
	}
	if cs.Tanks == 0 && cs.Durability < n*2-1 {
		cs.Needs = append(cs.Needs, "frontline")
	}
	if cs.CC < n+1 {
		cs.Needs = append(cs.Needs, "crowd control")
	}
	if cs.Ranged == 0 {
		cs.Needs = append(cs.Needs, "poke / ranged damage")
	}
	if cs.Melee == 0 {
		cs.Needs = append(cs.Needs, "engage / melee")
	}
	if cs.Utility < n {
		cs.Needs = append(cs.Needs, "utility (heals, shields, peel)")
	}
	return cs
}

// benchScore ranks a bench champion for the local player: comfort (mastery,
// recent record) plus what the team is missing.
func (s *Server) benchScore(ctx context.Context, b *csBench, team compSummary) (int, string) {
	score := 0
	var why []string
	if b.Mastery != nil {
		switch {
		case b.Mastery.ChampionLevel >= 15:
			score += 40
			why = append(why, fmt.Sprintf("M%d main", b.Mastery.ChampionLevel))
		case b.Mastery.ChampionLevel >= 7:
			score += 25
			why = append(why, fmt.Sprintf("M%d", b.Mastery.ChampionLevel))
		case b.Mastery.ChampionLevel >= 3:
			score += 10
		}
	}
	if b.Record != nil && b.Record.Games > 0 {
		wr := b.Record.Wins * 100 / b.Record.Games
		if wr >= 60 && b.Record.Games >= 2 {
			score += 20
			why = append(why, fmt.Sprintf("%d%% recent", wr))
		} else if wr < 40 && b.Record.Games >= 2 {
			score -= 10
		}
	}
	info := s.info.get(ctx, s.watcher.Client(), b.Champion.ID)
	if info != nil {
		for _, need := range team.Needs {
			switch {
			case need == "frontline" && (hasRole(info.Roles, "tank") || info.PlaystyleInfo.Durability >= 3):
				score += 20
				why = append(why, "fills frontline")
			case need == "crowd control" && info.PlaystyleInfo.CrowdControl >= 3:
				score += 15
				why = append(why, "brings CC")
			case need == "poke / ranged damage" && info.TacticalInfo.AttackType == "ranged":
				score += 10
				why = append(why, "ranged")
			case need == "engage / melee" && info.TacticalInfo.AttackType == "melee":
				score += 10
				why = append(why, "melee engage")
			case strings.HasPrefix(need, "utility") && info.PlaystyleInfo.Utility >= 3:
				score += 15
				why = append(why, "utility")
			}
		}
	}
	return score, strings.Join(why, " · ")
}

func hasRole(roles []string, r string) bool {
	for _, x := range roles {
		if x == r {
			return true
		}
	}
	return false
}

// rankBench sorts the bench best-first and fills score/why.
func (s *Server) rankBench(ctx context.Context, bench []csBench, team compSummary) {
	for i := range bench {
		bench[i].Score, bench[i].Why = s.benchScore(ctx, &bench[i], team)
	}
	sort.SliceStable(bench, func(i, j int) bool { return bench[i].Score > bench[j].Score })
}
