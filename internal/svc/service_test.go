package svc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/sys"
)

func fake(t *testing.T) (*sys.Fake, *[]string) {
	t.Helper()
	home := t.TempDir()
	var ran []string
	return &sys.Fake{HomeDir: home, GOOS: "darwin", Time: time.Now(), Envs: map[string]string{sys.HomeEnv: home, sys.SandboxEnv: "1"},
		RunFn: func(_ string, name string, args ...string) ([]byte, error) {
			ran = append(ran, name+" "+strings.Join(args, " "))
			return nil, nil
		}}, &ran
}

func TestPlist(t *testing.T) {
	env, _ := fake(t)
	s := Service{Env: env, StateDir: "/state", Exe: "/opt/ebb", UID: 501}
	path, content := s.Definition()
	if path != filepath.Join(env.Home(), "Library", "LaunchAgents", "com.doguyilmaz.ebb.plist") {
		t.Fatalf("path %s", path)
	}
	for _, want := range []string{"<string>com.doguyilmaz.ebb</string>", "<string>/opt/ebb</string>\n    <string>daemon</string>\n    <string>run</string>",
		"/state/daemon.log", "<key>KeepAlive</key>"} {
		if !strings.Contains(content, want) {
			t.Fatalf("plist misses %q:\n%s", want, content)
		}
	}
	odd := Service{Env: env, StateDir: "/state", Exe: "/Apps & <Tools>/ebb.app/Contents/MacOS/ebb", UID: 501}
	if _, c := odd.Definition(); program([]byte(c)) != odd.Exe {
		t.Fatalf("read back %q from:\n%s", program([]byte(c)), c)
	}
	// The service serves the window and runs the scans a person starts, so it must not be a Background job.
	for _, never := range []string{"ProcessType", "LowPriorityIO"} {
		if strings.Contains(content, never) {
			t.Fatalf("plist has %s:\n%s", never, content)
		}
	}
}

func TestRestartWritesAnOlderDefinitionAgain(t *testing.T) {
	env, _ := fake(t)
	s := Service{Env: env, StateDir: t.TempDir(), Exe: "/opt/ebb", UID: 501}
	path, want := s.Definition()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(want, "<key>KeepAlive</key>", "<key>ProcessType</key><string>Background</string>\n  <key>KeepAlive</key>", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if s.Current() {
		t.Fatal("an older definition counts as current")
	}
	if _, err := s.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != want || !s.Current() {
		t.Fatalf("restart kept the older definition:\n%s", got)
	}
}

func TestInstallInSandboxOnlyWritesTheFile(t *testing.T) {
	env, ran := fake(t)
	s := Service{Env: env, StateDir: t.TempDir(), Exe: "/opt/ebb", UID: 501}
	note, err := s.Install(context.Background())
	if err != nil || !strings.Contains(note, "skipped in sandbox") {
		t.Fatalf("%q %v", note, err)
	}
	if len(*ran) != 0 {
		t.Fatalf("ran %q", *ran)
	}
	if !s.Installed() {
		t.Fatal("the plist is written")
	}
	if _, err := s.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	path, _ := s.Definition()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("uninstall removes the plist")
	}
}

func TestInstallRefusesATemporaryBuild(t *testing.T) {
	env, _ := fake(t)
	s := Service{Env: env, StateDir: t.TempDir(), Exe: "/var/folders/x/T/go-build123/b001/exe/ebb", UID: 501}
	if _, err := s.Install(context.Background()); err == nil || s.Installed() {
		t.Fatal("a go run binary disappears; the service must not point at it")
	}
}

func TestHealBringsBackOnlyAWantedService(t *testing.T) {
	env, ran := fake(t)
	s := Service{Env: env, StateDir: t.TempDir(), Exe: "/bin/sh", UID: 501}
	if done, err := s.Heal(context.Background()); err != nil || done || s.Installed() || len(*ran) != 0 {
		t.Fatalf("healed a service nobody turned on: %v %v %q", done, err, *ran)
	}
	if _, err := s.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Homebrew's uninstall step removes the definition on every upgrade.
	path, _ := s.Definition()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if done, err := s.Heal(context.Background()); err != nil || !done || !s.Current() {
		t.Fatalf("a wanted service was not set up again: %v %v", done, err)
	}
	if _, err := s.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if done, _ := s.Heal(context.Background()); done || s.Installed() {
		t.Fatal("a service turned off came back")
	}
}

func TestHealPointsADefinitionAtThisBinaryWhenItsOwnIsGone(t *testing.T) {
	env, _ := fake(t)
	dir := t.TempDir()
	old := Service{Env: env, StateDir: dir, Exe: "/opt/homebrew/Caskroom/ebb/0.1.0/ebb & co", UID: 501}
	if _, err := old.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := Service{Env: env, StateDir: dir, Exe: "/bin/sh", UID: 501}
	if done, err := s.Heal(context.Background()); err != nil || !done || !s.Current() {
		t.Fatalf("the definition still names a binary that is gone: %v %v", done, err)
	}
}

