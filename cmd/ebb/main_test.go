package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/doguyilmaz/ebb/internal/cli"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"ebb": func() { os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr)) },
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: filepath.Join("testdata", "script"),
		Setup: func(e *testscript.Env) error {
			home := filepath.Join(e.WorkDir, "home")
			e.Setenv("EBB_HOME", home)
			e.Setenv("EBB_SANDBOX", "1")
			e.Setenv("HOME", home)
			return os.MkdirAll(home, 0o755)
		},
		Cmds: scriptCmds,
	})
}
