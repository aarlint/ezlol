package builds

import (
	"sort"
	"strconv"
	"strings"

	"github.com/aarlint/ezlol/internal/ddragon"
)

// Match is the subset of match-v5 we consume.
type Match struct {
	Metadata struct {
		MatchID      string   `json:"matchId"`
		Participants []string `json:"participants"`
	} `json:"metadata"`
	Info struct {
		GameVersion  string `json:"gameVersion"`
		GameDuration int    `json:"gameDuration"`
		QueueID      int    `json:"queueId"`
		Participants []struct {
			ParticipantID int    `json:"participantId"`
			ChampionID    int    `json:"championId"`
			TeamPosition  string `json:"teamPosition"`
			Win           bool   `json:"win"`
			Summoner1ID   int    `json:"summoner1Id"`
			Summoner2ID   int    `json:"summoner2Id"`
			Item0         int    `json:"item0"`
			Item1         int    `json:"item1"`
			Item2         int    `json:"item2"`
			Item3         int    `json:"item3"`
			Item4         int    `json:"item4"`
			Item5         int    `json:"item5"`
			Perks         struct {
				StatPerks struct {
					Offense int `json:"offense"`
					Flex    int `json:"flex"`
					Defense int `json:"defense"`
				} `json:"statPerks"`
				Styles []struct {
					Style      int `json:"style"`
					Selections []struct {
						Perk int `json:"perk"`
					} `json:"selections"`
				} `json:"styles"`
			} `json:"perks"`
		} `json:"participants"`
	} `json:"info"`
}

// Timeline is the subset of match-v5 timeline we consume.
type Timeline struct {
	Info struct {
		Frames []struct {
			Events []struct {
				Type          string `json:"type"`
				Timestamp     int    `json:"timestamp"`
				ParticipantID int    `json:"participantId"`
				ItemID        int    `json:"itemId"`
				BeforeID      int    `json:"beforeId"`
				AfterID       int    `json:"afterId"`
				SkillSlot     int    `json:"skillSlot"`
				LevelUpType   string `json:"levelUpType"`
			} `json:"events"`
		} `json:"frames"`
	} `json:"info"`
}

type purchase struct {
	ts   int
	item int
}

type participantTimeline struct {
	purchases []purchase
	skills    []int // slot 1..4 in order
}

// ShortPatch maps "16.17.8104348" to "16.17".
func ShortPatch(v string) string {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + parts[1]
}

// QueueTag groups queue ids into build families.
func QueueTag(queueID int) string {
	switch queueID {
	case 450, 2400, 100: // ARAM, ARAM Mayhem, Butcher's Bridge
		return "aram"
	case 1700, 1710: // Arena
		return "arena"
	default:
		return "sr"
	}
}

// StoreKey names the aggregate bucket for a patch + queue family.
func StoreKey(patch string, queueID int) string {
	if tag := QueueTag(queueID); tag != "sr" {
		return patch + "-" + tag
	}
	return patch
}

