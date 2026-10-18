package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/clean"
	"github.com/doguyilmaz/ebb/internal/sys"
)

const testRules = `
[[rule]]
id = "test.cache"
title = "Test cache"
category = "package-cache"
paths = ["~/w/cache"]
safety = "regenerable"
why = "w"
impact = "i"
min_size = "1MB"
min_target = "1MB"
action = { kind = "delete" }

[[rule]]
id = "test.dl"
title = "Test downloads"
category = "installer"
paths = ["~/w/old.dmg"]
safety = "caution"
why = "w"
impact = "i"
min_size = "1MB"
min_target = "1MB"
action = { kind = "trash" }
`

func cleanApp(t *testing.T) (*App, string) {
	t.Helper()
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".config", "ebb", "rules"), 0o755)
	os.WriteFile(filepath.Join(home, ".config", "ebb", "rules", "t.toml"), []byte(testRules), 0o600)
	env := &sys.Fake{HomeDir: home, GOOS: "darwin", Time: time.Now(),
		Envs:  map[string]string{sys.HomeEnv: home, sys.SandboxEnv: "1"},
		RunFn: func(string, string, ...string) ([]byte, error) { return nil, sys.ErrSandbox }}
	a, err := New(env)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(home, "w", "cache", "blob"), 3<<20)
	write(t, filepath.Join(home, "w", "old.dmg"), 2<<20)
	if _, err := a.Scan(context.Background(), ScanOptions{Root: home}, nil); err != nil {
		t.Fatal(err)
	}
	return a, home
}

func TestPlanExecuteUpdatesTreeAndFindings(t *testing.T) {
	a, home := cleanApp(t)
	p, err := a.Plan(clean.Request{FindingIDs: []string{"test.cache"}, Source: "cli"})
	if err != nil || p.NeedsApproval {
		t.Fatalf("%+v %v", p, err)
	}
	res, err := a.Execute(context.Background(), p.ID, nil)
	if err != nil || res.Done != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	last, _ := a.Last()
	if _, ok := last.Tree.Lookup(filepath.Join(home, "w", "cache")); ok {
		t.Fatal("deleted folder still in the tree")
	}
	for _, f := range last.Findings {
		if f.ID == "test.cache" {
			t.Fatal("finding for deleted folder still listed")
		}
	}
	fresh, _ := New(a.Env)
	if r, _ := fresh.Last(); r == nil || len(r.Findings) != len(last.Findings) {
		t.Fatal("the updated scan must be saved")
	}
}

func TestExecuteNeedsApprovalForCaution(t *testing.T) {
	a, _ := cleanApp(t)
	p, _ := a.Plan(clean.Request{FindingIDs: []string{"test.dl"}})
	if _, err := a.Execute(context.Background(), p.ID, nil); !errors.Is(err, clean.ErrNeedsApproval) {
		t.Fatalf("%v", err)
	}
	if _, err := a.Approve(p.ID); err != nil {
		t.Fatal(err)
	}
	res, err := a.Execute(context.Background(), p.ID, nil)
	if err != nil || res.Done != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := a.Execute(context.Background(), p.ID, nil); err == nil {
		t.Fatal("a plan runs once")
	}
	ops, _ := a.Ops(10)
	if err := a.Undo(ops[0].ID); err != nil {
		t.Fatal(err)
	}
}

func TestPresetSelectsCategories(t *testing.T) {
	a, _ := cleanApp(t)
	var dev uint64
	for _, p := range a.Presets() {
		if p.ID == "dev-caches" {
			dev = p.Bytes
		}
	}
	if dev < 3<<20 {
		t.Fatalf("dev-caches preset bytes %d", dev)
	}
	p, err := a.Plan(clean.Request{Preset: "dev-caches"})
	if err != nil || len(p.Items) != 1 || p.Items[0].FindingID != "test.cache" {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestIgnoreHidesUntilGrowth(t *testing.T) {
	a, home := cleanApp(t)
	if err := a.Ignore("test.cache"); err != nil {
		t.Fatal(err)
	}
	if fr, _ := a.Findings("", 0, false); hasFinding(fr.Findings, "test.cache") {
		t.Fatal("ignored finding still shown")
	}
	write(t, filepath.Join(home, "w", "cache", "more"), 3<<20)
	// A full scan: FSEvents hands a change over milliseconds after it happens.
	a.Scan(context.Background(), ScanOptions{Root: home, Full: true}, nil)
	if fr, _ := a.Findings("", 0, false); !hasFinding(fr.Findings, "test.cache") {
		t.Fatal("a finding that grew by half must come back")
	}
}

func TestNotesSurviveRescan(t *testing.T) {
	a, home := cleanApp(t)
	a.SetNote("finding:test.cache", "keep until Friday")
	a.SetNote("path:"+filepath.Join(home, "w"), "work stuff")
	a.Scan(context.Background(), ScanOptions{Root: home}, nil)
	fr, _ := a.Findings("", 0, false)
	for _, f := range fr.Findings {
		if f.ID == "test.cache" && f.UserNote != "keep until Friday" {
			t.Fatalf("%+v", f)
		}
	}
	ch, _ := a.Children(home, 10, false)
	for _, it := range ch.Items {
		if it.Name == "w" && it.Note != "work stuff" {
			t.Fatalf("%+v", it)
		}
	}
}

func TestProtectBlocksPlan(t *testing.T) {
	a, home := cleanApp(t)
	s := a.Settings()
	s.Protect = []string{"~/w/cache"}
	if _, err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	p, _ := a.Plan(clean.Request{Paths: []string{filepath.Join(home, "w", "cache")}})
	if p.Items[0].Blocked == "" {
		t.Fatalf("%+v", p.Items[0])
	}
}
