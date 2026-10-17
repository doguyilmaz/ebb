package config

import (
	"testing"

	"github.com/doguyilmaz/ebb/internal/sys"
)

func TestLoadDefaultsAndRoundTrip(t *testing.T) {
	home := t.TempDir()
	env := &sys.Fake{HomeDir: home, GOOS: "darwin", Envs: map[string]string{sys.HomeEnv: home}}
	c, err := Load(env)
	if err != nil || c.Scan.StaleDays != 30 || c.Scan.LargeFile != "1GiB" {
		t.Fatalf("defaults: %+v %v", c, err)
	}
	c.Scan.Threads = 4
	if err := Save(env, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(env)
	if err != nil || got.Scan.Threads != 4 || got.Scan.StaleDays != 30 {
		t.Fatalf("round trip: %+v %v", got, err)
	}
}
