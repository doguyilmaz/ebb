package detect

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb"
	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/rules"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/tree"
)

// home builds a tree rooted at /h from "path=bytes" specs; intermediate
// directories are created as needed.
func home(t *testing.T, specs ...string) *tree.Tree {
	t.Helper()
	tr := tree.New("/h")
	dirs := map[string]tree.ID{"": tr.Root()}
	var dir func(w *tree.Writer, p string) tree.ID
	dir = func(w *tree.Writer, p string) tree.ID {
		if id, ok := dirs[p]; ok {
			return id
		}
		parent, name := "", p
		if i := strings.LastIndex(p, "/"); i >= 0 {
			parent, name = p[:i], p[i+1:]
		}
		id := w.AddDir(dir(w, parent), []byte(name), 1, 0)
		dirs[p] = id
		return id
	}
	tr.Write(func(w *tree.Writer) {
		for _, s := range specs {
			p, size, _ := strings.Cut(s, "=")
			var n uint64
			for _, c := range size {
				n = n*10 + uint64(c-'0')
			}
			i := strings.LastIndex(p, "/")
			w.AddFile(dir(w, p[:i]), []byte(p[i+1:]), tree.KindFile, n<<20, n<<20, n<<20, 1, 0)
		}
	})
	tr.Finish()
	return tr
}

func ctxFor(t *testing.T, tr *tree.Tree) *Context {
	t.Helper()
	env := &sys.Fake{HomeDir: "/h", GOOS: "darwin", Envs: map[string]string{sys.HomeEnv: "/h"}}
	set, err := rules.Load(env, ebb.Rules)
	if err != nil {
		t.Fatal(err)
	}
	return &Context{Tree: tr, Env: env, Rules: set, Now: time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC), StaleAfter: 30 * 24 * time.Hour, LargeFile: 1 << 30}
}

func byRule(fs []finding.Finding, id string) *finding.Finding {
	for i := range fs {
		if fs[i].RuleID == id {
			return &fs[i]
		}
	}
	return nil
}

func TestRulesDetectorClaimsOnce(t *testing.T) {
	tr := home(t, "Library/Caches/go-build/a=300", "Library/Caches/other/b=200")
	fs, err := rulesDetector{}.Detect(context.Background(), ctxFor(t, tr))
	if err != nil {
		t.Fatal(err)
	}
	gb := byRule(fs, "go.build")
	if gb == nil || gb.Bytes != 300<<20 || gb.Targets[0].Path != "/h/Library/Caches/go-build" {
		t.Fatalf("go.build %+v", gb)
	}
	gen := byRule(fs, "app.caches")
	if gen == nil || len(gen.Targets) != 1 || gen.Targets[0].Path != "/h/Library/Caches/other" {
		t.Fatalf("generic caches must skip the go-build folder: %+v", gen)
	}
	if gb.Actions[0].Kind != "command" || gb.Safety != finding.Regenerable || gb.Why == "" {
		t.Fatalf("rule fields not carried: %+v", gb)
	}
}

func TestRulesDetectorMinSize(t *testing.T) {
	tr := home(t, "Library/Caches/go-build/a=50")
	fs, _ := rulesDetector{}.Detect(context.Background(), ctxFor(t, tr))
	if byRule(fs, "go.build") != nil {
		t.Fatal("below min_size must not be reported")
	}
	if byRule(fs, "app.caches") != nil {
		t.Fatal("a folder claimed by a specific rule stays claimed even when too small to report")
	}
}

func TestRulesDetectorEvidence(t *testing.T) {
	tr := home(t, ".bun/install/cache/react@19/x=400", ".bun/install/cache/next@15/x=300")
	fs, _ := rulesDetector{}.Detect(context.Background(), ctxFor(t, tr))
	f := byRule(fs, "bun.cache")
	if f == nil || len(f.Evidence) == 0 || !strings.Contains(f.Evidence[0], "2 entries") {
		t.Fatalf("evidence %+v", f)
	}
}

type stub struct {
	pass int
	fn   func(c *Context) []finding.Finding
}

func (s stub) ID() string   { return "stub" }
func (s stub) Pass() int    { return s.pass }
func (s stub) Detect(_ context.Context, c *Context) ([]finding.Finding, error) { return s.fn(c), nil }

func TestRunSecondPassUsesCovered(t *testing.T) {
	c := ctxFor(t, home(t, "a/x=1"))
	var covered, outside bool
	first := stub{1, func(*Context) []finding.Finding {
		return []finding.Finding{{RuleID: "one", Bytes: 1, Targets: []finding.Target{{Path: "/h/a"}}}}
	}}
	second := stub{2, func(c *Context) []finding.Finding {
		covered, outside = c.Covered("/h/a/x"), c.Covered("/h/b")
		return []finding.Finding{{RuleID: "two", Bytes: 5}}
	}}
	fs, errs := RunWith(context.Background(), c, []Detector{second, first})
	if len(errs) != 0 || !covered || outside {
		t.Fatalf("covered=%v outside=%v errs=%v", covered, outside, errs)
	}
	if fs[0].RuleID != "two" {
		t.Fatal("findings must be sorted by bytes")
	}
}

func TestRulesDetectorMatchRule(t *testing.T) {
	tr := home(t, "code/app/node_modules/x/y=200", "code/notproj/node_modules/z=300")
	app, _ := tr.Lookup("/h/code/app")
	tr.Write(func(w *tree.Writer) { w.SetFlags(app, tree.FlagProject) })
	c := ctxFor(t, tr)
	c.Rules = &rules.Set{Rules: []rules.Rule{{ID: "nm", Title: "nm", Category: "build-artifact", Safety: finding.Regenerable,
		Why: "w", Impact: "i", MinSize: 1, MinTarget: 1, Match: &rules.Match{Name: "node_modules", Project: true, Under: []string{"~/code"}}}}}
	fs, _ := rulesDetector{}.Detect(context.Background(), c)
	if len(fs) != 1 || len(fs[0].Targets) != 1 || fs[0].Targets[0].Path != "/h/code/app/node_modules" {
		t.Fatalf("%+v", fs)
	}
}
