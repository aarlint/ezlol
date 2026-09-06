package api

import (
	"testing"

	"github.com/aarlint/ezlol/internal/ddragon"
)

func champ(id int, name string) ddragon.Champion { return ddragon.Champion{ID: id, Name: name} }

func TestThreatHint(t *testing.T) {
	players := []livePlayer{
		{Team: "ORDER", ItemAP: 200, ItemMR: 0},
		{Team: "ORDER", ItemAD: 40},
		{Team: "CHAOS", ItemAD: 300}, // me
	}
	if got := threatHint(players, "CHAOS"); got != "enemy items lean AP — magic resist" {
		t.Fatalf("got %q", got)
	}
	armor := []livePlayer{{Team: "ORDER", ItemAD: 100, ItemArmor: 200}}
	if got := threatHint(armor, "CHAOS"); got != "enemy items lean AD — armor · they are stacking armor — armor pen / % pen" {
		t.Fatalf("got %q", got)
	}
	if got := threatHint(nil, "CHAOS"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestFocusTarget(t *testing.T) {
	s := &Server{watcher: nil}
	// watcher is nil so info lookups are skipped; scoring uses items and kills only.
	players := []livePlayer{
		{Team: "ORDER", Champion: champ(1, "Tank"), ItemArmor: 200, ItemHP: 900},
		{Team: "ORDER", Champion: champ(2, "Carry"), ItemAD: 200, Kills: 6},
		{Team: "CHAOS", Champion: champ(3, "Me"), ItemAD: 300},
	}
	name, why := s.focusTargetNoInfo(players, "CHAOS")
	if name != "Carry" {
		t.Fatalf("focus = %s (%s), want Carry", name, why)
	}
}
