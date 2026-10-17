package main

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/rogpeppe/go-internal/testscript"
)

var scriptCmds = map[string]func(ts *testscript.TestScript, neg bool, args []string){
	// fill BYTES PATH: write a file of BYTES bytes, creating parent folders.
	"fill": func(ts *testscript.TestScript, neg bool, args []string) {
		if len(args) != 2 {
			ts.Fatalf("usage: fill BYTES PATH")
		}
		n, err := strconv.Atoi(args[0])
		ts.Check(err)
		p := ts.MkAbs(args[1])
		ts.Check(os.MkdirAll(filepath.Dir(p), 0o755))
		buf := make([]byte, n)
		for i := range buf {
			buf[i] = byte(i)
		}
		ts.Check(os.WriteFile(p, buf, 0o644))
	},
}
