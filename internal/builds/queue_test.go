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
