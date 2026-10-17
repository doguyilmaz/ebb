package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/state"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/walk"
)

func testApp(t *testing.T) (*App, string) {
	t.Helper()
	home := t.TempDir()
	env := &sys.Fake{HomeDir: home, GOOS: "darwin", Time: time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC),
		Envs:  map[string]string{sys.HomeEnv: home, sys.SandboxEnv: "1"},
		RunFn: func(string, string, ...string) ([]byte, error) { return nil, sys.ErrSandbox }}
	a, err := New(env)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, "data")
	write(t, filepath.Join(root, ".bun", "install", "cache", "pkg", "blob"), 6<<20)
	write(t, filepath.Join(root, "docs", "a.txt"), 2000)
	return a, root
}

func write(t *testing.T, path string, size int64) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, size)); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestScanSavesLastAndSnapshot(t *testing.T) {
	a, root := testApp(t)
	res, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Meta.Files != 2 || res.Meta.ID == "" || res.Tree.Len() == 0 || res.Volume.Capacity == 0 {
		t.Fatalf("%+v", res.Meta)
	}
	sd := sys.StateDir(a.Env)
	for _, f := range []string{"last.tree", "last.json", filepath.Join("snapshots", "index.json")} {
		if _, err := os.Stat(filepath.Join(sd, f)); err != nil {
			t.Fatalf("%s missing: %v", f, err)
		}
	}
	if fi, _ := os.Stat(filepath.Join(sd, "last.json")); fi.Mode().Perm() != 0o600 {
		t.Fatal("state files must be private")
	}
}

func TestLastRoundTrip(t *testing.T) {
	a, root := testApp(t)
	res, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := New(a.Env)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fresh.Last()
	if err != nil {
		t.Fatal(err)
	}
	if got.Tree.Len() != res.Tree.Len() || got.Meta.ID != res.Meta.ID || len(got.Findings) != len(res.Findings) {
		t.Fatalf("reloaded %+v vs %+v", got.Meta, res.Meta)
	}
}

func TestLastBeforeAnyScan(t *testing.T) {
	a, _ := testApp(t)
	if _, err := a.Last(); !errors.Is(err, ErrNeverScanned) {
		t.Fatalf("want ErrNeverScanned, got %v", err)
	}
}

func TestCurrentReusesFreshResult(t *testing.T) {
	a, root := testApp(t)
	scans := 0
	count := func(p walk.Progress) {
		if p.Done {
			scans++
		}
	}
	for range 2 {
		if _, err := a.Current(context.Background(), time.Hour, ScanOptions{Root: root}, count); err != nil {
			t.Fatal(err)
		}
	}
	if scans != 1 {
		t.Fatalf("second call within maxAge must reuse the result, scanned %d times", scans)
	}
	if _, err := a.Current(context.Background(), time.Hour, ScanOptions{Root: filepath.Join(root, "docs")}, count); err != nil || scans != 2 {
		t.Fatalf("a different root needs a new scan: %d %v", scans, err)
	}
}

func TestConcurrentScanRejected(t *testing.T) {
	a, root := testApp(t)
	a.running = true
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); !errors.Is(err, ErrScanRunning) {
		t.Fatalf("want ErrScanRunning, got %v", err)
	}
}

func TestDefaultRootSandboxed(t *testing.T) {
	a, _ := testApp(t)
	if a.DefaultRoot() != a.Env.Home() {
		t.Fatalf("sandboxed default root %q", a.DefaultRoot())
	}
	_ = state.NewToken
}