func TestHealAsksLaunchdOnlyWhenTheServiceIsNotRunning(t *testing.T) {
	env, ran := fake(t)
	loaded := true
	env.RunFn = func(_ string, name string, args ...string) ([]byte, error) {
		*ran = append(*ran, name+" "+strings.Join(args, " "))
		if !loaded {
			return nil, errors.New("could not find service")
		}
		return nil, nil
	}
	s := Service{Env: env, StateDir: t.TempDir(), Exe: "/bin/sh", UID: 501}
	if _, err := s.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := WriteStatus(s.StateDir, api.Daemon{PID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}
	if done, err := s.Heal(context.Background()); err != nil || done || len(*ran) != 0 {
		t.Fatalf("a running service was touched: %v %v %q", done, err, *ran)
	}
	if err := WriteStatus(s.StateDir, api.Daemon{}); err != nil {
		t.Fatal(err)
	}
	if done, err := s.Heal(context.Background()); err != nil || done || strings.Join(*ran, "; ") != "launchctl print-disabled gui/501; launchctl print gui/501/com.doguyilmaz.ebb" {
		t.Fatalf("a loaded service was set up again: %v %v %q", done, err, *ran)
	}
	loaded = false
	if done, err := s.Heal(context.Background()); err != nil || !done {
		t.Fatalf("a service launchd lost was not set up again: %v %v", done, err)
	}
}

func TestHealLeavesAServiceThePersonTurnedOff(t *testing.T) {
	env, ran := fake(t)
	disabled := "\tdisabled services = {\n\t\t\"com.apple.ScriptMenuApp\" => disabled\n\t\t\"com.doguyilmaz.ebb\" => disabled\n\t}\n"
	env.RunFn = func(_ string, name string, args ...string) ([]byte, error) {
		*ran = append(*ran, name+" "+strings.Join(args, " "))
		if args[0] == "print-disabled" {
			return []byte(disabled), nil
		}
		return nil, errors.New("could not find service")
	}
	s := Service{Env: env, StateDir: t.TempDir(), Exe: "/bin/sh", UID: 501}
	if _, err := s.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, upgraded := range []bool{false, true, true} {
		if upgraded {
			path, _ := s.Definition()
			_ = os.Remove(path)
		}
		*ran = nil
		if done, err := s.Heal(context.Background()); err != nil || done || strings.Join(*ran, "; ") != "launchctl print-disabled gui/501" {
			t.Fatalf("a service turned off was touched: %v %v %q", done, err, *ran)
		}
	}
	if s.Installed() || !s.Wanted() {
		t.Fatal("heal changed what the person chose")
	}
	disabled = "\"com.doguyilmaz.ebb\" => true\n"
	if done, err := s.Heal(context.Background()); err != nil || done {
		t.Fatalf("older macOS: a service turned off came back: %v %v", done, err)
	}
	disabled = "\"com.doguyilmaz.ebb.helper\" => disabled\n\"com.doguyilmaz.ebb\" => enabled\n"
	if done, err := s.Heal(context.Background()); err != nil || !done {
		t.Fatalf("a service turned on again in Login Items was not set up: %v %v", done, err)
	}
}

func TestHealTellsALastingFailureOnce(t *testing.T) {
	env, _ := fake(t)
	dir := t.TempDir()
	if err := os.WriteFile(wantedPath(dir), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	tmp := Service{Env: env, StateDir: dir, Exe: "/var/folders/x/T/go-build123/b001/exe/ebb", UID: 501}
	heal := func(s Service) error {
		t.Helper()
		done, err := s.Heal(context.Background())
		if done {
			t.Fatal("healed")
		}
		return err
	}
	if err := heal(tmp); err == nil || tmp.Installed() {
		t.Fatalf("a temporary build took the service: %v", err)
	}
	if err := heal(tmp); err != nil {
		t.Fatalf("the same failure was told twice: %v", err)
	}
	// Another failure is told, once.
	agents := filepath.Join(env.Home(), "Library", "LaunchAgents")
	if err := os.MkdirAll(filepath.Dir(agents), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agents, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s := Service{Env: env, StateDir: dir, Exe: "/bin/sh", UID: 501}
	if err := heal(s); err == nil {
		t.Fatal("a new failure was not told")
	}
	if err := heal(s); err != nil {
		t.Fatalf("the same failure was told twice: %v", err)
	}
	// Once the service works, the next failure is told again.
	if err := os.Remove(agents); err != nil {
		t.Fatal(err)
	}
	if done, err := s.Heal(context.Background()); err != nil || !done {
		t.Fatalf("not healed: %v %v", done, err)
	}
	path, _ := s.Definition()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := heal(tmp); err == nil {
		t.Fatal("a failure after a working service was not told")
	}
	env.GOOS = "linux"
	if err := heal(tmp); err != nil {
		t.Fatalf("healed off macOS: %v", err)
	}
}

func TestPlistNamesTheAppItRunsFrom(t *testing.T) {
	env, _ := fake(t)
	bare := Service{Env: env, StateDir: "/state", Exe: "/opt/homebrew/bin/ebb", UID: 501}
	if _, content := bare.Definition(); strings.Contains(content, "AssociatedBundleIdentifiers") {
		t.Fatal("a bare binary has no app to name")
	}
	app := Service{Env: env, StateDir: "/state", Exe: "/Applications/ebb.app/Contents/MacOS/ebb", UID: 501}
	_, content := app.Definition()
	if !strings.Contains(content, "<key>AssociatedBundleIdentifiers</key><array><string>com.doguyilmaz.ebb</string></array>") {
		t.Fatalf("Login Items would show the signer's name instead of ebb:\n%s", content)
	}
}
