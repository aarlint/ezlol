package api

import (
	"testing"

	"github.com/aarlint/ezlol/internal/builds"
	"github.com/aarlint/ezlol/internal/screen"
)

func lines(texts ...string) []screen.Line {
	var out []screen.Line
	for _, t := range texts {
		out = append(out, screen.Line{Text: t, Conf: 0.9, X: 0.5, Y: 0.3})
	}
	return out
}

func TestMatchHeadliner(t *testing.T) {
	cases := map[string]string{
		"Maestro":                             "maestro",
		"K/DA":                                "kda",
		"La Ilusion":                          "lailusion",
		"La Ilusión":                          "lailusion",
		"Command Line":                        "commandline",
		"Pentakill takes the stage":           "pentakill",
		"PENTAKILL":                           "", // kill announcement, not the band
		"Heartsteel":                          "", // also an item name
		"HEARTSTEEL - Headliner":              "heartsteel",
		"BEES!!!":                             "", // an augment
		"Deals true damage to nearby enemies": "",
		"K/DA Ahri":                           "", // a skin line
	}
	for text, want := range cases {
		if got := matchHeadliner(lines(text)); got != want {
			t.Errorf("matchHeadliner(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestHeadlinerEvidenceOrder(t *testing.T) {
	var h headlinerState
	h.set("arcade", "screen")
	h.inferFromOffer(&augmentOffer{Active: true, Offered: make([]builds.Augment, 4)})
	if h.current() != "maestro" {
		t.Fatalf("four cards prove Maestro over a screen read, got %q", h.current())
	}
	h.set("arcade", "screen")
	if h.current() != "maestro" {
		t.Fatal("a screen read must not replace inferred evidence")
	}
	h.set("debonair", "manual")
	h.inferFromOffer(&augmentOffer{Active: true, Offered: []builds.Augment{{Special: true, Skill: "Q"}}})
	if h.current() != "debonair" {
		t.Fatal("the player's own choice wins")
	}
	h.tick(500)
	h.tick(10) // new game
	if h.current() != "" {
		t.Fatal("a new game forgets the Headliner")
	}
	h.inferFromLevels([]livePlayer{{Level: 19}})
	if h.current() != "" {
		t.Fatal("level 19 is reachable without Superfans (HEARTSTEEL K'Sante)")
	}
	h.inferFromLevels([]livePlayer{{Level: 20}})
	if h.current() != "superfans" {
		t.Fatal("level 20 means Superfans")
	}
	if got := h.extraPickLevels(); len(got) != 1 || got[0] != 21 {
		t.Fatalf("Superfans adds a pick at 21, got %v", got)
	}
}
