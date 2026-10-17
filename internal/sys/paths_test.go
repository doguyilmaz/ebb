package sys

import (
	"path/filepath"
	"testing"
)

func TestConfigDirUnderEbbHome(t *testing.T) {
	env := &Fake{HomeDir: "/sb", GOOS: "darwin", Envs: map[string]string{HomeEnv: "/sb", "XDG_CONFIG_HOME": "/elsewhere"}}
	if got := ConfigDir(env); got != filepath.Join("/sb", ".config", "ebb") {
		t.Fatalf("got %q", got)
	}
	if got := StateDir(env); got != filepath.Join("/sb", ".local", "state", "ebb") {
		t.Fatalf("got %q", got)
	}
}

func TestConfigDirWindowsUsesAppData(t *testing.T) {
	env := &Fake{HomeDir: `C:\Users\d`, GOOS: "windows", Envs: map[string]string{"APPDATA": `C:\Users\d\AppData\Roaming`, "LOCALAPPDATA": `C:\Users\d\AppData\Local`}}
	if got := ConfigDir(env); got != filepath.Join(`C:\Users\d\AppData\Roaming`, "ebb") {
		t.Fatalf("got %q", got)
	}
	if got := StateDir(env); got != filepath.Join(`C:\Users\d\AppData\Local`, "ebb") {
		t.Fatalf("got %q", got)
	}
}

func TestStateDirXDG(t *testing.T) {
	env := &Fake{HomeDir: "/home/d", GOOS: "linux", Envs: map[string]string{"XDG_STATE_HOME": "/home/d/.xdg-state", "XDG_CONFIG_HOME": "/home/d/.xdg-config"}}
	if got := StateDir(env); got != filepath.Join("/home/d/.xdg-state", "ebb") {
		t.Fatalf("got %q", got)
	}
	if got := ConfigDir(env); got != filepath.Join("/home/d/.xdg-config", "ebb") {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultsWithoutXDG(t *testing.T) {
	env := &Fake{HomeDir: "/home/d", GOOS: "linux"}
	if got := ConfigDir(env); got != filepath.Join("/home/d", ".config", "ebb") {
		t.Fatalf("got %q", got)
	}
	if got := StateDir(env); got != filepath.Join("/home/d", ".local", "state", "ebb") {
		t.Fatalf("got %q", got)
	}
}
