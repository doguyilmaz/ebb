package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

// Live returns the running server's status when its process is alive and
// runs this same program. A stale file whose pid was reused must not make the
// CLI hand the API token to whatever listens on that port now.
func Live(env sys.Env) (Status, bool) {
	var s Status
	if err := state.LoadJSON(statusPath(env), &s); err != nil || s.PID <= 0 {
		return s, false
	}
	// Another user's process (EPERM) is not this ebb: the token must never go to it.
	if err := syscall.Kill(s.PID, 0); err != nil {
		return s, false
	}
	return s, sameProgram(s.PID)
}

func sameProgram(pid int) bool {
	out, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	self, err := os.Executable()
	if err != nil {
		return false
	}
	return filepath.Base(strings.TrimSpace(string(out))) == filepath.Base(self)
}
