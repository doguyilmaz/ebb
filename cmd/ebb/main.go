package main

import (
	"os"
	"runtime"
	"runtime/debug"

	"github.com/doguyilmaz/ebb/internal/cli"
)

// AppKit only runs on the main thread, and the background service gives it the
// main goroutine; locking it here, before main, keeps that goroutine there.
func init() { runtime.LockOSThread() }

func main() {
	// Collect at 50 % heap growth instead of 100 %: a full-disk scan then
	// peaks around 400 MB and the headroom scales with the disk, unlike a
	// fixed limit that would make the collector run nonstop on a larger one.
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(50)
	}
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