// Ingest folds one match (and optional timeline) into the store.
func (s *Store) Ingest(d *ddragon.Data, platform string, m *Match, tl *Timeline) {
	patch := StoreKey(ShortPatch(m.Info.GameVersion), m.Info.QueueID)
	// Remakes carry no signal.
	if m.Info.GameDuration < 300 {
		return
	}
	per := map[int]*participantTimeline{}
	if tl != nil {
		for _, f := range tl.Info.Frames {
			for _, e := range f.Events {
				pt := per[e.ParticipantID]
				if pt == nil {
					pt = &participantTimeline{}
					per[e.ParticipantID] = pt
				}
				switch e.Type {
				case "ITEM_PURCHASED":
					pt.purchases = append(pt.purchases, purchase{e.Timestamp, e.ItemID})
				case "ITEM_UNDO":
					// undo removes the most recent purchase of the undone item
					for i := len(pt.purchases) - 1; i >= 0; i-- {
						if pt.purchases[i].item == e.BeforeID {
							pt.purchases = append(pt.purchases[:i], pt.purchases[i+1:]...)
							break
						}
					}
				case "SKILL_LEVEL_UP":
					if e.LevelUpType == "NORMAL" && e.SkillSlot >= 1 && e.SkillSlot <= 4 {
						pt.skills = append(pt.skills, e.SkillSlot)
					}
				}
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	pd := s.patch(patch, platform)
	if pd.Seen[m.Metadata.MatchID] {
		return
	}
	pd.Seen[m.Metadata.MatchID] = true
	pd.Matches++

	for _, p := range m.Info.Participants {
		role := p.TeamPosition
		if role == "" {
			role = "UNKNOWN"
		}
		byRole := pd.Champions[p.ChampionID]
		if byRole == nil {
			byRole = map[string]*Agg{}
			pd.Champions[p.ChampionID] = byRole
		}
		agg := byRole[role]
		if agg == nil {
			agg = newAgg()
			byRole[role] = agg
		}
		agg.Total.add(p.Win)

		// runes
		if len(p.Perks.Styles) == 2 {
			var prim, sub []string
			for _, sel := range p.Perks.Styles[0].Selections {
				prim = append(prim, strconv.Itoa(sel.Perk))
			}
			for _, sel := range p.Perks.Styles[1].Selections {
				sub = append(sub, strconv.Itoa(sel.Perk))
			}
			sp := p.Perks.StatPerks
			key := strconv.Itoa(p.Perks.Styles[0].Style) + "|" + strconv.Itoa(p.Perks.Styles[1].Style) + "|" +
				strings.Join(append(prim, sub...), ",") + "|" +
				strconv.Itoa(sp.Offense) + "," + strconv.Itoa(sp.Flex) + "," + strconv.Itoa(sp.Defense)
			bump(agg.Runes, key, p.Win)
		}

		// spells
		a, b := p.Summoner1ID, p.Summoner2ID
		if a > b {
			a, b = b, a
		}
		bump(agg.Spells, strconv.Itoa(a)+","+strconv.Itoa(b), p.Win)

		// items
		final := []int{p.Item0, p.Item1, p.Item2, p.Item3, p.Item4, p.Item5}
		pt := per[p.ParticipantID]
		if pt != nil && len(pt.purchases) > 0 {
			ingestTimelineItems(d, agg, pt, final, p.Win)
		} else {
			ingestFinalItems(d, agg, final, p.Win)
		}

		// skills
		if pt != nil && len(pt.skills) >= 3 {
			bump(agg.SkillStart, skillLetters(pt.skills[:3]), p.Win)
			bump(agg.SkillOrder, maxOrder(pt.skills), p.Win)
		}
	}
}

func ingestTimelineItems(d *ddragon.Data, agg *Agg, pt *participantTimeline, final []int, win bool) {
	var starting []int
	var completed []int
	boots := 0
	seen := map[int]bool{}
	for _, pu := range pt.purchases {
		if pu.ts < 90_000 {
			if it, ok := d.Items[pu.item]; ok && it.Gold > 0 {
				starting = append(starting, pu.item)
			}
			continue
		}
		if boots == 0 && d.IsBoots(pu.item) {
			boots = pu.item
		}
		if d.IsCompleted(pu.item) && !seen[pu.item] {
			seen[pu.item] = true
			completed = append(completed, pu.item)
		}
	}
	// If the game ended before 3 completed items, fall back to final inventory for the item pool
	// but do not record a core build.
	sort.Ints(starting)
	bump(agg.Starting, joinInts(starting), win)
	if boots != 0 {
		bump(agg.Boots, strconv.Itoa(boots), win)
	}
	for i, id := range completed {
		bump(agg.Items, strconv.Itoa(id), win)
		if i >= 3 {
			bump(agg.Late, strconv.Itoa(id), win)
		}
	}
	if len(completed) >= 3 {
		bump(agg.Core, joinInts(completed[:3]), win)
	}
}

func ingestFinalItems(d *ddragon.Data, agg *Agg, final []int, win bool) {
	var completed []int
	for _, id := range final {
		if d.IsBoots(id) {
			bump(agg.Boots, strconv.Itoa(id), win)
		}
		if d.IsCompleted(id) {
			completed = append(completed, id)
			bump(agg.Items, strconv.Itoa(id), win)
		}
	}
	// Without purchase order the best we can do is an unordered core.
	if len(completed) >= 3 {
		sort.Ints(completed)
		bump(agg.Core, joinInts(completed[:3]), win)
	}
}

func joinInts(ids []int) string {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = strconv.Itoa(id)
	}
	return strings.Join(s, ",")
}

var slotLetter = map[int]string{1: "Q", 2: "W", 3: "E", 4: "R"}

func skillLetters(slots []int) string {
	var b strings.Builder
	for _, s := range slots {
		b.WriteString(slotLetter[s])
	}
	return b.String()
}

// maxOrder returns e.g. "Q>W>E": the order in which basic abilities reach 5 points,
// falling back to total points then first level-up for abilities that never max.
func maxOrder(seq []int) string {
	type st struct {
		slot   int
		points int
		maxAt  int // index in seq when it hit 5, or -1
		first  int
	}
	stats := map[int]*st{1: {slot: 1, maxAt: -1, first: -1}, 2: {slot: 2, maxAt: -1, first: -1}, 3: {slot: 3, maxAt: -1, first: -1}}
	for i, s := range seq {
		x := stats[s]
		if x == nil {
			continue
		}
		if x.first < 0 {
			x.first = i
		}
		x.points++
		if x.points == 5 && x.maxAt < 0 {
			x.maxAt = i
		}
	}
	list := []*st{stats[1], stats[2], stats[3]}
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if (a.maxAt >= 0) != (b.maxAt >= 0) {
			return a.maxAt >= 0
		}
		if a.maxAt >= 0 && a.maxAt != b.maxAt {
			return a.maxAt < b.maxAt
		}
		if a.points != b.points {
			return a.points > b.points
		}
		return a.first < b.first
	})
	return slotLetter[list[0].slot] + ">" + slotLetter[list[1].slot] + ">" + slotLetter[list[2].slot]
}
