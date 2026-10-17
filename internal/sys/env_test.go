package sys

import (
	"path/filepath"
	"testing"
)

func TestRealHomeHonorsEbbHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(HomeEnv, dir+string(filepath.Separator))
	if got := Real().Home(); got != dir {
		t.Fatalf("Home() = %q, want %q", got, dir)
	}
}

func TestRealHomeFallsBackToUserHome(t *testing.T) {
	t.Setenv(HomeEnv, "")
	if Real().Home() == "" {
		t.Fatal("Home() empty")
	}
}

func TestSandboxed(t *testing.T) {
	if Sandboxed(&Fake{Envs: map[string]string{SandboxEnv: "1"}}) != true {
		t.Fatal("want sandboxed")
	}
	if Sandboxed(&Fake{}) {
		t.Fatal("want not sandboxed")
	}
}
