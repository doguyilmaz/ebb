package detect

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

const (
	old    = "1735689600" // 2025-01-01
	recent = "1792000000" // 2026-10-14
)

func flagProjects(tr *tree.Tree, paths ...string) {
	var ids []tree.ID
	for _, p := range paths {
		id, _ := tr.Lookup(p)
		ids = append(ids, id)
	}
	tr.Write(func(w *tree.Writer) {
		for _, id := range ids {
			w.SetFlags(id, tree.FlagProject)
		}
	})
}

func staleTree(t *testing.T) *tree.Tree {
	tr := home(t,
		"code/a/package.json=0@"+old, "code/a/src/index.js=0@"+old, "code/a/node_modules/react/x=300@"+recent,
		"code/b/package.json=0@"+old, "code/b/src/x=0@"+recent, "code/b/node_modules/y=200@"+old,
		"code/c/package.json=0@"+old, "code/c/node_modules/z/x=150@"+old, "code/c/node_modules/z/node_modules/q/x=50@"+old,
		"code/rust/Cargo.toml=0@"+old, "code/rust/target/debug/x=400@"+old)
	flagProjects(tr, "/h/code/a", "/h/code/b", "/h/code/c", "/h/code/rust")
	return tr
}

func TestStaleNodeModulesGrouped(t *testing.T) {
	fs, _ := stale{}.Detect(context.Background(), ctxFor(t, staleTree(t)))
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
	fs, _ := stale{}.Detect(context.Background(), ctxFor(t, staleTree(t)))
	for _, f := range fs {
		for _, p := range paths(&f) {
			if strings.HasPrefix(p, "/h/code/b/") {
				t.Fatalf("active project offered: %s", p)
			}
		}
	}
}

func TestStaleNoNestedArtifacts(t *testing.T) {
	fs, _ := stale{}.Detect(context.Background(), ctxFor(t, staleTree(t)))
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
