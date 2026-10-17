// Package sys is the single seam between ebb and the machine it runs on.
// Everything that depends on the host (home directory, OS, clock, PATH, subprocesses)
// goes through Env so tests can substitute Fake.
package sys

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// HomeEnv overrides the user's home directory, which redirects every path ebb
// reads or writes; development and tests use it to stay away from real data.
const HomeEnv = "EBB_HOME"

// SandboxEnv, when "1", blocks destructive actions and external tools.
const SandboxEnv = "EBB_SANDBOX"

type Env interface {
	Home() string
	OS() string
	Arch() string
	Hostname() string
	Now() time.Time
	Getenv(key string) string
	LookPath(bin string) (string, error)
	Run(ctx context.Context, dir, name string, args ...string) ([]byte, error)
}

type real struct{}

func Real() Env { return real{} }

func (real) Home() string {
	if h := os.Getenv(HomeEnv); h != "" {
		return filepath.Clean(h)
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

func (real) OS() string   { return runtime.GOOS }
func (real) Arch() string { return runtime.GOARCH }

func (real) Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func (real) Now() time.Time           { return time.Now() }
func (real) Getenv(key string) string { return os.Getenv(key) }

// LookPath searches PATH, then the folders user-level installers put commands
// in: a launchd agent gets a bare PATH without them.
func (r real) LookPath(bin string) (string, error) {
	p, err := exec.LookPath(bin)
	if err == nil {
		return p, nil
	}
	home := r.Home()
	for _, dir := range []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, "go", "bin"),
		"/opt/homebrew/bin",
		"/usr/local/bin",
		"/usr/local/go/bin",
	} {
		c := filepath.Join(dir, bin)
		if st, serr := os.Stat(c); serr == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return c, nil
		}
	}
	return "", err
}

func (r real) Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	if err := SandboxRun(r, filepath.Base(name)); err != nil {
		return nil, err
	}
	if !strings.ContainsRune(name, '/') {
		if p, err := r.LookPath(name); err == nil {
			name = p
		}
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.Output()
}

// Override is base with some environment variables replaced; an empty value
// unsets one.
func Override(base Env, vars map[string]string) Env { return overlay{base, vars} }

type overlay struct {
	Env
	vars map[string]string
}

func (o overlay) Getenv(key string) string {
	if v, ok := o.vars[key]; ok {
		return v
	}
	return o.Env.Getenv(key)
}

func (o overlay) Home() string {
	if _, ok := o.vars[HomeEnv]; !ok {
		return o.Env.Home()
	}
	if h := o.vars[HomeEnv]; h != "" {
		return filepath.Clean(h)
	}
	h, _ := os.UserHomeDir()
	return h
}
