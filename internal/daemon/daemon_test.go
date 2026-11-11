package daemon

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/server"
	"github.com/doguyilmaz/ebb/internal/svc"
	"github.com/doguyilmaz/ebb/internal/sys"
)

type machine struct {
	power string
	idle  time.Duration
	ran   []string
}

func (m *machine) run(_ string, name string, args ...string) ([]byte, error) {
	m.ran = append(m.ran, name+" "+strings.Join(args, " "))
	switch name {
	case "pmset":
		return []byte("Now drawing from '" + m.power + "'\n"), nil
	case "ioreg":
		return []byte(`    | |   "HIDIdleTime" = ` + strconv.FormatInt(int64(m.idle), 10) + "\n"), nil
	}
	return nil, sys.ErrSandbox
}

func sandbox(t *testing.T) (*app.App, *sys.Fake, *machine, string) {
	t.Helper()
	home := t.TempDir()
	root := filepath.Join(home, "data")
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "a.txt"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &machine{power: "AC Power", idle: 20 * time.Minute}
	env := &sys.Fake{HomeDir: home, GOOS: "darwin", Time: time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC),
		Envs: map[string]string{sys.HomeEnv: home, sys.SandboxEnv: "1"}, RunFn: m.run}
	a, err := app.New(env)
	if err != nil {
		t.Fatal(err)
	}
	return a, env, m, root
}

func TestParseIdle(t *testing.T) {
	out := []byte("+-o IOHIDSystem\n    | |   \"HIDIdleTime\" = 1500000000000\n    | |   \"HIDIdleTime\" = 9\n")
	if got := parseIdle(out); got != 25*time.Minute {
		t.Fatalf("got %v", got)
	}
	if parseIdle([]byte("nothing")) != 0 {
		t.Fatal("not knowing counts as someone working")
	}
}

func waitScan(t *testing.T, a *app.App) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); a.ScanState().Running; {
		if time.Now().After(deadline) {
			t.Fatal("scan did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestCheckScansOnlyOnPowerWhileIdle(t *testing.T) {
	a, env, m, root := sandbox(t)
	d := &daemon{a: a, s: server.New(a, "test", "tok"), dir: sys.StateDir(env), opt: Options{Out: &strings.Builder{}}, access: func(string) bool { return true }}

	d.check(context.Background())
	if d.st.LastWork != "" {
		t.Fatal("the first scan is the person's own")
	}
	if _, err := a.Scan(context.Background(), app.ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	env.Time = env.Time.Add(2 * time.Hour)

	m.power = "Battery Power"
	d.check(context.Background())
	if d.st.LastWork != "" || d.st.LastCheck.IsZero() {
		t.Fatalf("on battery nothing runs: %+v", d.st)
	}
	m.power, m.idle = "AC Power", 3*time.Minute
	d.check(context.Background())
	if d.st.LastWork != "" {
		t.Fatal("someone is working")
	}
	m.idle = 20 * time.Minute
	d.check(context.Background())
	// No watcher in a test: what changed is unknown, so the scan is complete.
	if d.st.LastWork != "full" {
		t.Fatalf("idle on power with changes unknown: %+v", d.st)
	}
	waitScan(t, a)
	if st := a.ScanState(); st.Error != "" {
		t.Fatal(st.Error)
	}
}

// logBuf is a log a scan can write to from its own goroutine while the test reads it.
type logBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *logBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// waitLog waits until the log holds n lines containing s.
func waitLog(t *testing.T, l *logBuf, s string, n int) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); strings.Count(l.String(), s) < n; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("waited for %d lines with %q:\n%s", n, s, l.String())
		}
	}
}

func TestCheckWaitsAfterAFailedScan(t *testing.T) {
	a, env, _, root := sandbox(t)
	out := &logBuf{}
	d := &daemon{a: a, s: server.New(a, "test", "tok"), dir: sys.StateDir(env), opt: Options{Out: out}, access: func(string) bool { return true }}
	if _, err := a.Scan(context.Background(), app.ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	// A folder where the saved tree goes makes saving the scan fail, the way
	// a full disk did: the history snapshot is written, the saved scan stays old.
	saved := filepath.Join(sys.StateDir(env), "last.tree")
	if err := os.Remove(saved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(saved, 0o755); err != nil {
		t.Fatal(err)
	}
	env.Time = env.Time.Add(2 * time.Hour)

	d.check(context.Background())
	if d.st.LastWork != "full" {
		t.Fatalf("idle on power with changes unknown: %+v", d.st)
	}
	tried := d.st.LastWorkAt
	waitLog(t, out, "full scan failed: ", 1)
	if !strings.Contains(out.String(), "last.tree") {
		t.Fatalf("the log does not say why:\n%s", out)
	}

	env.Time = env.Time.Add(5 * time.Minute)
	d.check(context.Background())
	if !d.st.LastWorkAt.Equal(tried) || a.ScanState().Running || strings.Count(out.String(), "scan started") != 1 {
		t.Fatalf("started again 5 minutes after a failed scan: %+v\n%s", d.st, out)
	}
	if n := strings.Count(out.String(), "scan failed"); n != 1 {
		t.Fatalf("%d failure lines for one failed scan:\n%s", n, out)
	}

	env.Time = env.Time.Add(30 * time.Minute)
	d.check(context.Background())
	if !d.st.LastWorkAt.After(tried) {
		t.Fatalf("never tried again: %+v\n%s", d.st, out)
	}
	waitLog(t, out, "full scan failed: ", 2)
}

func TestStoppedScanIsNotLoggedAsFailed(t *testing.T) {
	out := &logBuf{}
	d := &daemon{opt: Options{Out: out}}
	d.scanEnded(Full, app.ErrScanStopped)
	d.scanEnded(Update, nil)
	if out.String() != "" {
		t.Fatalf("logged:\n%s", out)
	}
	d.scanEnded(Update, errors.New("no space left on device"))
	if got := out.String(); !strings.HasSuffix(got, " update scan failed: no space left on device\n") {
		t.Fatalf("logged %q", got)
	}
}

func TestCheckWaitsForFullDiskAccess(t *testing.T) {
	a, env, _, root := sandbox(t)
	d := &daemon{a: a, s: server.New(a, "test", "tok"), dir: sys.StateDir(env), opt: Options{Out: &strings.Builder{}}, access: func(string) bool { return false }}
	if _, err := a.Scan(context.Background(), app.ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	env.Time = env.Time.Add(2 * time.Hour)
	d.check(context.Background())
	// Without access macOS would ask about each protected folder while nobody is there to answer.
	if d.st.LastWork != "" {
		t.Fatalf("scanned without Full Disk Access: %+v", d.st)
	}
}

func TestRunServesAndRecordsItself(t *testing.T) {
	a, env, _, _ := sandbox(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	dir := sys.StateDir(env)
	if err := svc.WriteStatus(dir, api.Daemon{PID: os.Getppid()}); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), a, Options{Port: port}); err != ErrRunning {
		t.Fatalf("a second service on the same state: %v", err)
	}
	if err := svc.WriteStatus(dir, api.Daemon{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, a, Options{Version: "test", Port: port, Every: time.Hour}) }()

	sv := svc.Local(env)
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if st, ok := server.Live(env); ok && st.Daemon && st.Port == port {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the service never served")
		}
	}
	if st := sv.Read(); !st.Running || st.PID != os.Getpid() || st.Exe == "" || st.Started.IsZero() {
		t.Fatalf("status while running: %+v", st)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if st := sv.Read(); st.Running {
		t.Fatalf("after stopping: %+v", st)
	}
}
