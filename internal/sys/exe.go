package sys

import (
	"os"
	"path/filepath"
	"strings"
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

// InAppBundle reports whether exe is an app bundle's own executable, which
// macOS knows by the bundle rather than by its path.
func InAppBundle(exe string) bool { return strings.Contains(exe, ".app/Contents/MacOS/") }

func stable(env Env, exe string) string {
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe
	}
	// The app keeps its path across upgrades; a link to it would hide the bundle.
	if InAppBundle(real) {
		return real
	}
	if p, err := env.LookPath("ebb"); err == nil {
		if rp, err := filepath.EvalSymlinks(p); err == nil && rp == real {
			return p
		}
	}
	return real
}
