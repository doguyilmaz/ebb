package sys

import (
	"os"
	"path/filepath"
)

// Executable is this program's path as something else should start it: the
// name on PATH (Homebrew's bin link) when it is this program, since the file
// it points to sits in a versioned folder that an upgrade removes.
func Executable(env Env) string {
	exe, err := os.Executable()
	if err != nil {
		return exe
	}
	return stable(env, exe)
}

func stable(env Env, exe string) string {
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe
	}
	if p, err := env.LookPath("ebb"); err == nil {
		if rp, err := filepath.EvalSymlinks(p); err == nil && rp == real {
			return p
		}
	}
	return real
}
