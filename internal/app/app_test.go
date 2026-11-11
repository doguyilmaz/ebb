package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
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

// holdReads makes readLast count its calls and wait for release.
func holdReads(t *testing.T) (loads *atomic.Int32, reading <-chan struct{}, release chan struct{}) {
	t.Helper()
	read := readLast
	t.Cleanup(func() { readLast = read })
	loads, started, release := new(atomic.Int32), make(chan struct{}, 100), make(chan struct{})
	readLast = func(a *App) (*ScanResult, error) {
		loads.Add(1)
		started <- struct{}{}
		<-release
		return read(a)
	}
	return loads, started, release
}

func TestColdLastLoadsOnce(t *testing.T) {
	a, root := testApp(t)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	fresh, err := New(a.Env)
	if err != nil {
		t.Fatal(err)
	}
	synctest.Test(t, func(t *testing.T) {
		loads, _, release := holdReads(t)
		const callers = 8
		got := make([]*ScanResult, callers)
		var wg sync.WaitGroup
		for i := range callers {
			wg.Go(func() {
				r, err := fresh.Last()
				if err != nil {
					t.Error(err)
				}
				got[i] = r
			})
		}
		// Every caller now reads the saved scan or waits for a read.
		synctest.Wait()
		close(release)
		wg.Wait()
		if n := loads.Load(); n != 1 {
			t.Fatalf("%d callers read the saved scan %d times", callers, n)
		}
		for _, r := range got {
			if r == nil || r != got[0] {
				t.Fatal("every caller must get the same result")
			}
		}
	})
	if r, err := fresh.Last(); err != nil || r.Tree.Len() == 0 {
		t.Fatalf("the read result stays in memory: %v", err)
	}
}

