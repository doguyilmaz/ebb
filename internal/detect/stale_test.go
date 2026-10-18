package detect

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

const (
	old    = "1735689600" // 2025-01-01
	recent = "1792000000" // 2026-10-14
)

func staleTree(t *testing.T) *tree.Tree {
	tr := home(t,
		"code/a/package.json=0@"+old, "code/a/src/index.js=0@"+old, "code/a/node_modules/react/x=300@"+recent,
		"code/b/package.json=0@"+old, "code/b/src/x=0@"+recent, "code/b/node_modules/y=200@"+old,
		"code/c/package.json=0@"+old, "code/c/node_modules/z/x=150@"+old, "code/c/node_modules/z/node_modules/q/x=50@"+old,
		"code/rust/Cargo.toml=0@"+old, "code/rust/target/debug/x=400@"+old)
	for _, p := range []string{"/h/code/a", "/h/code/b", "/h/code/c"} {
		markers(tr, p, "package.json")
	}
	markers(tr, "/h/code/rust", "Cargo.toml")
	return tr
}

func staleCtx(t *testing.T) *Context {
	c := ctxFor(t, staleTree(t))
	c.Covered = func(string) bool { return false }
	return c
}

func TestStaleNodeModulesGrouped(t *testing.T) {
	fs, _ := stale{}.Detect(context.Background(), staleCtx(t))
	nm := byRule(fs, "stale.node_modules")
	if nm == nil || !slices.Equal(paths(nm), []string{"/h/code/a/node_modules", "/h/code/c/node_modules"}) {
		t.Fatalf("node_modules %+v", nm)
	}
	if nm.Bytes != 500<<20 || nm.Safety != finding.Regenerable || !strings.Contains(nm.Title, "2 projects") {
		t.Fatalf("%+v", nm)
	}
	if target := byRule(fs, "stale.target"); target == nil || target.Targets[0].Path != "/h/code/rust/target" {
		t.Fatalf("target %+v", target)
	}
}

func TestStaleIgnoresActiveProject(t *testing.T) {
	fs, _ := stale{}.Detect(context.Background(), staleCtx(t))
	for _, f := range fs {
		for _, p := range paths(&f) {
			if strings.HasPrefix(p, "/h/code/b/") {
				t.Fatalf("active project offered: %s", p)
			}
		}
	}
}

func TestStaleNoNestedArtifacts(t *testing.T) {
	fs, _ := stale{}.Detect(context.Background(), staleCtx(t))
	for _, f := range fs {
		for _, p := range paths(&f) {
			if strings.Count(p, "node_modules") > 1 {
				t.Fatalf("nested artifact listed: %s", p)
			}
		}
	}
}

func TestLargeFilesSkipCovered(t *testing.T) {
	tr := home(t, "big.iso=2000", "Library/Caches/x/huge.bin=1500", "Applications/X.app/Contents/blob=1200", "small.bin=900")
	c := ctxFor(t, tr)
	c.Covered = func(p string) bool { return strings.HasPrefix(p, "/h/Library/Caches/x") }
	fs, _ := large{}.Detect(context.Background(), c)
	if len(fs) != 1 || !slices.Equal(paths(&fs[0]), []string{"/h/big.iso"}) || fs[0].Safety != finding.Caution {
		t.Fatalf("%+v", fs)
	}
}

func TestDockerEvidence(t *testing.T) {
	path := "/h/Library/Containers/com.docker.docker/Data/vms/0/data"
	tr := home(t, "Library/Containers/com.docker.docker/Data/vms/0/data/keep=0")
	dir, _ := tr.Lookup(path)
	tr.Write(func(w *tree.Writer) {
		w.AddFile(dir, []byte("Docker.raw"), tree.KindFile, 20<<30, 20<<30, 64<<30, 1, 0)
	})
	fs, _ := docker{}.Detect(context.Background(), ctxFor(t, tr))
	if len(fs) != 1 || fs[0].Actions[0].Kind != "manual" || !strings.Contains(strings.Join(fs[0].Evidence, " "), "68.7 GB") {
		t.Fatalf("%+v", fs)
	}
}

func TestSnapshotsInformational(t *testing.T) {
	c := ctxFor(t, home(t))
	snap := volume.Snapshot{Name: "com.apple.TimeMachine.2026-10-10-101500.local", Date: time.Date(2026, 10, 10, 10, 15, 0, 0, time.UTC)}
	c.Volume = volume.Breakdown{Snapshots: []volume.Snapshot{snap}}
	fs, _ := snapshots{}.Detect(context.Background(), c)
	if len(fs) != 1 || fs[0].Safety != finding.System || fs[0].Bytes != 0 || fs[0].Actions[0].Kind != "manual" || fs[0].Actions[0].Hint == "" {
		t.Fatalf("%+v", fs)
	}
	c.Volume = volume.Breakdown{}
	if none, _ := (snapshots{}).Detect(context.Background(), c); len(none) != 0 {
		t.Fatal("no snapshots, no finding")
	}
}

