package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/clean"
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

func TestNormalizeOrigin(t *testing.T) {
	for in, want := range map[string]string{"https://EBB.apps.doguyilmaz.com/": "https://ebb.apps.doguyilmaz.com", "http://localhost:3000": "http://localhost:3000"} {
		if got, ok := NormalizeOrigin(in); !ok || got != want {
			t.Errorf("%q → %q %v", in, got, ok)
		}
	}
	for _, bad := range []string{"http://evil.example", "https://x.com/path", "javascript:alert(1)", "https://u:p@x.com", ""} {
		if _, ok := NormalizeOrigin(bad); ok {
			t.Errorf("%q must be rejected", bad)
		}
	}
}

func TestCorruptLastTreeMeansRescan(t *testing.T) {
	a, root := testApp(t)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(sys.StateDir(a.Env), "last.tree"), []byte("garbage"), 0o600)
	fresh, _ := New(a.Env)
	if _, err := fresh.Last(); !errors.Is(err, ErrNeverScanned) {
		t.Fatalf("a damaged saved scan must ask for a rescan, got %v", err)
	}
}

func TestConfigBadLargeFileFails(t *testing.T) {
	a, _ := testApp(t)
	os.MkdirAll(sys.ConfigDir(a.Env), 0o755)
	os.WriteFile(filepath.Join(sys.ConfigDir(a.Env), "config.toml"), []byte("[scan]\nlarge_file = \"lots\"\n"), 0o600)
	if _, err := New(a.Env); err == nil || !strings.Contains(err.Error(), "large_file") {
		t.Fatalf("a typo must fail loudly instead of listing every file: %v", err)
	}
}

func hasFinding(fs []api.FindingView, id string) bool {
	for _, f := range fs {
		if f.ID == id {
			return true
		}
	}
	return false
}

func TestStopScan(t *testing.T) {
	a, root := testApp(t)
	if a.StopScan() {
		t.Fatal("stopped a scan that was not running")
	}
	done := make(chan error, 1)
	if err := a.StartScan(ScanOptions{Root: root}, nil, func(_ *ScanResult, err error) { done <- err }); err != nil {
		t.Fatal(err)
	}
	if !a.StopScan() {
		t.Fatal("no running scan to stop")
	}
	if err := <-done; !errors.Is(err, ErrScanStopped) {
		t.Fatalf("want ErrScanStopped, got %v", err)
	}
	if st := a.ScanState(); st.Running || st.Error != "" {
		t.Fatalf("a stopped scan is not an error: %+v", st)
	}
	if _, err := a.Last(); !errors.Is(err, ErrNeverScanned) {
		t.Fatalf("a stopped scan must not replace the last result: %v", err)
	}
}

func TestChooseFolder(t *testing.T) {
	a, _ := testApp(t)
	var script string
	a.Env.(*sys.Fake).RunFn = func(_ string, name string, args ...string) ([]byte, error) {
		script = strings.Join(args, " ")
		return []byte("/Users/me/Work/\n"), nil
	}
	got, err := a.ChooseFolder(context.Background(), "scan")
	if err != nil || got != "/Users/me/Work" {
		t.Fatalf("got %q, %v", got, err)
	}
	if !strings.Contains(script, "choose folder") {
		t.Fatalf("script %q", script)
	}
	a.Env.(*sys.Fake).RunFn = func(string, string, ...string) ([]byte, error) {
		return nil, &exec.ExitError{Stderr: []byte("execution error: User canceled. (-128)")}
	}
	if got, err := a.ChooseFolder(context.Background(), "protect"); err != nil || got != "" {
		t.Fatalf("cancel: %q, %v", got, err)
	}
	if _, err := a.ChooseFolder(context.Background(), `"); do shell script "x`); err == nil {
		t.Fatal("unknown purposes must be refused, the prompt is never built from input")
	}
}

func TestDiffWithoutEarlierScan(t *testing.T) {
	a, root := testApp(t)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root, Full: true}, nil); err != nil {
		t.Fatal(err)
	}
	d, err := a.Diff("", "", 0)
	if err != nil || d.From != nil || len(d.Changes) != 0 || d.To.Root == "" {
		t.Fatalf("a single scan compares with nothing yet: %+v %v", d, err)
	}
}

func TestExplainAndFiles(t *testing.T) {
	a, root := testApp(t)
	rule := "[[rule]]\nid = \"bun.cache\"\ntitle = \"Bun install cache\"\ncategory = \"package-cache\"\npaths = [\"" + filepath.Join(root, ".bun", "install", "cache") +
		"\"]\nsafety = \"regenerable\"\nwhy = \"w\"\nimpact = \"i\"\nmin_size = \"1MB\"\nmin_target = \"1MB\"\naction = { kind = \"delete\" }\n"
	if err := os.MkdirAll(filepath.Join(a.Env.Home(), ".config", "ebb", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Env.Home(), ".config", "ebb", "rules", "t.toml"), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := New(a.Env)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "docs", "old.iso"), 3<<20)
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(root, "docs", "old.iso"), old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root, Full: true}, nil); err != nil {
		t.Fatal(err)
	}
	ex, err := a.Explain(filepath.Join(root, ".bun", "install", "cache"))
	if err != nil {
		t.Fatal(err)
	}
	if ex.Item.Alloc < 6<<20 || len(ex.Findings) == 0 || ex.Findings[0].RuleID != "bun.cache" {
		t.Fatalf("explain %+v", ex)
	}
	if _, err := a.Explain("/etc"); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("outside the scan: %v", err)
	}
	all, err := a.Files("", 1<<20, 0, 10)
	if err != nil || len(all.Items) != 2 || all.Items[0].Alloc < all.Items[1].Alloc {
		t.Fatalf("files %+v %v", all.Items, err)
	}
	stale, _ := a.Files("", 1<<20, 365, 10)
	if len(stale.Items) != 1 || stale.Items[0].Name != "old.iso" {
		t.Fatalf("old files %+v", stale.Items)
	}
}

func TestCleanupAndScanExcludeEachOther(t *testing.T) {
	a, root := testApp(t)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	p, err := a.Plan(clean.Request{Paths: []string{filepath.Join(root, "docs", "a.txt")}, Source: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	a.running = true
	if _, err := a.Execute(context.Background(), p.ID, nil); !errors.Is(err, ErrScanRunning) {
		t.Fatalf("no cleanup while a scan edits the tree: %v", err)
	}
	a.running, a.cleaning = false, true
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); !errors.Is(err, ErrCleaning) {
		t.Fatalf("no scan while a cleanup runs: %v", err)
	}
}
