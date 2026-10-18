package sys

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSandboxBlocksExternalTools(t *testing.T) {
	t.Setenv(SandboxEnv, "1")
	if _, err := Real().Run(context.Background(), "", "xcrun", "simctl", "list"); !errors.Is(err, ErrSandbox) {
		t.Fatalf("want ErrSandbox, got %v", err)
	}
}

func TestSandboxAllowsReadOnlyTools(t *testing.T) {
	t.Setenv(SandboxEnv, "1")
	if err := SandboxRun(Real(), "plutil"); err != nil {
		t.Fatalf("plutil must stay available in the sandbox: %v", err)
	}
}

func TestIsolatedNeedsAHomeOfItsOwn(t *testing.T) {
	real, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	for _, c := range []struct {
		sandbox, home string
		want          bool
	}{
		{"1", t.TempDir(), true},
		{"1", "", false},
		{"", t.TempDir(), false},
		{"1", real, false},
		{"1", strings.ToUpper(real), false},
		{"1", filepath.Dir(real), false},
	} {
		t.Setenv(SandboxEnv, c.sandbox)
		t.Setenv(HomeEnv, c.home)
		if got := Isolated(Real()); got != c.want {
			t.Errorf("EBB_SANDBOX=%q EBB_HOME=%q: isolated %v, want %v", c.sandbox, c.home, got, c.want)
		}
	}
}
