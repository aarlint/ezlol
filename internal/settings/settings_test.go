package settings

import (
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	st := Load(p)
	if !st.Get().AutoAccept || st.Get().Platform != "na1" {
		t.Fatal("defaults")
	}
	if err := st.Update(func(s *Settings) { s.RiotAPIKey = "RGAPI-1234-abcd"; s.AutoAccept = false }); err != nil {
		t.Fatal(err)
	}
	again := Load(p)
	if again.Get().RiotAPIKey != "RGAPI-1234-abcd" || again.Get().AutoAccept {
		t.Fatalf("got %+v", again.Get())
	}
	m := again.Get().Masked()
	if m["riotApiKeyHint"] != "••••abcd" || m["riotApiKeySet"] != true {
		t.Fatalf("mask: %v", m)
	}
	if _, ok := m["riotApiKey"]; ok {
		t.Fatal("raw key must not be exposed")
	}
}
