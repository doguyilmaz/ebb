package sys

import (
	"os"
	"path/filepath"
	"strings"
)

// Responsibility is what macOS asks about when this process reads protected
// folders: launchd starts the service on its own, so macOS asks about ebb (its
// app when it runs from one); in a terminal it asks about the terminal.
type Responsibility struct {
	Kind string // app, binary or terminal
	Path string // the app or binary
	Name string // the terminal, when it says which one it is
}

func Responsible(env Env) Responsibility {
	exe, _ := os.Executable()
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return responsible(os.Getppid(), exe, env.Getenv("TERM_PROGRAM"))
}

var terminals = map[string]string{
	"Apple_Terminal": "Terminal", "iTerm.app": "iTerm", "vscode": "Visual Studio Code", "WarpTerminal": "Warp",
	"ghostty": "Ghostty", "WezTerm": "WezTerm", "Hyper": "Hyper", "Tabby": "Tabby", "zed": "Zed",
}

func responsible(ppid int, exe, termProgram string) Responsibility {
	if ppid != 1 {
		return Responsibility{Kind: "terminal", Name: terminals[termProgram]}
	}
	if i := strings.Index(exe, ".app/Contents/MacOS/"); i >= 0 {
		return Responsibility{Kind: "app", Path: exe[:i+len(".app")]}
	}
	return Responsibility{Kind: "binary", Path: exe}
}
