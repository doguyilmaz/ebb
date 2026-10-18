package detect

import (
	"context"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb"
	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/rules"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/tree"
)

// home builds a tree rooted at /h from "path=MB" or "path=MB@mtime" specs;
// intermediate directories are created as needed.
func home(t *testing.T, specs ...string) *tree.Tree { return treeAt(t, "/h", specs...) }

func treeAt(t *testing.T, root string, specs ...string) *tree.Tree {
	t.Helper()
	tr := tree.New(root)
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
			p, rest, _ := strings.Cut(s, "=")
			size, mt, _ := strings.Cut(rest, "@")
			n, _ := strconv.ParseUint(size, 10, 64)
			mtime := int64(1)
			if mt != "" {
				mtime, _ = strconv.ParseInt(mt, 10, 64)
			}
			parent, name := "", p
			if i := strings.LastIndex(p, "/"); i >= 0 {
				parent, name = p[:i], p[i+1:]
			}
			w.AddFile(dir(w, parent), []byte(name), tree.KindFile, n<<20, n<<20, n<<20, mtime, 0)
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

func (s stub) ID() string                                                      { return "stub" }
func (s stub) Pass() int                                                       { return s.pass }
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

func walkMarker(name string) uint16 {
	if name == "xcode" {
		return tree.MarkerXcode
	}
	return tree.MarkerBit(name)
}

func TestCoverageRules(t *testing.T) {
	tr := home(t,
		"Library/Caches/Google/Chrome/Default/Cache/data=150",
		"Library/Application Support/Google/Chrome/Profile 1/Code Cache/js=120",
		"Library/Application Support/Google/Chrome/Profile 1/History=40",
		"Library/Caches/com.apple.Safari/WebKitCache/x=200",
		"Library/Caches/Firefox/Profiles/ab.default/cache2/entries=130",
		"Library/Caches/com.apple.dt.Xcode/data=300",
		"Library/iTunes/iPhone Software Updates/iPhone.ipsw=400",
		"Library/Containers/com.apple.mail/Data/Library/Mail Downloads/a.pdf=60",
		".m2/repository/org/x.jar=200",
		".pub-cache/hosted/pkg=150",
		"miniforge3/pkgs/numpy=300",
		".cache/uv/wheels=200",
		".cache/huggingface/hub/model=500",
		".ollama/models/blobs/b=900",
		"Library/Caches/ms-playwright/chromium-1100/x=300",
		"Library/Caches/JetBrains/IntelliJIdea2025.2/index=400",
		"Desktop/Installer.pkg=80",
		"Documents/Old.pkg=90",
		"Documents/Vault.dmg=90",
		"Documents/Projects/keep.dmg=90",
	)
	fs, err := rulesDetector{}.Detect(context.Background(), ctxFor(t, tr))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"browser.chrome": 2, "browser.safari": 1, "browser.firefox": 1, "xcode.cache": 1, "ios.updates": 1, "mail.downloads": 1,
		"maven.repo": 1, "pub.cache": 1, "conda.pkgs": 1, "uv.cache": 1, "huggingface.cache": 1, "models.local": 1, "browsers.testing": 1,
		"jetbrains.caches": 1, "installers": 2}
	for id, n := range want {
		f := byRule(fs, id)
		if f == nil || len(f.Targets) != n {
			t.Errorf("%s: %+v", id, f)
		}
	}
	for _, f := range fs {
		for _, tg := range f.Targets {
			if strings.HasSuffix(tg.Path, "/History") || strings.HasSuffix(tg.Path, ".dmg") && !strings.Contains(tg.Path, "/Downloads/") {
				t.Errorf("%s claims %s", f.RuleID, tg.Path)
			}
		}
	}
	if f := byRule(fs, "apple.caches"); f != nil {
		t.Errorf("Safari's and Xcode's caches go to their own rules, not the macOS one: %+v", f.Targets)
	}
	if f := byRule(fs, "models.local"); f != nil && len(f.Actions) != 0 {
		t.Error("downloaded models are the person's: no action")
	}
}

func TestOldIDEVersions(t *testing.T) {
	tr := home(t,
		"Library/Application Support/JetBrains/IntelliJIdea2024.3/options/a=5",
		"Library/Application Support/JetBrains/IntelliJIdea2025.2/options/a=5",
		"Library/Caches/JetBrains/IntelliJIdea2024.3/index=600",
		"Library/Caches/JetBrains/IntelliJIdea2025.2/index=700",
		"Library/Logs/JetBrains/IntelliJIdea2024.3/idea.log=20",
		"Library/Caches/JetBrains/PyCharm2023.1/index=300",
		"Library/Caches/JetBrains/PyCharm2024.1/index=300",
		"Library/Application Support/Google/AndroidStudio2024.1/options/a=5",
		"Library/Application Support/Google/AndroidStudio2025.1/options/a=5",
		"Library/Caches/Google/AndroidStudio2024.1/caches=900",
		"Library/Application Support/Google/Chrome/Default/x=10",
		// The release still in use beside a newer EAP.
		"Library/Application Support/JetBrains/GoLand2025.1/options/a=5@1791590400",
		"Library/Application Support/JetBrains/GoLand2025.2/options/a=5",
	)
	fs, err := RunWith(context.Background(), ctxFor(t, tr), []Detector{ideVersions{}, rulesDetector{}})
	if err != nil {
		t.Fatal(err)
	}
	f := byRule(fs, "ide.old-versions")
	if f == nil {
		t.Fatal("no finding")
	}
	var got []string
	for _, tg := range f.Targets {
		got = append(got, strings.TrimPrefix(tg.Path, "/h/Library/"))
	}
	slices.Sort(got)
	want := []string{"Application Support/Google/AndroidStudio2024.1", "Application Support/JetBrains/IntelliJIdea2024.3",
		"Caches/Google/AndroidStudio2024.1", "Caches/JetBrains/IntelliJIdea2024.3", "Logs/JetBrains/IntelliJIdea2024.3"}
	if !slices.Equal(got, want) {
		t.Fatalf("old versions\n got %q\nwant %q", got, want)
	}
	if jb := byRule(fs, "jetbrains.caches"); jb == nil || slices.ContainsFunc(jb.Targets, func(t finding.Target) bool { return strings.Contains(t.Path, "2024.3") }) {
		t.Fatalf("the current version's caches stay with the cache rule: %+v", jb)
	}
}

func TestTrashOnAnotherDisk(t *testing.T) {
	uid := strconv.Itoa(os.Getuid())
	tr := treeAt(t, "/Volumes/Backup", ".Trashes/"+uid+"/old.mov=300", ".Trashes/0/other=300", "Photos/a.jpg=50")
	fs, err := volumeTrash{}.Detect(context.Background(), ctxFor(t, tr))
	if err != nil || len(fs) != 1 || fs[0].Targets[0].Path != "/Volumes/Backup/.Trashes/"+uid || fs[0].Actions[0].Kind != "empty-trash" {
		t.Fatalf("%+v %v", fs, err)
	}
	if fs, _ := (volumeTrash{}).Detect(context.Background(), ctxFor(t, home(t, ".Trashes/"+uid+"/x=300"))); len(fs) != 0 {
		t.Fatal("only a disk's own root has its Trash there")
	}
}
