package svc

import (
	"bytes"
	"io"
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

// TrimLog keeps the newest complete lines once the service log is past max.
// launchd only ever appends to it, so dropping a prefix is safe; retaining a
// partial first line would make the next append look like a continuation of a
// message from before the trim.
func TrimLog(stateDir string, max int64) {
	p := LogPath(stateDir)
	fi, err := os.Stat(p)
	if err != nil || fi.Size() <= max {
		return
	}
	if max <= 0 {
		_ = os.Truncate(p, 0)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		return
	}
	defer f.Close()
	if _, err := f.Seek(-max, io.SeekEnd); err != nil {
		return
	}
	tail := make([]byte, int(max))
	n, err := io.ReadFull(f, tail)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return
	}
	tail = tail[:n]
	if i := bytes.IndexByte(tail, '\n'); i >= 0 {
		tail = tail[i+1:]
	} else {
		// The last max bytes are one line. Keeping a suffix would corrupt it.
		tail = nil
	}
	out, err := os.OpenFile(p, os.O_WRONLY|os.O_TRUNC, fi.Mode().Perm())
	if err != nil {
		return
	}
	_, _ = out.Write(tail)
	_ = out.Close()
}
