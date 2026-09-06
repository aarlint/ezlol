package watcher

import "testing"

func TestShortPatch(t *testing.T) {
	if got := shortPatch("16.17.8104348+branch"); got != "16.17" {
		t.Fatalf("got %s", got)
	}
	if got := shortPatch("weird"); got != "weird" {
		t.Fatalf("got %s", got)
	}
}
