package sys

import (
	"context"
	"errors"
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
