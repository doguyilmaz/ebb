package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/sys"
)

const interruptRules = `
[[rule]]
id = "test.big"
title = "Big cache"
category = "package-cache"
paths = ["~/w/big"]
safety = "regenerable"
why = "w"
impact = "i"
min_size = "1MB"
min_target = "1MB"
action = { kind = "delete" }

[[rule]]
id = "test.next"
title = "Next cache"
category = "package-cache"
paths = ["~/w/next"]
safety = "regenerable"
why = "w"
impact = "i"
min_size = "1MB"
min_target = "1MB"
action = { kind = "delete" }
`

// A Ctrl-C while ebb clean removes a folder lets that folder go completely
// and stops before the next item: killed halfway, a delete leaves the rest
// under a hidden name and nothing in the log.
func TestInterruptedCleanFinishesTheItemInHand(t *testing.T) {
	if os.Getenv("EBB_TEST_CLEAN_CHILD") == "1" {
		os.Exit(Main([]string{"clean", "test.big", "test.next"}, os.Stdout, os.Stderr))
	}
	home := t.TempDir()
	t.Setenv(sys.HomeEnv, home)
	t.Setenv(sys.SandboxEnv, "1")
	rules := filepath.Join(home, ".config", "ebb", "rules")
	if err := os.MkdirAll(rules, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rules, "t.toml"), []byte(interruptRules), 0o600); err != nil {
		t.Fatal(err)
	}
	big := filepath.Join(home, "w", "big")
	for d := range 200 {
		dir := filepath.Join(big, "d"+strconv.Itoa(d))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for f := range 200 {
			if err := os.WriteFile(filepath.Join(dir, "f"+strconv.Itoa(f)), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, p := range []string{filepath.Join(big, "blob"), filepath.Join(home, "w", "next", "blob")} {
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, make([]byte, 2<<20), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a, err := app.New(sys.Real())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Scan(context.Background(), app.ScanOptions{Root: home}, nil); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	child := exec.Command(os.Args[0], "-test.run=^TestInterruptedCleanFinishesTheItemInHand$")
	child.Env = append(os.Environ(), "EBB_TEST_CLEAN_CHILD=1")
	child.Stdin = strings.NewReader("y\n")
	child.Stdout, child.Stderr = &out, &out
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	deleting := func() bool {
		des, _ := os.ReadDir(filepath.Join(home, "w"))
		for _, d := range des {
			if strings.HasPrefix(d.Name(), ".ebb-delete-") {
				return true
			}
		}
		return false
	}
	caught, ended := false, false
	for deadline := time.Now().Add(30 * time.Second); !ended && !caught && time.Now().Before(deadline); {
		select {
		case <-done:
			ended = true
		default:
			if deleting() {
				caught = child.Process.Signal(os.Interrupt) == nil
			}
		}
	}
	if !ended {
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			child.Process.Kill()
			t.Fatalf("ebb clean did not end:\n%s", out.String())
		}
	}
	if !caught {
		t.Skipf("the folder was gone before it could be interrupted:\n%s", out.String())
	}
	ops, _ := a.Ops(0)
	if deleting() || len(ops) != 1 || ops[0].Path != big || ops[0].Result != "done" {
		t.Fatalf("half removed: %v, ops %+v\n%s", deleting(), ops, out.String())
	}
	if _, err := os.Stat(filepath.Join(home, "w", "next", "blob")); err != nil {
		t.Fatalf("the next item ran after the interrupt: %v\n%s", err, out.String())
	}
}
