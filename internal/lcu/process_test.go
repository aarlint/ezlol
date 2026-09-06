package lcu

import "testing"

func TestParseProcessList(t *testing.T) {
	out := "foo\n/Applications/League of Legends.app/Contents/LoL/League of Legends.app/Contents/MacOS/LeagueClientUx --riotclient-auth-token=x --app-port=54407 --remoting-auth-token=abc123 --region=NA\n"
	c, err := parseProcessList(out)
	if err != nil || c.Port != 54407 || c.Password != "abc123" {
		t.Fatalf("got %+v %v", c, err)
	}
	if _, err := parseProcessList("nothing here"); err == nil {
		t.Fatal("expected error")
	}
}
