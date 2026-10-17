package sys

import (
	"context"
	"errors"
	"os/exec"
	"time"
)

// Fake is an Env with fixed answers for tests.
type Fake struct {
	HomeDir string
	GOOS    string
	GOARCH  string
	Host    string
	Time    time.Time
	Envs    map[string]string
	Bins    map[string]string
	RunFn   func(dir, name string, args ...string) ([]byte, error)
}

func (f *Fake) Home() string { return f.HomeDir }

func (f *Fake) OS() string {
	if f.GOOS == "" {
		return "linux"
	}
	return f.GOOS
}

func (f *Fake) Arch() string {
	if f.GOARCH == "" {
		return "arm64"
	}
	return f.GOARCH
}

func (f *Fake) Hostname() string {
	if f.Host == "" {
		return "fake-host"
	}
	return f.Host
}

func (f *Fake) Now() time.Time {
	if f.Time.IsZero() {
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return f.Time
}

func (f *Fake) Getenv(key string) string { return f.Envs[key] }

func (f *Fake) LookPath(bin string) (string, error) {
	if p, ok := f.Bins[bin]; ok {
		return p, nil
	}
	return "", exec.ErrNotFound
}

func (f *Fake) Run(_ context.Context, dir, name string, args ...string) ([]byte, error) {
	if f.RunFn == nil {
		return nil, errors.New("sys.Fake: Run not configured")
	}
	return f.RunFn(dir, name, args...)
}
