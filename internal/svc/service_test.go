package svc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestRestoreBringsBackOnlyAWantedService(t *testing.T) {
	env, _ := fake(t)
	s := Service{Env: env, StateDir: t.TempDir(), Exe: "/opt/ebb", UID: 501}
	if done, err := s.Restore(context.Background()); err != nil || done || s.Installed() {
		t.Fatalf("restored a service nobody turned on: %v %v", done, err)
	}
	if _, err := s.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Homebrew's uninstall step removes the definition on every upgrade.
	path, _ := s.Definition()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if done, err := s.Restore(context.Background()); err != nil || !done || !s.Installed() {
		t.Fatalf("a wanted service was not restored: %v %v", done, err)
	}
	if _, err := s.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if done, _ := s.Restore(context.Background()); done || s.Installed() {
		t.Fatal("a service turned off came back")
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
