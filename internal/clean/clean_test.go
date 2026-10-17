package clean

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/sys"
)

func sandboxEnv(t *testing.T) (*sys.Fake, string) {
	t.Helper()
	home := t.TempDir()
	return &sys.Fake{HomeDir: home, GOOS: "darwin", Time: time.Now(),
		Envs:  map[string]string{sys.HomeEnv: home, sys.SandboxEnv: "1"},
		RunFn: func(string, string, ...string) ([]byte, error) { return nil, errors.New("not running") }}, home
}

func mkfile(t *testing.T, p string, n int) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func planner(env *sys.Fake, fs ...finding.Finding) *Planner {
	return &Planner{Env: env, Findings: fs, Roots: []string{env.Home()}, AutoExecute: "regenerable"}
}

func findingFor(path string, s finding.Safety, a finding.Action) finding.Finding {
	return finding.Finding{ID: "f:" + filepath.Base(path), RuleID: "r", Title: "t", Category: "package-cache", Safety: s,
		Targets: []finding.Target{{Path: path, Bytes: 1 << 20, Alloc: 1 << 20}}, Actions: []finding.Action{a}}
}

func planFor(t *testing.T, env *sys.Fake, path string, s finding.Safety) *Plan {
	t.Helper()
	f := findingFor(path, s, finding.Action{Kind: "delete"})
	p, err := planner(env, f).Plan(Request{FindingIDs: []string{f.ID}, Source: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	p.Approved = true
	return p
}

func executor(env *sys.Fake) *Executor {
	return &Executor{Env: env, Log: OpenOpLog(sys.StateDir(env)), Trash: newTrasher(env), Guard: Guard{Home: env.Home(), Roots: []string{env.Home()}}}
}

func TestExecuteDeletes(t *testing.T) {
	env, home := sandboxEnv(t)
	target := filepath.Join(home, "w", "cache")
	mkfile(t, filepath.Join(target, "f"), 1<<20)
	res, err := executor(env).Execute(context.Background(), planFor(t, env, target, finding.Regenerable), nil)
	if err != nil || res.Done != 1 || exists(target) || res.Freed != 1<<20 {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestExecuteRefusesSwappedInode(t *testing.T) {
	env, home := sandboxEnv(t)
	target := filepath.Join(home, "w", "cache")
	mkfile(t, filepath.Join(target, "f"), 1<<20)
	p := planFor(t, env, target, finding.Regenerable)
	os.RemoveAll(target)
	mkfile(t, filepath.Join(target, "other"), 1<<20)
	res, _ := executor(env).Execute(context.Background(), p, nil)
	if res.Refused != 1 || !exists(filepath.Join(target, "other")) {
		t.Fatalf("swapped target must be refused: %+v", res)
	}
}

func TestExecuteRefusesSymlinkSwap(t *testing.T) {
	env, home := sandboxEnv(t)
	target := filepath.Join(home, "w", "cache")
	precious := filepath.Join(home, "w", "precious")
	mkfile(t, filepath.Join(target, "f"), 1<<20)
	mkfile(t, filepath.Join(precious, "keep"), 1<<20)
	p := planFor(t, env, target, finding.Regenerable)
	os.RemoveAll(target)
	os.Symlink(precious, target)
	res, _ := executor(env).Execute(context.Background(), p, nil)
	if !exists(filepath.Join(precious, "keep")) || res.Refused != 1 {
		t.Fatalf("followed a swapped-in symlink: %+v", res)
	}
}

func TestGuardProtectedPaths(t *testing.T) {
	g := Guard{Home: "/Users/h", Roots: []string{"/"}}
	for _, p := range []string{"/", "/System", "/System/Library/x", "/Users/h", "/Users/h/Documents", "/Users/h/Library",
		"/Users/h/Library/Keychains/login.keychain-db", "/usr/bin/ls", "/Applications", "/Users/h/Library/Mobile Documents/x",
		"/Users/h/Library/CloudStorage/Dropbox/a", "/etc/hosts", "/Volumes/Backup", "relative/x", "/Users/h/a/../../../etc"} {
		if err := g.Check(p); err == nil {
			t.Errorf("%s must be protected", p)
		}
	}
	for _, p := range []string{"/usr/local/x", "/Users/h/Library/Caches/x", "/Applications/Xcode-16.4.app", "/Users/h/.bun/install/cache"} {
		if err := g.Check(p); err != nil {
			t.Errorf("%s must be allowed: %v", p, err)
		}
	}
	g2 := Guard{Home: "/Users/h", Roots: []string{"/Users/h/Developer"}, Protect: []string{"~/Developer/keep/**"}}
	if g2.Check("/Users/h/.bun/x") == nil || g2.Check("/Users/h/Developer/keep/a") == nil || g2.Check("/Users/h/Developer/x/y") != nil {
		t.Fatal("roots and user protect list")
	}
}

func TestPlanDefaultsBySafety(t *testing.T) {
	env, home := sandboxEnv(t)
	for _, n := range []string{"a", "b", "c"} {
		mkfile(t, filepath.Join(home, n, "f"), 10)
	}
	fs := []finding.Finding{
		findingFor(filepath.Join(home, "w", "a"), finding.UserData, finding.Action{Kind: "delete"}),
		findingFor(filepath.Join(home, "w", "b"), finding.System, finding.Action{Kind: "delete"}),
		findingFor(filepath.Join(home, "w", "c"), finding.Caution, finding.Action{Kind: "trash"}),
	}
	p, err := planner(env, fs...).Plan(Request{FindingIDs: []string{fs[0].ID, fs[1].ID, fs[2].ID}})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, it := range p.Items {
		kinds[filepath.Base(it.Path)] = it.Action.Kind
	}
	if kinds["a"] != "trash" || kinds["b"] != "manual" || kinds["c"] != "trash" {
		t.Fatalf("user data goes to the Trash, system is manual only: %v", kinds)
	}
}

func TestPlanFallbackWhenToolMissing(t *testing.T) {
	env, home := sandboxEnv(t)
	mkfile(t, filepath.Join(home, "w", "cache", "f"), 10)
	f := findingFor(filepath.Join(home, "w", "cache"), finding.Regenerable, finding.Action{Kind: "command", Argv: []string{"bun", "pm", "cache", "rm"}, Requires: "bun", Fallback: "delete"})
	p, _ := planner(env, f).Plan(Request{FindingIDs: []string{f.ID}})
	if p.Items[0].Action.Kind != "delete" || !strings.Contains(strings.Join(p.Items[0].Warnings, " "), "bun") {
		t.Fatalf("%+v", p.Items[0])
	}
	env.Bins = map[string]string{"bun": "/opt/bun"}
	p, _ = planner(env, f).Plan(Request{FindingIDs: []string{f.ID}})
	if p.Items[0].Action.Kind != "command" || strings.Join(p.Items[0].Command, " ") != "bun pm cache rm" {
		t.Fatalf("%+v", p.Items[0])
	}
}

func TestPlanBlockedWhileRunning(t *testing.T) {
	env, home := sandboxEnv(t)
	mkfile(t, filepath.Join(home, "w", "dd", "f"), 10)
	env.RunFn = func(_, name string, args ...string) ([]byte, error) {
		if name == "pgrep" && args[len(args)-1] == "Xcode" {
			return []byte("123\n"), nil
		}
		return nil, errors.New("no")
	}
	f := findingFor(filepath.Join(home, "w", "dd"), finding.Regenerable, finding.Action{Kind: "delete", Quit: []string{"Xcode"}})
	p, _ := planner(env, f).Plan(Request{FindingIDs: []string{f.ID}})
	if !strings.Contains(p.Items[0].Blocked, "Quit Xcode") {
		t.Fatalf("%+v", p.Items[0])
	}
}

func TestNeedsApproval(t *testing.T) {
	env, home := sandboxEnv(t)
	mkfile(t, filepath.Join(home, "w", "r", "f"), 10)
	mkfile(t, filepath.Join(home, "w", "c", "f"), 10)
	reg := findingFor(filepath.Join(home, "w", "r"), finding.Regenerable, finding.Action{Kind: "delete"})
	cau := findingFor(filepath.Join(home, "w", "c"), finding.Caution, finding.Action{Kind: "trash"})
	if p, _ := planner(env, reg).Plan(Request{FindingIDs: []string{reg.ID}}); p.NeedsApproval {
		t.Fatal("regenerable-only plans may run without a second confirmation")
	}
	if p, _ := planner(env, reg, cau).Plan(Request{FindingIDs: []string{reg.ID, cau.ID}}); !p.NeedsApproval {
		t.Fatal("anything riskier needs approval")
	}
	pl := planner(env, reg)
	pl.AutoExecute = "never"
	if p, _ := pl.Plan(Request{FindingIDs: []string{reg.ID}}); !p.NeedsApproval {
		t.Fatal("auto_execute=never always asks")
	}
	p, _ := planner(env, reg, cau).Plan(Request{FindingIDs: []string{reg.ID, cau.ID}})
	if _, err := executor(env).Execute(context.Background(), p, nil); !errors.Is(err, ErrNeedsApproval) {
		t.Fatalf("unapproved plan executed: %v", err)
	}
}

func TestExpiredPlan(t *testing.T) {
	env, home := sandboxEnv(t)
	mkfile(t, filepath.Join(home, "w", "x", "f"), 10)
	p := planFor(t, env, filepath.Join(home, "w", "x"), finding.Regenerable)
	p.Expires = time.Now().Add(-time.Second)
	if _, err := executor(env).Execute(context.Background(), p, nil); !errors.Is(err, ErrExpired) {
		t.Fatalf("%v", err)
	}
}

func TestTrashAndUndo(t *testing.T) {
	env, home := sandboxEnv(t)
	target := filepath.Join(home, "w", "old.dmg")
	mkfile(t, target, 1000)
	f := findingFor(target, finding.Caution, finding.Action{Kind: "trash"})
	p, _ := planner(env, f).Plan(Request{FindingIDs: []string{f.ID}})
	p.Approved = true
	ex := executor(env)
	res, err := ex.Execute(context.Background(), p, nil)
	if err != nil || res.Done != 1 || exists(target) || res.Ops[0].TrashURL == "" {
		t.Fatalf("%+v %v", res, err)
	}
	if err := Undo(env, ex.Log, res.Ops[0].ID); err != nil || !exists(target) {
		t.Fatalf("undo: %v", err)
	}
	ops, _ := ex.Log.List(10)
	if len(ops) != 2 || ops[0].Action != "undo" {
		t.Fatalf("oplog %+v", ops)
	}
}

func TestManualNeverExecuted(t *testing.T) {
	env, home := sandboxEnv(t)
	target := filepath.Join(home, "w", "Docker.raw")
	mkfile(t, target, 1000)
	f := findingFor(target, finding.Caution, finding.Action{Kind: "manual", Hint: "docker system prune"})
	p, _ := planner(env, f).Plan(Request{FindingIDs: []string{f.ID}})
	p.Approved = true
	res, _ := executor(env).Execute(context.Background(), p, nil)
	if !exists(target) || res.Done != 0 || res.Manual != 1 {
		t.Fatalf("%+v", res)
	}
}

func TestRefuseRoot(t *testing.T) {
	env, home := sandboxEnv(t)
	mkfile(t, filepath.Join(home, "w", "x", "f"), 10)
	ex := executor(env)
	ex.euid = func() int { return 0 }
	if _, err := ex.Execute(context.Background(), planFor(t, env, filepath.Join(home, "w", "x"), finding.Regenerable), nil); err == nil {
		t.Fatal("running as root must be refused")
	}
}

func TestSandboxRefusesOutsideHome(t *testing.T) {
	env, _ := sandboxEnv(t)
	outside := t.TempDir()
	mkfile(t, filepath.Join(outside, "f"), 10)
	ex := executor(env)
	ex.Guard.Roots = []string{"/"}
	p := planFor(t, env, outside, finding.Regenerable)
	res, _ := ex.Execute(context.Background(), p, nil)
	if !exists(filepath.Join(outside, "f")) || res.Done != 0 {
		t.Fatal("a sandboxed session deleted outside its home")
	}
}

func TestNestedItemsDeduped(t *testing.T) {
	env, home := sandboxEnv(t)
	mkfile(t, filepath.Join(home, "w", "a", "b", "f"), 10)
	outer := findingFor(filepath.Join(home, "w", "a"), finding.Regenerable, finding.Action{Kind: "delete"})
	inner := findingFor(filepath.Join(home, "w", "a", "b"), finding.Regenerable, finding.Action{Kind: "delete"})
	inner.ID = "inner"
	p, _ := planner(env, outer, inner).Plan(Request{FindingIDs: []string{inner.ID, outer.ID}})
	if len(p.Items) != 1 || p.Items[0].Path != filepath.Join(home, "w", "a") {
		t.Fatalf("%+v", p.Items)
	}
}

func TestPlanArbitraryPathIsCautiousTrash(t *testing.T) {
	env, home := sandboxEnv(t)
	mkfile(t, filepath.Join(home, "w", "stuff", "f"), 10)
	p, err := planner(env).Plan(Request{Paths: []string{filepath.Join(home, "w", "stuff")}})
	if err != nil || len(p.Items) != 1 || p.Items[0].Action.Kind != "trash" || p.Items[0].Safety != finding.Caution || !p.NeedsApproval {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestGuardCaseVariants(t *testing.T) {
	g := Guard{Home: "/Users/h", Roots: []string{"/"}}
	for _, p := range []string{"/users/h/library/keychains/x", "/SYSTEM/Library/x", "/users/h/Documents", "/Users/H/Library/Mobile Documents/x", "/USR/bin/x"} {
		if err := g.Check(p); err == nil {
			t.Errorf("APFS ignores case, so %s is protected too", p)
		}
	}
}

func TestGuardSymlinkIntoProtected(t *testing.T) {
	_, home := sandboxEnv(t)
	keychains := filepath.Join(home, "Library", "Keychains")
	mkfile(t, filepath.Join(keychains, "login.keychain-db"), 10)
	os.MkdirAll(filepath.Join(home, "w"), 0o755)
	if err := os.Symlink(keychains, filepath.Join(home, "w", "alias")); err != nil {
		t.Fatal(err)
	}
	g := Guard{Home: home, Roots: []string{home}}
	if err := g.Check(filepath.Join(home, "w", "alias", "login.keychain-db")); err == nil {
		t.Fatal("a symlinked parent must not lead into a protected folder")
	}
}

func TestExecuteDeleteRechecksAfterRename(t *testing.T) {
	env, home := sandboxEnv(t)
	target := filepath.Join(home, "w", "cache")
	mkfile(t, filepath.Join(target, "f"), 1<<20)
	res, err := executor(env).Execute(context.Background(), planFor(t, env, target, finding.Regenerable), nil)
	if err != nil || res.Done != 1 || exists(target) {
		t.Fatalf("%+v %v", res, err)
	}
	des, _ := os.ReadDir(filepath.Join(home, "w"))
	for _, d := range des {
		if strings.HasPrefix(d.Name(), ".ebb-delete-") {
			t.Fatal("temporary name left behind")
		}
	}
}

func TestGuardVolumeRootChildren(t *testing.T) {
	g := Guard{Home: "/Users/h", Roots: []string{"/", "/System/Volumes/Data"}}
	for _, p := range []string{"/System/Volumes/Data/.DocumentRevisions-V100", "/.Spotlight-V100", "/private", "/private/var",
		"/var", "/tmp", "/private/tmp", "/System/Volumes/Data/private/var", "/Volumes/Backup"} {
		if err := g.Check(p); err == nil {
			t.Errorf("%s must be protected", p)
		}
	}
	if err := g.Check("/private/var/folders/xy/T/cache"); err != nil {
		t.Errorf("deep temp folders stay allowed: %v", err)
	}
}