func TestOlderReadDoesNotReplaceNewerScan(t *testing.T) {
	a, root := testApp(t)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	fresh, err := New(a.Env)
	if err != nil {
		t.Fatal(err)
	}
	_, reading, release := holdReads(t)
	read := make(chan *ScanResult)
	go func() {
		r, _ := fresh.Last()
		read <- r
	}()
	<-reading
	res, err := fresh.Scan(context.Background(), ScanOptions{Root: root, Full: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	if r := <-read; r != res {
		t.Fatal("a read that finished after a scan must return the scan's result")
	}
	if r, _ := fresh.Last(); r != res {
		t.Fatal("the older saved scan replaced the newer result")
	}
}

// countReads makes readLast count its calls.
func countReads(t *testing.T) *atomic.Int32 {
	t.Helper()
	read := readLast
	t.Cleanup(func() { readLast = read })
	loads := new(atomic.Int32)
	readLast = func(a *App) (*ScanResult, error) {
		loads.Add(1)
		return read(a)
	}
	return loads
}

// waitRead fails instead of hanging when no read of the saved scan starts.
func waitRead(t *testing.T, reading <-chan struct{}) {
	t.Helper()
	select {
	case <-reading:
	case <-time.After(10 * time.Second):
		t.Fatal("the saved scan was not read again")
	}
}

// `ebb scan` in a terminal scans in a process of its own while a server
// already shows an earlier scan.
func TestLastShowsScanSavedByAnotherProcess(t *testing.T) {
	a, root := testApp(t)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	terminal, err := New(a.Env)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "docs", "b.txt"), 2000)
	res, err := terminal.Scan(context.Background(), ScanOptions{Root: root, Full: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Last()
	if err != nil || got.Meta.ID != res.Meta.ID || got.Meta.Files != 3 {
		t.Fatalf("the server still shows its own scan: %+v %v, saved %s", got.Meta, err, res.Meta.ID)
	}
	if st := a.Status("test"); st.Last == nil || st.Last.ID != res.Meta.ID {
		t.Fatalf("status %+v", st.Last)
	}
}

// `ebb clean` in a terminal cleans in a process of its own: the server must
// stop listing what is gone, its packages included.
func TestLastShowsCleanupByAnotherProcess(t *testing.T) {
	a, home := cleanApp(t)
	write(t, filepath.Join(home, "proj", "package.json"), 20)
	write(t, filepath.Join(home, "proj", "node_modules", "lib", "package.json"), 20)
	write(t, filepath.Join(home, "proj", "node_modules", "lib", "index.js"), 2<<20)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: home, Full: true}, nil); err != nil {
		t.Fatal(err)
	}
	if p, err := a.Packages(); err != nil || p.Projects != 1 {
		t.Fatalf("packages before %+v %v", p, err)
	}
	terminal, err := New(a.Env)
	if err != nil {
		t.Fatal(err)
	}
	gone := []string{filepath.Join(home, "w", "cache"), filepath.Join(home, "proj", "node_modules")}
	p, err := terminal.Plan(clean.Request{Paths: gone, Source: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.Approve(p.ID); err != nil {
		t.Fatal(err)
	}
	if res, err := terminal.Execute(context.Background(), p.ID, nil); err != nil || res.Done != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	got, err := a.Last()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range gone {
		if _, ok := got.Tree.Lookup(g); ok {
			t.Fatalf("%s was removed in the terminal and is still listed", g)
		}
	}
	if fr, _ := a.Findings("", 0, true); hasFinding(fr.Findings, "test.cache") {
		t.Fatal("the finding of a removed folder is still listed")
	}
	if p, err := a.Packages(); err != nil || p.Projects != 0 {
		t.Fatalf("packages after %+v %v", p, err)
	}
}

// What this process saves is what it holds already: reading it back would
// cost a whole tree's worth of memory and time after every scan and cleanup.
func TestOwnSavesAreNotReadBack(t *testing.T) {
	a, home := cleanApp(t)
	loads := countReads(t)
	if _, err := a.Last(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Scan(context.Background(), ScanOptions{Root: home, Full: true}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Last(); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(home, "w", "new.bin"), 2<<20)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: home}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Last(); err != nil {
		t.Fatal(err)
	}
	p, err := a.Plan(clean.Request{FindingIDs: []string{"test.cache"}, Source: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Execute(context.Background(), p.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Last(); err != nil {
		t.Fatal(err)
	}
	if n := loads.Load(); n != 0 {
		t.Fatalf("this process read its own saves back %d times", n)
	}
}

// A scan or cleanup running here sets a result of its own, newer than the
// saved one, and a cleanup edits the result in memory: a newer save from
// elsewhere waits until they are done.
func TestNewerSaveWaitsForOwnScanOrCleanup(t *testing.T) {
	a, root := testApp(t)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	mine, _ := a.Last()
	terminal, err := New(a.Env)
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := terminal.Scan(context.Background(), ScanOptions{Root: root, Full: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	set := func(running, cleaning bool) {
		a.mu.Lock()
		a.running, a.cleaning = running, cleaning
		a.mu.Unlock()
	}
	loads := countReads(t)
	for _, busy := range [][2]bool{{true, false}, {false, true}} {
		set(busy[0], busy[1])
		if r, _ := a.Last(); r != mine {
			t.Fatalf("running %v, cleaning %v: the result in memory was replaced", busy[0], busy[1])
		}
	}
	if n := loads.Load(); n != 0 {
		t.Fatalf("read a save %d times only to throw it away", n)
	}
	set(false, false)

	// A scan or cleanup that begins while the newer save is read.
	_, reading, release := holdReads(t)
	read := make(chan *ScanResult)
	go func() {
		r, _ := a.Last()
		read <- r
	}()
	waitRead(t, reading)
	set(false, true)
	close(release)
	if r := <-read; r != mine {
		t.Fatal("a read that ended during a cleanup replaced the result it edits")
	}
	set(false, false)
	if r, _ := a.Last(); r == mine || r.Meta.ID != theirs.Meta.ID {
		t.Fatal("once the cleanup is done the newer save is read")
	}
}

// A scan here that finishes while a newer save from elsewhere is read is
// newer still.
func TestOwnScanDuringReloadWins(t *testing.T) {
	a, root := testApp(t)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	terminal, err := New(a.Env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.Scan(context.Background(), ScanOptions{Root: root, Full: true}, nil); err != nil {
		t.Fatal(err)
	}
	loads, reading, release := holdReads(t)
	read := make(chan *ScanResult)
	go func() {
		r, _ := a.Last()
		read <- r
	}()
	waitRead(t, reading)
	res, err := a.Scan(context.Background(), ScanOptions{Root: root, Full: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	if r := <-read; r != res {
		t.Fatal("a read that finished after a scan must return the scan's result")
	}
	if r, _ := a.Last(); r != res || loads.Load() != 1 {
		t.Fatalf("the own scan must stay, read %d times", loads.Load())
	}
}

// A save that is gone or cannot be read is no reason to stop showing the
// scan in memory.
func TestUnreadableSaveKeepsResult(t *testing.T) {
	a, root := testApp(t)
	res, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(sys.StateDir(a.Env), "last.json")
	if err := os.Remove(saved); err != nil {
		t.Fatal(err)
	}
	if r, err := a.Last(); r != res || err != nil {
		t.Fatalf("without last.json: %v", err)
	}
	if err := state.WriteAtomic(saved, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if r, err := a.Last(); r != res || err != nil {
		t.Fatalf("with a damaged last.json: %v", err)
	}
}

// Changes are counted from the scan shown: after reading a scan another
// process made, of another folder here, the watcher follows that one.
func TestReloadedScanIsWatched(t *testing.T) {
	a, root := testApp(t)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	watching := make(chan struct{})
	go func() {
		a.Watch(ctx)
		close(watching)
	}()
	defer func() {
		cancel()
		<-watching
	}()
	watched := func() string {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.watch == nil {
			return ""
		}
		return a.watch.root
	}
	for end := time.Now().Add(10 * time.Second); watched() == "" && time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
	}
	if watched() == "" {
		t.Fatal("not watching")
	}
	terminal, err := New(a.Env)
	if err != nil {
		t.Fatal(err)
	}
	res, err := terminal.Scan(context.Background(), ScanOptions{Root: filepath.Join(root, "docs"), Full: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r, err := a.Last(); err != nil || r.Meta.ID != res.Meta.ID {
		t.Fatalf("%v", err)
	}
	if got := watched(); got != res.Tree.RootPath() {
		t.Fatalf("watching %s, the scan shown is of %s", got, res.Tree.RootPath())
	}
}

// Every page request re-reads the settings (a CORS check, a paired page's
// token), so they change under a scan and under other requests all the time.
func TestSettingsReloadsDoNotRaceWithScans(t *testing.T) {
	a, root := testApp(t)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				a.WebOriginAllowed("https://example.com")
			}
		}
	})
	for range 3 {
		if _, err := a.Scan(context.Background(), ScanOptions{Root: root, Full: true}, nil); err != nil {
			t.Error(err)
		}
		_ = a.Status("test")
	}
	close(stop)
	wg.Wait()
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

func TestDuplicateSearchAndCleanup(t *testing.T) {
	a, root := testApp(t)
	write(t, filepath.Join(root, "docs", "x.bin"), 3<<20)
	write(t, filepath.Join(root, "docs", "copy", "x.bin"), 3<<20)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.FindDuplicates(context.Background(), "/etc", 0, nil); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("outside the scan: %v", err)
	}
	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.FindDuplicates(stopped, root, 0, nil); err == nil || !a.Duplicates().Stopped {
		t.Fatalf("a stopped search says so: %v %+v", err, a.Duplicates())
	}
	// Home is outside this scan, so the default is the scanned folder.
	res, err := a.FindDuplicates(context.Background(), "", 0, nil)
	if err != nil || len(res.Groups) != 1 || len(res.Groups[0].Copies) != 2 || a.Duplicates().Stopped {
		t.Fatalf("%+v %v", res, err)
	}
	extra := res.Groups[0].Copies[1].Path
	p, err := a.Plan(clean.Request{Paths: []string{extra}, Source: "ui"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Approve(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Execute(context.Background(), p.ID, nil); err != nil {
		t.Fatal(err)
	}
	if st := a.Duplicates(); st.Result == nil || len(st.Result.Groups) != 0 {
		t.Fatalf("a group left with one copy is gone: %+v", st.Result)
	}
	a.dupes.running = true
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); !errors.Is(err, ErrSearching) {
		t.Fatalf("no scan during a search: %v", err)
	}
}

// A search from before a restart is what the window shows until the next
// one: a cleanup since must not leave a removed copy in it.
func TestCleanupForgetsCopiesOfSearchBeforeRestart(t *testing.T) {
	a, root := testApp(t)
	write(t, filepath.Join(root, "docs", "x.bin"), 3<<20)
	write(t, filepath.Join(root, "docs", "copy", "x.bin"), 3<<20)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	res, err := a.FindDuplicates(context.Background(), "", 0, nil)
	if err != nil || len(res.Groups) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	restarted, err := New(a.Env)
	if err != nil {
		t.Fatal(err)
	}
	extra := res.Groups[0].Copies[1].Path
	p, err := restarted.Plan(clean.Request{Paths: []string{extra}, Source: "ui"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Approve(p.ID); err != nil {
		t.Fatal(err)
	}
	if res, err := restarted.Execute(context.Background(), p.ID, nil); err != nil || res.Done != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if st := restarted.Duplicates(); st.Result == nil || len(st.Result.Groups) != 0 {
		t.Fatalf("a removed copy is still listed: %+v", st.Result)
	}
}
