package builds

import (
	"testing"

	"github.com/aarlint/ezlol/internal/ddragon"
)

func TestMaxOrder(t *testing.T) {
	cases := []struct {
		seq  []int
		want string
	}{
		// Q maxed first, then W, then E
		{[]int{1, 2, 3, 1, 1, 4, 1, 1, 2, 2, 4, 2, 2, 3, 3, 4, 3, 3}, "Q>W>E"},
		// game ended early: Q has most points, E started before W
		{[]int{1, 3, 2, 1, 1}, "Q>E>W"},
		// E maxed, then Q, W never maxed
		{[]int{3, 1, 2, 3, 3, 4, 3, 3, 1, 1, 4, 1, 1, 2}, "E>Q>W"},
	}
	for _, c := range cases {
		if got := maxOrder(c.seq); got != c.want {
			t.Errorf("maxOrder(%v) = %s, want %s", c.seq, got, c.want)
		}
	}
}

func TestShortPatch(t *testing.T) {
	if got := ShortPatch("16.17.8104348"); got != "16.17" {
		t.Fatalf("got %s", got)
	}
}

func testData() *ddragon.Data {
	return &ddragon.Data{
		Items: map[int]ddragon.Item{
			1055: {ID: 1055, Name: "Doran's Blade", Gold: 450},
			2003: {ID: 2003, Name: "Health Potion", Gold: 50, Tags: []string{"Consumable"}},
			3020: {ID: 3020, Name: "Sorcerer's Shoes", Gold: 1100, Tags: []string{"Boots"}},
			1001: {ID: 1001, Name: "Boots", Gold: 300, Tags: []string{"Boots"}, Into: []string{"3020"}},
			3089: {ID: 3089, Name: "Rabadon's Deathcap", Gold: 3600},
			4645: {ID: 4645, Name: "Shadowflame", Gold: 3200},
			3157: {ID: 3157, Name: "Zhonya's Hourglass", Gold: 3250},
			3135: {ID: 3135, Name: "Void Staff", Gold: 3000},
			1058: {ID: 1058, Name: "Needlessly Large Rod", Gold: 1250, Into: []string{"3089"}},
		},
	}
}

func TestIngestWithTimeline(t *testing.T) {
	d := testData()
	s := &Store{patches: map[string]*PatchData{}}
	m := &Match{}
	m.Metadata.MatchID = "NA1_1"
	m.Info.GameVersion = "16.17.1.2"
	m.Info.GameDuration = 1800
	m.Info.QueueID = 420
	m.Info.Participants = append(m.Info.Participants, struct {
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
	}{ParticipantID: 1, ChampionID: 103, TeamPosition: "MIDDLE", Win: true, Summoner1ID: 12, Summoner2ID: 4})

	tl := &Timeline{}
	type ev = struct {
		Type          string `json:"type"`
		Timestamp     int    `json:"timestamp"`
		ParticipantID int    `json:"participantId"`
		ItemID        int    `json:"itemId"`
		BeforeID      int    `json:"beforeId"`
		AfterID       int    `json:"afterId"`
		SkillSlot     int    `json:"skillSlot"`
		LevelUpType   string `json:"levelUpType"`
	}
	frame := struct {
		Events []ev `json:"events"`
	}{Events: []ev{
		{Type: "ITEM_PURCHASED", Timestamp: 5000, ParticipantID: 1, ItemID: 2003},
		{Type: "ITEM_PURCHASED", Timestamp: 5000, ParticipantID: 1, ItemID: 1055},
		{Type: "ITEM_PURCHASED", Timestamp: 5000, ParticipantID: 1, ItemID: 2003},
		{Type: "ITEM_PURCHASED", Timestamp: 400000, ParticipantID: 1, ItemID: 1058},
		{Type: "ITEM_PURCHASED", Timestamp: 400000, ParticipantID: 1, ItemID: 3135}, // undone below
		{Type: "ITEM_UNDO", Timestamp: 401000, ParticipantID: 1, BeforeID: 3135},
		{Type: "ITEM_PURCHASED", Timestamp: 500000, ParticipantID: 1, ItemID: 3020},
		{Type: "ITEM_PURCHASED", Timestamp: 600000, ParticipantID: 1, ItemID: 4645},
		{Type: "ITEM_PURCHASED", Timestamp: 800000, ParticipantID: 1, ItemID: 3089},
		{Type: "ITEM_PURCHASED", Timestamp: 1000000, ParticipantID: 1, ItemID: 3157},
		{Type: "ITEM_PURCHASED", Timestamp: 1200000, ParticipantID: 1, ItemID: 3135},
		{Type: "SKILL_LEVEL_UP", ParticipantID: 1, SkillSlot: 1, LevelUpType: "NORMAL"},
		{Type: "SKILL_LEVEL_UP", ParticipantID: 1, SkillSlot: 2, LevelUpType: "NORMAL"},
		{Type: "SKILL_LEVEL_UP", ParticipantID: 1, SkillSlot: 3, LevelUpType: "NORMAL"},
	}}
	tl.Info.Frames = append(tl.Info.Frames, frame)

	s.Ingest(d, "na1", m, tl)
	s.Ingest(d, "na1", m, tl) // duplicate must be ignored

	agg, err := s.Agg("16.17", 103, "MIDDLE")
	if err != nil {
		t.Fatal(err)
	}
	if agg.Total.Games != 1 || agg.Total.Wins != 1 {
		t.Fatalf("total = %+v", agg.Total)
	}
	if agg.Starting["1055,2003,2003"] == nil {
		t.Errorf("starting items missing: %v", keys(agg.Starting))
	}
	if agg.Core["4645,3089,3157"] == nil {
		t.Errorf("core missing: %v", keys(agg.Core))
	}
	if agg.Boots["3020"] == nil {
		t.Errorf("boots missing: %v", keys(agg.Boots))
	}
	if agg.Late["3135"] == nil {
		t.Errorf("late item missing: %v", keys(agg.Late))
	}
	if agg.Spells["4,12"] == nil {
		t.Errorf("spells missing: %v", keys(agg.Spells))
	}
	if agg.SkillStart["QWE"] == nil {
		t.Errorf("skill start missing: %v", keys(agg.SkillStart))
	}
	if !s.Seen("NA1_1") {
		t.Error("match should be seen")
	}
}

func keys(m map[string]*Count) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
