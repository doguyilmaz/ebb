package sys

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStablePrefersTheLinkOnPath(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(dir, "Cellar", "ebb", "1.0", "bin", "ebb")
	link := filepath.Join(dir, "bin", "ebb")
	for _, d := range []string{filepath.Dir(real), filepath.Dir(link)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(real, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	env := &Fake{Bins: map[string]string{"ebb": link}}
	if got := stable(env, real); got != link {
		t.Fatalf("got %s, want the PATH link %s", got, link)
	}
	env.Bins["ebb"] = "/usr/bin/true"
	if got := stable(env, link); got != real {
		t.Fatalf("another program on PATH is not this one: got %s, want %s", got, real)
	}
}

func TestStableKeepsTheAppBundlePath(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(dir, "Applications", "ebb.app", "Contents", "MacOS", "ebb")
	link := filepath.Join(dir, "bin", "ebb")
	for _, d := range []string{filepath.Dir(real), filepath.Dir(link)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(real, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	env := &Fake{Bins: map[string]string{"ebb": link}}
	if got := stable(env, link); got != real {
		t.Fatalf("got %s, want the app's own executable %s", got, real)
	}
	if !InAppBundle(real) || InAppBundle(link) {
		t.Fatal("InAppBundle")
	}
}
