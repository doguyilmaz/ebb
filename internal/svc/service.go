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
	exe, _ := os.Executable()
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return Service{Env: env, StateDir: sys.StateDir(env), Exe: exe, UID: os.Getuid()}
}

func LogPath(stateDir string) string { return filepath.Join(stateDir, "daemon.log") }

// Definition is the launchd plist and where it goes.
func (s Service) Definition() (path, content string) {
	home := s.Env.Home()
	log := LogPath(s.StateDir)
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
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
  <key>ProcessType</key><string>Background</string>
  <key>LowPriorityIO</key><true/>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`, Label, xmlEscape(s.Exe), xmlEscape(home), xmlEscape(home), xmlEscape(log), xmlEscape(log))
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
	path, content := s.Definition()
	if err := state.WriteAtomic(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	// A previous copy may still be loaded; bootstrap fails on a loaded label.
	if !sys.Sandboxed(s.Env) {
		_, _ = s.Env.Run(ctx, "", "launchctl", "bootout", fmt.Sprintf("gui/%d/%s", s.UID, Label))
	}
	return s.run(ctx, "launchctl", "bootstrap", fmt.Sprintf("gui/%d", s.UID), path)
}

// Restart asks launchd to restart the service so a new binary takes over.
func (s Service) Restart(ctx context.Context) (string, error) {
	return s.run(ctx, "launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/%s", s.UID, Label))
}

func (s Service) Uninstall(ctx context.Context) (string, error) {
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
