package builds

import "testing"

func TestRateAugmentsByRankWithinRarity(t *testing.T) {
	var out []Augment
	for i := 0; i < 20; i++ {
		out = append(out, Augment{Rarity: "gold", WinRate: 0.6 - float64(i)*0.01, Games: 500})
	}
	out = append(out, Augment{Rarity: "gold", WinRate: 0.9, Games: 10}) // thin sample: unrated
	out = append(out, Augment{Rarity: "silver", WinRate: 0.5, Games: 500}, Augment{Rarity: "silver", WinRate: 0.4, Games: 500})
	rateAugments(out, func(a Augment) bool { return a.Games >= 200 })
	want := []int{1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5}
	for i, w := range want {
		if out[i].Tier != w {
			t.Fatalf("gold #%d: tier %d, want %d", i, out[i].Tier, w)
		}
	}
	if out[20].Tier != 0 {
		t.Fatalf("thin sample rated %d, want 0", out[20].Tier)
	}
	if out[21].Tier != 1 || out[22].Tier != 3 {
		t.Fatalf("silver tiers %d/%d, want 1/3", out[21].Tier, out[22].Tier)
	}
	// Tiers never go up while walking down the list within a rarity.
	for i := 1; i < 20; i++ {
		if out[i].Tier < out[i-1].Tier {
			t.Fatalf("tier order broken at %d", i)
		}
	}
}
