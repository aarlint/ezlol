package builds

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aarlint/ezlol/internal/ddragon"
	"github.com/aarlint/ezlol/internal/lcu"
)

// ItemRef is an item with display data.
type ItemRef struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
}

// ItemSet is a group of items with a tally.
type ItemSet struct {
	Items    []ItemRef `json:"items"`
	AvgPlace float64   `json:"avgPlace,omitempty"` // Arena
	Count
}

// RuneRef is a perk with display data.
type RuneRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
	Desc string `json:"desc,omitempty"`
}

// StyleRef is a rune tree with display data.
type StyleRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// RunePage is a full page.
type RunePage struct {
	Primary   StyleRef  `json:"primary"`
	Secondary StyleRef  `json:"secondary"`
	Perks     []RuneRef `json:"perks"`  // 4 primary + 2 secondary
	Shards    []RuneRef `json:"shards"` // 3
	Source    string    `json:"source"` // riot | lcu
	Count
}

// SpellRef is a summoner spell with display data.
type SpellRef struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
}

// SpellPair is two spells with a tally.
type SpellPair struct {
	Spells []SpellRef `json:"spells"`
	Count
}

// Skills is a skill order row.
type Skills struct {
	Order string `json:"order"`
	Count
}

// Build is the resolved, display-ready build for one champion/role.
type Build struct {
	Champion   ddragon.Champion `json:"champion"`
	Patch      string           `json:"patch"`
	Role       string           `json:"role"`
	Mode       string           `json:"mode"` // sr | aram | arena
	Roles      []RoleCount      `json:"roles"`
	Source     string           `json:"source"` // riot | lcu | none
	Total      Count            `json:"total"`
	Starting   []ItemSet        `json:"starting"`
	Core       []ItemSet        `json:"core"`
	Boots      []ItemSet        `json:"boots"`
	Late       []ItemSet        `json:"late"`
	Items      []ItemSet        `json:"items"`
	Runes      []RunePage       `json:"runes"`
	Spells     []SpellPair      `json:"spells"`
	SkillOrder []Skills         `json:"skillOrder"`
	SkillStart []Skills         `json:"skillStart"`
	SkillPath  []string         `json:"skillPath,omitempty"` // full level-up order
	Tier       int              `json:"tier,omitempty"`      // 1 best .. 5 (op.gg)
	Rank       int              `json:"rank,omitempty"`
	PickRate   float64          `json:"pickRate,omitempty"`
	Augments   []Augment        `json:"augments,omitempty"`
	AugScope   string           `json:"augScope,omitempty"`  // champion | global
	Prismatic  []ItemSet        `json:"prismatic,omitempty"` // Arena prismatic items
	Synergies  []Synergy        `json:"synergies,omitempty"` // Arena partners
	AvgPlace   float64          `json:"avgPlace,omitempty"`  // Arena
	Top1       float64          `json:"top1,omitempty"`      // Arena first-place rate
	Notes      []string         `json:"notes"`
}

// Resolve turns an aggregate into a display build.
func Resolve(d *ddragon.Data, champ ddragon.Champion, patch, role string, roles []RoleCount, agg *Agg) *Build {
	b := &Build{Champion: champ, Patch: patch, Role: role, Roles: roles, Source: "riot", Total: agg.Total}
	item := func(id int) ItemRef {
		it := d.Items[id]
		return ItemRef{ID: id, Name: it.Name, Image: it.Image}
	}
	sets := func(m map[string]*Count, n int) []ItemSet {
		var out []ItemSet
		for _, kc := range Top(m, n) {
			s := ItemSet{Count: kc.Count}
			for _, p := range strings.Split(kc.Key, ",") {
				if id, err := strconv.Atoi(p); err == nil && id != 0 {
					s.Items = append(s.Items, item(id))
				}
			}
			if len(s.Items) > 0 {
				out = append(out, s)
			}
		}
		return out
	}
	b.Starting = sets(agg.Starting, 3)
	b.Core = sets(agg.Core, 3)
	b.Boots = sets(agg.Boots, 3)
	b.Late = sets(agg.Late, 8)
	b.Items = sets(agg.Items, 10)

	for _, kc := range Top(agg.Runes, 3) {
		if rp, ok := parseRuneKey(d, kc.Key); ok {
			rp.Count = kc.Count
			rp.Source = "riot"
			b.Runes = append(b.Runes, rp)
		}
	}
	for _, kc := range Top(agg.Spells, 3) {
		sp := SpellPair{Count: kc.Count}
		for _, p := range strings.Split(kc.Key, ",") {
			id, _ := strconv.Atoi(p)
			s := d.Spells[id]
			sp.Spells = append(sp.Spells, SpellRef{ID: id, Name: s.Name, Image: s.Image})
		}
		b.Spells = append(b.Spells, sp)
	}
	for _, kc := range Top(agg.SkillOrder, 3) {
		b.SkillOrder = append(b.SkillOrder, Skills{Order: kc.Key, Count: kc.Count})
	}
	for _, kc := range Top(agg.SkillStart, 3) {
		b.SkillStart = append(b.SkillStart, Skills{Order: kc.Key, Count: kc.Count})
	}
	return b
}

func parseRuneKey(d *ddragon.Data, key string) (RunePage, bool) {
	parts := strings.Split(key, "|")
	if len(parts) != 4 {
		return RunePage{}, false
	}
	pid, _ := strconv.Atoi(parts[0])
	sid, _ := strconv.Atoi(parts[1])
	rp := RunePage{Primary: styleRef(d, pid), Secondary: styleRef(d, sid)}
	for _, p := range strings.Split(parts[2], ",") {
		id, _ := strconv.Atoi(p)
		rp.Perks = append(rp.Perks, runeRef(d, id))
	}
	for _, p := range strings.Split(parts[3], ",") {
		id, _ := strconv.Atoi(p)
		rp.Shards = append(rp.Shards, runeRef(d, id))
	}
	return rp, true
}

func styleRef(d *ddragon.Data, id int) StyleRef {
	s := d.Styles[id]
	return StyleRef{ID: id, Name: s.Name, Icon: s.Icon}
}

func runeRef(d *ddragon.Data, id int) RuneRef {
	r := d.Runes[id]
	return RuneRef{ID: id, Name: r.Name, Icon: r.Icon, Desc: r.ShortDesc}
}

// FromLCU converts Riot's in-client recommended pages into display pages.
func FromLCU(d *ddragon.Data, pages []lcu.RecommendedPage) ([]RunePage, []SpellPair) {
	var out []RunePage
	var spells []SpellPair
	seenSpells := map[string]bool{}
	for _, p := range pages {
		rp := RunePage{Primary: styleRef(d, p.PrimaryPerkStyleID), Secondary: styleRef(d, p.SecondaryPerkStyleID), Source: "lcu"}
		for _, pk := range p.Perks {
			if pk.ID >= 5000 && pk.ID < 5100 {
				rp.Shards = append(rp.Shards, runeRef(d, pk.ID))
			} else {
				rp.Perks = append(rp.Perks, runeRef(d, pk.ID))
			}
		}
		out = append(out, rp)
		if len(p.SummonerSpellIDs) == 2 {
			key := fmt.Sprint(p.SummonerSpellIDs)
			if seenSpells[key] {
				continue
			}
			seenSpells[key] = true
			sp := SpellPair{}
			for _, id := range p.SummonerSpellIDs {
				s := d.Spells[id]
				sp.Spells = append(sp.Spells, SpellRef{ID: id, Name: s.Name, Image: s.Image})
			}
			spells = append(spells, sp)
		}
	}
	return out, spells
}
