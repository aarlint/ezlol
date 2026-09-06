package screen

import "testing"

func TestMatch(t *testing.T) {
	cands := map[string]string{
		Normalize("Infinite Recursion"): "1001",
		Normalize("High Roller"):        "1002",
		Normalize("Circle of Death"):    "1003",
		Normalize("Eureka"):             "1004",
	}
	lines := []Line{
		{Text: "CHOOSE AN AUGMENT", Conf: 0.9, X: 0.5, Y: 0.2},
		{Text: "Circle of Deatn", Conf: 0.6, X: 0.75, Y: 0.4}, // OCR typo, edit distance 1
		{Text: "INFINITE RECURSION", Conf: 0.8, X: 0.25, Y: 0.4},
		{Text: "Eureka", Conf: 0.9, X: 0.5, Y: 0.4},
		{Text: "High", Conf: 0.9, X: 0.1, Y: 0.9}, // too short / partial
	}
	got := OfferLayout(Match(lines, cands))
	want := []string{"1001", "1004", "1003"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i].Key != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	// Chat announcements bottom-left, stacked: must not look like an offer.
	chat := []Line{
		{Text: "Rumble chose Escape Plan", Conf: 0.9, X: 0.12, Y: 0.82},
		{Text: "Alistar chose High Roller", Conf: 0.9, X: 0.12, Y: 0.86},
		{Text: "Twitch chose Eureka", Conf: 0.9, X: 0.12, Y: 0.9},
	}
	cands["escapeplan"] = "2001"
	if got := OfferLayout(Match(chat, cands)); len(got) != 0 {
		t.Fatalf("chat lines matched as offer: %v", got)
	}
}
