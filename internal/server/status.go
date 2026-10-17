package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/doguyilmaz/ebb/internal/state"
	"github.com/doguyilmaz/ebb/internal/sys"
)

// Status is written by a running server (ebb serve or the daemon) so the CLI
// and the MCP server can find it.
type Status struct {
	PID     int       `json:"pid"`
	Port    int       `json:"port"`
	Version string    `json:"version"`
	Started time.Time `json:"started"`
	Daemon  bool      `json:"daemon"`
}

func (s Status) URL() string { return fmt.Sprintf("http://127.0.0.1:%d", s.Port) }

func statusPath(env sys.Env) string { return filepath.Join(sys.StateDir(env), "server.json") }

func WriteStatus(env sys.Env, s Status) error { return state.SaveJSON(statusPath(env), s) }

func RemoveStatus(env sys.Env) { _ = os.Remove(statusPath(env)) }

// Live returns the running server's status, if its process is alive.
func Live(env sys.Env) (Status, bool) {
	var s Status
	if err := state.LoadJSON(statusPath(env), &s); err != nil || s.PID <= 0 {
		return s, false
	}
	err := syscall.Kill(s.PID, 0)
	return s, err == nil || errors.Is(err, syscall.EPERM)
}
