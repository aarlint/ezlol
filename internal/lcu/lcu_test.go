package lcu

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLockfile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lockfile")
	if err := os.WriteFile(p, []byte("LeagueClient:123:54407:s3cret:https"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := readLockfile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 54407 || c.Password != "s3cret" {
		t.Fatalf("got %+v", c)
	}
}
