package builds

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The pool, not the stats, decides which Mayhem augments exist: removed ones
// drop out, new ones show up unrated and flagged, and every pool entry can be
// recognised on screen. Files are pre-seeded in the cache, so no network.
func TestMayhemAugmentsFollowThePool(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("cherry-augments.json", `[
		{"id":1,"nameTRA":"Old Faithful","augmentSmallIconPath":"","rarity":"kGold"},
		{"id":2,"nameTRA":"Fresh Pick","augmentSmallIconPath":"","rarity":"kSilver"},
		{"id":9,"nameTRA":"Retired","augmentSmallIconPath":"","rarity":"kGold"}]`)
	write("blitz-augments-16.23.json", `{
		"1":{"id":1,"name":"ARAM_OldFaithful","displayName":"Old Faithful","description":"old","rarity":1,"enabled":true},
		"2":{"id":2,"name":"ARAM_FreshPick","displayName":"Fresh Pick","description":"<b>new</b>","rarity":0,"enabled":true},
		"3":{"id":3,"name":"GoH_PentakillAug_Q","displayName":"HEAVY","description":"q","rarity":1,"enabled":true},
		"4":{"id":4,"name":"ARAM_Benched","displayName":"Benched","description":"","rarity":1,"enabled":false},
		"5":{"id":5,"name":"ARAM_GlobalOnly","displayName":"Global Only","description":"","rarity":1,"enabled":true}}`)
	write("blitz-augments-16.22.json", `{
		"1":{"id":1,"name":"ARAM_OldFaithful","displayName":"Old Faithful","rarity":1,"enabled":true},
		"3":{"id":3,"name":"GoH_PentakillAug_Q","displayName":"HEAVY","rarity":1,"enabled":true},
		"5":{"id":5,"name":"ARAM_GlobalOnly","displayName":"Global Only","rarity":1,"enabled":true}}`)
	write("aramgg-champ-7.json", `{"championAugments":[["7","{\"augments\":{\"1\":{\"tier\":\"1\",\"win_rate\":\"0.55\",\"pick_rate\":\"0.1\",\"num_games\":\"900\"},\"9\":{\"tier\":\"1\",\"win_rate\":\"0.6\",\"pick_rate\":\"0.1\",\"num_games\":\"900\"}}}"]]}`)
	write("aramgg-global.json", `[["5","{\"tier\":\"2\",\"win_rate\":\"0.52\",\"pick_rate\":\"0.2\",\"num_games\":null}"]]`)

	c := NewCommunity(dir)
	augs, scope, err := c.Augments(context.Background(), "16.23", 7)
	if err != nil {
		t.Fatal(err)
	}
	if scope != "champion" {
		t.Errorf("scope %q, want champion", scope)
	}
	by := map[int]Augment{}
	for _, a := range augs {
		by[a.ID] = a
	}
	if _, ok := by[9]; ok {
		t.Error("augment 9 has stats but left the pool; it must be dropped")
	}
	if _, ok := by[4]; ok {
		t.Error("disabled augment 4 must be dropped")
	}
	if a := by[1]; a.Scope != "champion" || a.Games != 900 || a.New || a.Tier != 1 {
		t.Errorf("old augment: %+v", a)
	}
	if a := by[2]; !a.New || a.Scope != "none" || a.Rarity != "silver" || a.Desc != "new" || a.Tier != 0 {
		t.Errorf("new augment: %+v", a)
	}
	if a := by[5]; a.Scope != "global" || a.WinRate != 0.52 || a.Tier != 0 {
		t.Errorf("global-only augment should carry all-champion stats, unrated: %+v", a)
	}
	if a := by[3]; !a.Special || a.Skill != "Q" || a.Tier != 0 {
		t.Errorf("Pentakill pick: %+v", a)
	}
	// Within gold: the rated augment first, the Pentakill entry last.
	var gold []int
	for _, a := range augs {
		if a.Rarity == "gold" {
			gold = append(gold, a.ID)
		}
	}
	if len(gold) != 3 || gold[0] != 1 || gold[2] != 3 {
		t.Errorf("gold order %v, want [1 5 3]", gold)
	}
}

func TestPrevPatch(t *testing.T) {
	for in, want := range map[string]string{"16.23": "16.22", "16.10": "16.9", "16.1": "", "": "", "x": ""} {
		if got := prevPatch(in); got != want {
			t.Errorf("prevPatch(%q) = %q, want %q", in, got, want)
		}
	}
}
