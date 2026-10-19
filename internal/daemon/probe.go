package daemon

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/doguyilmaz/ebb/internal/appkit"
	"github.com/doguyilmaz/ebb/internal/sys"
)

// onAC: desktops report AC Power too. Not knowing counts as on battery.
func onAC(ctx context.Context, env sys.Env) bool {
	out, err := env.Run(ctx, "", "pmset", "-g", "batt")
	return err == nil && bytes.Contains(out, []byte("'AC Power'"))
}

// idle is how long nobody touched the keyboard, mouse or trackpad. Not
// knowing counts as someone working.
func idle(ctx context.Context, env sys.Env) time.Duration {
	out, err := env.Run(ctx, "", "ioreg", "-c", "IOHIDSystem", "-d", "4")
	if err != nil {
		return 0
	}
	return parseIdle(out)
}

func parseIdle(out []byte) time.Duration {
	_, rest, ok := bytes.Cut(out, []byte(`"HIDIdleTime" = `))
	if !ok {
		return 0
	}
	end := bytes.IndexFunc(rest, func(r rune) bool { return r < '0' || r > '9' })
	if end < 0 {
		end = len(rest)
	}
	ns, err := strconv.ParseInt(string(rest[:end]), 10, 64)
	if err != nil {
		return 0
	}
	return time.Duration(ns)
}

// free is the space left for the person on the volume holding path.
func free(path string) (uint64, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, false
	}
	return st.Bavail * uint64(st.Bsize), true
}

// notify shows a macOS notification. The text is ebb's own, never a path.
// From ebb.app it is ebb's own notification, and a click opens the window;
// osascript's belongs to Script Editor, which a click opens instead.
func notify(ctx context.Context, env sys.Env, text string) error {
	if appkit.Bundled() {
		appkit.Notify("ebb", text)
		return nil
	}
	text = strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(text)
	_, err := env.Run(ctx, "", "osascript", "-e", `display notification "`+text+`" with title "ebb"`)
	return err
}
