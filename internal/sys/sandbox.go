package sys

import (
	"errors"
	"os/user"
	"path/filepath"
	"strings"
)

var ErrSandbox = errors.New("not run in sandbox")

// Read-only lookups that sandboxed sessions may still make.
var sandboxAllowed = map[string]bool{"plutil": true, "sw_vers": true, "mdfind": true}

func Sandboxed(env Env) bool { return env.Getenv(SandboxEnv) == "1" }

// Isolated reports a sandbox on a home of its own: EBB_HOME names a folder
// that is neither the user's real home nor above it. Only there may answers
// to ebb's questions come from a script.
func Isolated(env Env) bool {
	h := env.Getenv(HomeEnv)
	if !Sandboxed(env) || h == "" {
		return false
	}
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		return false
	}
	rel, err := filepath.Rel(canonical(h), canonical(u.HomeDir))
	return err == nil && (rel == ".." || strings.HasPrefix(rel, "../"))
}

// canonical resolves links and folds case, as APFS compares names.
func canonical(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	abs, _ := filepath.Abs(p)
	return strings.ToLower(abs)
}

// SandboxRun refuses, in a sandboxed session, every external tool that could
// change the machine or make results depend on it.
func SandboxRun(env Env, name string) error {
	if Sandboxed(env) && !sandboxAllowed[name] {
		return ErrSandbox
	}
	return nil
}