func markers(tr *tree.Tree, path string, names ...string) {
	id, _ := tr.Lookup(path)
	var m uint16
	for _, n := range names {
		m |= walkMarker(n)
	}
	tr.Write(func(w *tree.Writer) { w.SetMarkers(id, m) })
}

func TestStaleRequiresMatchingMarker(t *testing.T) {
	tr := home(t, "code/electron/package.json=0@"+old, "code/electron/build/icon.icns=300@"+old,
		"code/android/build.gradle=0@"+old, "code/android/build/out=300@"+old)
	markers(tr, "/h/code/electron", "package.json")
	markers(tr, "/h/code/android", "build.gradle")
	c := ctxFor(t, tr)
	c.Covered = func(string) bool { return false }
	fs, _ := stale{}.Detect(context.Background(), c)
	b := byRule(fs, "stale.build")
	if b == nil || !slices.Equal(paths(b), []string{"/h/code/android/build"}) {
		t.Fatalf("build/ next to only package.json may hold sources: %+v", b)
	}
}

func TestStaleSkipsClaimedFolders(t *testing.T) {
	tr := home(t, ".bun/install/cache/pkg@1/package.json=0@"+old, ".bun/install/cache/pkg@1/node_modules/x=300@"+old)
	markers(tr, "/h/.bun/install/cache/pkg@1", "package.json")
	c := ctxFor(t, tr)
	fs, _ := RunWith(context.Background(), c, []Detector{rulesDetector{}, stale{}})
	if byRule(fs, "stale.node_modules") != nil {
		t.Fatal("folders inside a cache another finding claims must not be reported again")
	}
}

func TestLargeFilesOnlyInPersonalFolders(t *testing.T) {
	tr := home(t, "Movies/film.mov=2000", "Library/Unity/pkg.unitypackage=3000", ".android/avd/P.avd/userdata.img=4000",
		"Developer/app/build/Run.xcresult/Staging/log=4400", "Developer/app/build/Run.xcresult/Info.plist=0")
	c := ctxFor(t, tr)
	c.Covered = func(string) bool { return false }
	fs, _ := large{}.Detect(context.Background(), c)
	if len(fs) != 1 || !slices.Equal(paths(&fs[0]), []string{"/h/Developer/app/build/Run.xcresult", "/h/Movies/film.mov"}) {
		t.Fatalf("%+v", fs)
	}
}

func TestRunDropsTinyFindings(t *testing.T) {
	c := ctxFor(t, home(t, "a/x=1"))
	tiny := stub{1, func(*Context) []finding.Finding {
		return []finding.Finding{{RuleID: "tiny", Alloc: 10, Targets: []finding.Target{{Path: "/h/a", Alloc: 10}}}, {RuleID: "info"}}
	}}
	fs, _ := RunWith(context.Background(), c, []Detector{tiny})
	if len(fs) != 1 || fs[0].RuleID != "info" {
		t.Fatalf("%+v", fs)
	}
}

func TestStaleOnlyInPersonalProjects(t *testing.T) {
	tr := treeAt(t, "/",
		"Applications/IDE.app/Contents/Resources/app/package.json=0@"+old, "Applications/IDE.app/Contents/Resources/app/node_modules/x=200@"+old,
		"Users/h/.vscode/extensions/ext/package.json=0@"+old, "Users/h/.vscode/extensions/ext/node_modules/x=200@"+old,
		"Users/h/Library/Application Support/Zed/ls/package.json=0@"+old, "Users/h/Library/Application Support/Zed/ls/node_modules/x=200@"+old,
		"Users/h/Tools/App.app/Contents/package.json=0@"+old, "Users/h/Tools/App.app/Contents/node_modules/x=200@"+old,
		"Users/h/code/mine/package.json=0@"+old, "Users/h/code/mine/node_modules/x=200@"+old)
	for _, p := range []string{"/Applications/IDE.app/Contents/Resources/app", "/Users/h/.vscode/extensions/ext",
		"/Users/h/Library/Application Support/Zed/ls", "/Users/h/Tools/App.app/Contents", "/Users/h/code/mine"} {
		markers(tr, p, "package.json")
	}
	c := ctxFor(t, tr)
	c.Env.(*sys.Fake).HomeDir = "/Users/h"
	c.Covered = func(string) bool { return false }
	fs, _ := stale{}.Detect(context.Background(), c)
	nm := byRule(fs, "stale.node_modules")
	if nm == nil || !slices.Equal(paths(nm), []string{"/Users/h/code/mine/node_modules"}) {
		t.Fatalf("apps, editor extensions and tool folders must never be offered: %+v", nm)
	}
}

