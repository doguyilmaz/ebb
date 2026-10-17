package main

import "github.com/rogpeppe/go-internal/testscript"

var scriptCmds = map[string]func(ts *testscript.TestScript, neg bool, args []string){}
