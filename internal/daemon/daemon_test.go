package daemon

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	d := &daemon{a: a, s: server.New(a, "test", "tok"), dir: sys.StateDir(env), opt: Options{Out: &strings.Builder{}}}

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
