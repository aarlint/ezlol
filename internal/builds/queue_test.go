package builds

import "testing"

func TestQueueTagAndStoreKey(t *testing.T) {
	cases := map[int]string{420: "16.17", 440: "16.17", 450: "16.17-aram", 2400: "16.17-aram", 1700: "16.17-arena"}
	for q, want := range cases {
		if got := StoreKey("16.17", q); got != want {
			t.Errorf("StoreKey(16.17, %d) = %s, want %s", q, got, want)
		}
	}
}

func TestQueueFamilies(t *testing.T) {
	tags := map[int]string{
		420: "sr", 480: "sr", 450: "aram", 2400: "aram", 2401: "aram", 2410: "aram", 2450: "aram", 3270: "aram", 3280: "aram",
		1700: "arena", 1740: "arena", 1750: "arena", 4310: "classic", 4320: "classic", 3262: "classic",
	}
	for q, want := range tags {
		if got := QueueTag(q); got != want {
			t.Errorf("QueueTag(%d) = %s, want %s", q, got, want)
		}
	}
	if IsMayhem(450) || !IsMayhem(2400) {
		t.Error("IsMayhem: plain ARAM has no augments; 2400 is Mayhem")
	}
	if IsClassic(420) || !IsClassic(4300) {
		t.Error("IsClassic: 4300-4321 are League Classic (Jade)")
	}
}
