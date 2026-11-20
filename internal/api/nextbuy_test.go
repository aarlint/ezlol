package api

import (
	"testing"

	"github.com/aarlint/ezlol/internal/ddragon"
	"github.com/aarlint/ezlol/internal/live"
)

func testItems() *ddragon.Data {
	it := func(id int, name string, gold int, from ...string) ddragon.Item {
		return ddragon.Item{ID: id, Name: name, Gold: gold, From: from}
	}
	return &ddragon.Data{Items: map[int]ddragon.Item{
		1001: it(1001, "Boots", 300),
		3006: it(3006, "Berserker's Greaves", 1100, "1001", "1042"),
		1042: it(1042, "Dagger", 250),
		1038: it(1038, "B.F. Sword", 1300),
		1037: it(1037, "Pickaxe", 875),
		1018: it(1018, "Cloak of Agility", 600),
		1036: it(1036, "Long Sword", 350),
		3031: it(3031, "Infinity Edge", 3450, "1038", "1037", "1018"),
		3133: it(3133, "Caulfield's Warhammer", 1050, "1036", "1036"),
		3071: it(3071, "Black Cleaver", 3000, "3133", "1028"),
		1028: it(1028, "Ruby Crystal", 400),
	}}
}

func TestNextBuyCreditsComponents(t *testing.T) {
	d := testItems()
	nb := planNextBuy(d, []int{3031, 3006, 3071}, []int{1038, 1001}, 1000)
	if nb.Target.ID != 3031 || nb.Cost != 3450 || nb.Have != 1300 || nb.Left != 2150 {
		t.Fatalf("target %+v cost %d have %d left %d", nb.Target, nb.Cost, nb.Have, nb.Left)
	}
	if nb.Complete {
		t.Fatal("1000 gold cannot complete Infinity Edge")
	}
	// 1000 gold: Pickaxe (875) fits, then nothing else (Cloak 600 > 125).
	if len(nb.Buy) != 1 || nb.Buy[0].ID != 1037 || nb.BuyCost != 875 {
		t.Fatalf("buy %+v (%d)", nb.Buy, nb.BuyCost)
	}
	if len(nb.Then) != 2 || nb.Then[0].ID != 3006 || nb.Then[1].ID != 3071 {
		t.Fatalf("then %+v", nb.Then)
	}
}

func TestNextBuyCompleteAndUpgrades(t *testing.T) {
	d := testItems()
	// Greaves owned (built from Boots): skipped. All IE parts owned: 675 to finish.
	nb := planNextBuy(d, []int{3006, 3031}, []int{3006, 1038, 1037, 1018}, 700)
	if nb.Target.ID != 3031 || nb.Left != 675 || !nb.Complete || nb.Done != 1 || nb.Total != 2 {
		t.Fatalf("%+v", nb)
	}
	if len(nb.After) != 0 {
		t.Fatalf("25 gold left buys nothing: %+v", nb.After)
	}
	// Leftover gold goes to the next item: 3000 - 800 (Greaves) = 2200 -> B.F. Sword, Pickaxe.
	nb = planNextBuy(d, []int{3006, 3031}, []int{1001}, 3000)
	if !nb.Complete || len(nb.After) != 2 || nb.After[0].ID != 1038 || nb.After[1].ID != 1037 {
		t.Fatalf("after %+v", nb.After)
	}
	// Owning only Boots does not count as owning Greaves, but credits them.
	nb = planNextBuy(d, []int{3006}, []int{1001}, 0)
	if nb.Target.ID != 3006 || nb.Left != 800 {
		t.Fatalf("%+v", nb)
	}
}

func TestNextBuyNestedComponents(t *testing.T) {
	d := testItems()
	// One Long Sword owned goes into Caulfield's inside Black Cleaver.
	nb := planNextBuy(d, []int{3071}, []int{1036}, 1100)
	if nb.Left != 2650 {
		t.Fatalf("left %d", nb.Left)
	}
	// Caulfield's needs 700 more (1050-350): fits; Ruby 400 also fits.
	ids := map[int]int{}
	for _, b := range nb.Buy {
		ids[b.ID] = b.Gold
	}
	if ids[3133] != 700 || ids[1028] != 400 || nb.BuyCost != 1100 {
		t.Fatalf("buy %+v", nb.Buy)
	}
}

func TestNextBuyWholeBuildOwned(t *testing.T) {
	nb := planNextBuy(testItems(), []int{3031}, []int{3031}, 5000)
	if nb.Target.ID != 0 || nb.Done != 1 || nb.Total != 1 {
		t.Fatalf("%+v", nb)
	}
	if planNextBuy(testItems(), nil, nil, 0) != nil {
		t.Fatal("no path should give nil")
	}
}

func TestParsePath(t *testing.T) {
	got := parsePath("3031, 3006,x,0,-4,3071")
	if len(got) != 3 || got[0] != 3031 || got[2] != 3071 {
		t.Fatalf("%v", got)
	}
}

func TestTimelineRecord(t *testing.T) {
	var tl timelineState
	for _, ts := range []float64{0, 1.5, 3, 9, 10.5, 12, 21} {
		tl.record(tlPoint{T: ts})
	}
	pts := tl.record(tlPoint{T: 22})
	// kept: 0, 10.5 (>=10 after 0), 21 (>=10 after 10.5), then 22 as the live point
	want := []float64{0, 10.5, 21, 22}
	if len(pts) != len(want) {
		t.Fatalf("%v", pts)
	}
	for i, p := range pts {
		if p.T != want[i] {
			t.Fatalf("%v", pts)
		}
	}
	// New game: time jumps back by more than 30 s.
	tl.record(tlPoint{T: 60})
	if pts = tl.record(tlPoint{T: 2}); len(pts) != 1 || pts[0].T != 2 {
		t.Fatalf("%v", pts)
	}
}

func TestMomentumMarks(t *testing.T) {
	players := []livePlayer{
		{Name: "me", Team: "ORDER", Champion: champ(1, "Jinx")},
		{Name: "them", Team: "CHAOS", Champion: champ(2, "Zed")},
	}
	evs := []live.Event{
		{Name: "FirstBlood", Time: 90, Recipient: "them"},
		{Name: "ChampionKill", Time: 90, KillerName: "them"},
		{Name: "Multikill", Time: 300, KillerName: "me", KillStreak: 2},
		{Name: "Multikill", Time: 301, KillerName: "me", KillStreak: 4},
		{Name: "DragonKill", Time: 400, KillerName: "me", DragonType: "Fire", Stolen: "True"},
		{Name: "Ace", Time: 500, AcingTeam: "ORDER"},
	}
	m := momentumMarks(evs, players, "ORDER")
	if len(m) != 4 {
		t.Fatalf("%+v", m)
	}
	if m[0].Ally || m[0].Label != "First blood · Zed" {
		t.Fatalf("first blood %+v", m[0])
	}
	if !m[1].Ally || m[1].Label != "Jinx Quadra kill" {
		t.Fatalf("multikill %+v", m[1])
	}
	if !m[2].Ally || m[2].Label != "Fire drake (stolen)" {
		t.Fatalf("dragon %+v", m[2])
	}
	if !m[3].Ally || m[3].Kind != "ace" {
		t.Fatalf("ace %+v", m[3])
	}
}