func TestRunNoDoubleCounting(t *testing.T) {
	c := ctxFor(t, home(t, "a/b/x=10", "c/y=10"))
	outer := stub{1, func(*Context) []finding.Finding {
		return []finding.Finding{{RuleID: "outer", Alloc: 10 << 20, Bytes: 10 << 20, Targets: []finding.Target{{Path: "/h/a", Bytes: 10 << 20, Alloc: 10 << 20}}}}
	}}
	inner := stub{1, func(*Context) []finding.Finding {
		return []finding.Finding{{RuleID: "inner", Targets: []finding.Target{{Path: "/h/a/b", Bytes: 10 << 20, Alloc: 10 << 20}, {Path: "/h/c", Bytes: 10 << 20, Alloc: 10 << 20}}}}
	}}
	later := stub{2, func(*Context) []finding.Finding {
		return []finding.Finding{{RuleID: "later", Targets: []finding.Target{{Path: "/h/c/y", Bytes: 10 << 20, Alloc: 10 << 20}}}}
	}}
	grow := stub{2, func(*Context) []finding.Finding {
		return []finding.Finding{{RuleID: "growth", Category: "growth", Targets: []finding.Target{{Path: "/h/a", Bytes: 5 << 20, Alloc: 10 << 20}}}}
	}}
	fs, _ := RunWith(context.Background(), c, []Detector{outer, inner, later, grow})
	in := byRule(fs, "inner")
	if in == nil || len(in.Targets) != 1 || in.Targets[0].Path != "/h/c" || in.Bytes != 10<<20 {
		t.Fatalf("a location already claimed must not count twice: %+v", in)
	}
	if byRule(fs, "later") != nil {
		t.Fatal("pass-2 findings inside claimed locations must go")
	}
	if byRule(fs, "growth") == nil {
		t.Fatal("growth is informational and may point anywhere")
	}
}

func TestLargeFilesSkipLibraries(t *testing.T) {
	tr := home(t, "Pictures/Old/Family.photoslibrary/originals/a.heic=2000", "Movies/Trip.fcpbundle/media/clip.mov=3000",
		"Music/Band.logicx/audio/take.wav=2500", "Movies/raw.mov=2200")
	c := ctxFor(t, tr)
	c.Covered = func(string) bool { return false }
	fs, _ := large{}.Detect(context.Background(), c)
	if len(fs) != 1 || !slices.Equal(paths(&fs[0]), []string{"/h/Movies/raw.mov"}) {
		t.Fatalf("libraries are the person's own data: %+v", fs)
	}
}

func TestFindingIDsStable(t *testing.T) {
	a := finish(finding.Finding{RuleID: "x", Targets: []finding.Target{{Path: "/h/a", Alloc: 2}, {Path: "/h/b", Alloc: 1}}})
	b := finish(finding.Finding{RuleID: "x", Targets: []finding.Target{{Path: "/h/a", Alloc: 1}, {Path: "/h/b", Alloc: 2}}})
	if a.ID != b.ID || a.ID == "" {
		t.Fatalf("IDs must not depend on target order or sizes: %q %q", a.ID, b.ID)
	}
}

func TestStaleDistOnlyWhenGitIgnoresIt(t *testing.T) {
	tr := home(t,
		"code/app/package.json=0@"+old, "code/app/dist/bundle.js=120@"+old,
		"code/lib/package.json=0@"+old, "code/lib/.git/HEAD=0@"+old, "code/lib/dist/index.js=110@"+old,
		"code/mono/.git/HEAD=0@"+old, "code/mono/packages/ui/package.json=0@"+old, "code/mono/packages/ui/dist/ui.js=100@"+old,
		"code/root/.git/HEAD=0@"+old, "code/root/packages/x/package.json=0@"+old, "code/root/packages/x/dist/x.js=100@"+old,
		"code/web/.git/HEAD=0@"+old, "code/web/package.json=0@"+old, "code/web/dist/w.js=100@"+old,
		"code/swift/Package.swift=0@"+old, "code/swift/.build/debug/x=200@"+old)
	for _, p := range []string{"/h/code/app", "/h/code/lib", "/h/code/mono/packages/ui", "/h/code/root/packages/x", "/h/code/web"} {
		markers(tr, p, "package.json")
	}
	markers(tr, "/h/code/swift", "Package.swift")
	c := ctxFor(t, tr)
	c.Covered = func(string) bool { return false }
	c.Env.(*sys.Fake).RunFn = func(string, string, ...string) ([]byte, error) {
		t.Fatal("Git never runs in someone else's repository")
		return nil, nil
	}
	ignores := map[string]string{
		"/h/code/mono/.gitignore": "# build\nnode_modules\ndist/\n",
		"/h/code/root/.gitignore": "/dist/\n",
		"/h/code/web/.gitignore":  "/dist\n",
	}
	c.ReadFile = func(p string) ([]byte, error) {
		if b, ok := ignores[p]; ok {
			return []byte(b), nil
		}
		return nil, os.ErrNotExist
	}
	fs, err := stale{}.Detect(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if dist := byRule(fs, "stale.dist"); dist != nil {
		for _, tg := range dist.Targets {
			got = append(got, strings.TrimPrefix(tg.Path, "/h/code/"))
		}
	}
	slices.Sort(got)
	// Only inside a repository, and only when a .gitignore lists it: an
	// anchored /dist covers the folder next to that .gitignore, not nested ones.
	if want := []string{"mono/packages/ui/dist", "web/dist"}; !slices.Equal(got, want) {
		t.Fatalf("dist targets %q, want %q", got, want)
	}
	if byRule(fs, "stale.dot-build") == nil {
		t.Fatal("Swift packages' .build is build output, with an ID of its own")
	}
}
