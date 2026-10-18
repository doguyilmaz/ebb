package svc

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/state"
)

func statusPath(stateDir string) string { return filepath.Join(stateDir, "daemon.json") }

// WriteStatus records what the running service did last.
func WriteStatus(stateDir string, st api.Daemon) error {
	return state.SaveJSON(statusPath(stateDir), st)
}

// Read is the service as the CLI and the window show it: installed or not,
// running or not, and what it did last.
func (s Service) Read() api.Daemon {
	var st api.Daemon
	if err := state.LoadJSON(statusPath(s.StateDir), &st); err != nil {
		st = api.Daemon{}
	}
	st.Supported = s.Env.OS() == "darwin"
	st.Installed = s.Installed()
	st.Running = st.PID > 0 && alive(st.PID)
	if !st.Running {
		st.Waiting = false
	}
	st.Log = LogPath(s.StateDir)
	return st
}

// alive asks whether the process exists and is this user's: the service never runs as anyone else.
func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

// TrimLog empties the service log once it is past max: launchd only ever
// appends to it.
func TrimLog(stateDir string, max int64) {
	p := LogPath(stateDir)
	if fi, err := os.Stat(p); err == nil && fi.Size() > max {
		_ = os.Truncate(p, 0)
	}
}
