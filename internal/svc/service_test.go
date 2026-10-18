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
		"<key>ProcessType</key><string>Background</string>", "/state/daemon.log", "<key>KeepAlive</key>"} {
		if !strings.Contains(content, want) {
			t.Fatalf("plist misses %q:\n%s", want, content)
		}
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
