package main

import (
	"os"
	"runtime/debug"

	"github.com/doguyilmaz/ebb/internal/cli"
)

func main() {
	// A soft limit keeps a full-disk scan around 400 MB instead of letting the
	// heap double before each collection; GOMEMLIMIT still overrides it.
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(256 << 20)
	}
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
