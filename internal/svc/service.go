// Package svc installs ebb's background service as a launchd agent: it keeps
// the server and the change watcher running after the window is closed.
package svc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/state"
	"github.com/doguyilmaz/ebb/internal/sys"
)

const Label = "com.doguyilmaz.ebb"

var ErrUnsupported = errors.New("the background service is for macOS")

type Service struct {
	Env      sys.Env
	StateDir string
	Exe      string
	UID      int
}

// Local describes the service for the running binary.
func Local(env sys.Env) Service {
	return Service{Env: env, StateDir: sys.StateDir(env), Exe: sys.Executable(env), UID: os.Getuid()}
}

// temporary: go run, go test and anything in a temp folder disappear.
func temporary(exe string) bool {
	if strings.Contains(exe, "/go-build") {
		return true
	}
	tmps := []string{os.TempDir(), "/tmp", "/private/tmp", "/var/folders", "/private/var/folders"}
	if r, err := filepath.EvalSymlinks(os.TempDir()); err == nil {
		tmps = append(tmps, r)
	}
	for _, t := range tmps {
		if t = strings.TrimSuffix(t, "/"); strings.HasPrefix(exe, t+"/") {
			return true
		}
	}
	return false
}

var errTemporary = errors.New("this ebb is a temporary build: install it first (brew install doguyilmaz/tap/ebb), then set up the service")

func LogPath(stateDir string) string { return filepath.Join(stateDir, "daemon.log") }

// wantedPath marks that the person turned the service on. Homebrew removes the
// service on every upgrade as well as on uninstall, so this is how the new
// version tells which one it was.
func wantedPath(stateDir string) string { return filepath.Join(stateDir, "service-wanted") }

// Definition is the launchd plist and where it goes. Run from ebb.app, it names
// the app, so Login Items shows ebb instead of whoever signed it.
func (s Service) Definition() (path, content string) {
	home := s.Env.Home()
	log := LogPath(s.StateDir)
	app := ""
	if sys.InAppBundle(s.Exe) {
		app = "\n  <key>AssociatedBundleIdentifiers</key><array><string>" + Label + "</string></array>"
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>%s
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>daemon</string>
    <string>run</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key><string>/opt/homebrew/bin:/usr/local/bin:%s/.bun/bin:%s/.local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`, Label, app, xmlEscape(s.Exe), xmlEscape(home), xmlEscape(home), xmlEscape(log), xmlEscape(log))
}

func xmlEscape(v string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(v)
}

func (s Service) run(ctx context.Context, args ...string) (string, error) {
	note := strings.Join(args, " ")
	if sys.Sandboxed(s.Env) {
		return note + " (skipped in sandbox)", nil
	}
	if out, err := s.Env.Run(ctx, "", args[0], args[1:]...); err != nil {
		return "", fmt.Errorf("%s: %v %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return note, nil
}

// Install writes the plist and starts the service; in the sandbox only the
// file is written and the launchctl command is reported, never run.
func (s Service) Install(ctx context.Context) (string, error) {
	if s.Env.OS() != "darwin" {
		return "", ErrUnsupported
	}
	if temporary(s.Exe) {
		return "", errTemporary
	}
	path, content := s.Definition()
	if err := state.WriteAtomic(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	if err := state.WriteAtomic(wantedPath(s.StateDir), nil, 0o600); err != nil {
		return "", err
	}
	// A previous copy may still be loaded; bootstrap fails on a loaded label,
	// also while bootout, which returns early, is still taking it down.
	if !sys.Sandboxed(s.Env) {
		job := fmt.Sprintf("gui/%d/%s", s.UID, Label)
		_, _ = s.Env.Run(ctx, "", "launchctl", "bootout", job)
		for range 50 {
			if _, err := s.Env.Run(ctx, "", "launchctl", "print", job); err != nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return s.run(ctx, "launchctl", "bootstrap", fmt.Sprintf("gui/%d", s.UID), path)
}

// Restart asks launchd to restart the service so a new binary takes over. A
// definition an older ebb wrote is written and loaded again: launchd reads it
// only when the service is loaded, and an upgrade leaves the file as it was.
func (s Service) Restart(ctx context.Context) (string, error) {
	if s.Installed() && !s.Current() {
		return s.Install(ctx)
	}
	return s.run(ctx, "launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/%s", s.UID, Label))
}

// Current reports whether the installed definition is the one this ebb writes.
func (s Service) Current() bool {
	path, want := s.Definition()
	got, err := os.ReadFile(path)
	return err == nil && string(got) == want
}

// Restore sets the service up again after an upgrade removed it, when the
// person had it on; it reports whether it did.
func (s Service) Restore(ctx context.Context) (bool, error) {
	if _, err := os.Stat(wantedPath(s.StateDir)); err != nil {
		return false, nil
	}
	_, err := s.Install(ctx)
	return err == nil, err
}

func (s Service) Uninstall(ctx context.Context) (string, error) {
	if err := os.Remove(wantedPath(s.StateDir)); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	path, _ := s.Definition()
	// Not loaded is fine: removing the plist is what keeps it from coming back.
	note, _ := s.run(ctx, "launchctl", "bootout", fmt.Sprintf("gui/%d/%s", s.UID, Label))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return note, err
	}
	return note, nil
}

func (s Service) Installed() bool {
	path, _ := s.Definition()
	_, err := os.Stat(path)
	return err == nil
}
