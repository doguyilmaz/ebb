package sources

import (
	"strings"
	"testing"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

func build(t *testing.T, root string, files map[string]uint64) *tree.Tree {
	t.Helper()
	tr := tree.New(root)
	dirs := map[string]tree.ID{"": tr.Root()}
	var dir func(w *tree.Writer, p string) tree.ID
	dir = func(w *tree.Writer, p string) tree.ID {
		if id, ok := dirs[p]; ok {
			return id
		}
		i := strings.LastIndex(p, "/")
		parent, name := "", p
		if i >= 0 {
			parent, name = p[:i], p[i+1:]
		}
		id := w.AddDir(dir(w, parent), []byte(name), 1, 0)
		dirs[p] = id
		return id
	}
	tr.Write(func(w *tree.Writer) {
		for p, mb := range files {
			i := strings.LastIndex(p, "/")
			w.AddFile(dir(w, p[:i]), []byte(p[i+1:]), tree.KindFile, mb<<20, mb<<20, mb<<20, 1, 0)
		}
	})
	tr.Finish()
	return tr
}

func TestFlowsSplitPlacesByOwnerAndKind(t *testing.T) {
	tr := build(t, "/d", map[string]uint64{
		"Users/me/Library/Caches/go-build/a": 300,
		"Users/me/Library/Other/b":           100,
		"Users/me/Documents/c":               200,
		"Applications/X.app/d":               400,
	})
	fs := []finding.Finding{{ID: "go.build", Title: "Go build cache", Owner: "Go", Category: "build-artifact", Safety: finding.Regenerable,
		Actions: []finding.Action{{Kind: "delete"}}, Targets: []finding.Target{{Path: "/d/Users/me/Library/Caches/go-build", Bytes: 300 << 20}}}}
	f := Build(tr, volume.Firmlinks{}, "/d/Users/me", fs)
	flow := map[string]uint64{}
	for _, l := range f.Links {
		flow[l.From+" > "+l.To] = l.Bytes >> 20
	}
	want := map[string]uint64{
		"p:/d/Users/me/Library > o:Go": 300, "o:Go > k:build-artifact": 300,
		"p:/d/Users/me/Library > o:Not claimed": 100,
		"p:/d/Users/me/Documents > o:You":       200, "o:You > k:yours": 200,
		"p:/d/Applications > o:Apps": 400, "o:Apps > k:apps": 400,
	}
	for k, v := range want {
		if flow[k] != v {
			t.Errorf("%s = %d, want %d (all: %v)", k, flow[k], v, flow)
		}
	}
	if f.Total>>20 != 1000 {
		t.Fatalf("total %d", f.Total>>20)
	}
	for _, n := range f.Nodes {
		if n.ID == "p:/d/Users/me/Library" && (n.Label != "~/Library" || n.Path != "/d/Users/me/Library") {
			t.Fatalf("%+v", n)
		}
		if n.ID == "p:/d/Users" {
			t.Fatal("home stands for Users")
		}
	}
}

func TestFlowsWhenTheScanIsTheHomeFolder(t *testing.T) {
	tr := build(t, "/h", map[string]uint64{"Documents/a": 10, "code/b": 20})
	f := Build(tr, volume.Firmlinks{}, "/h", nil)
	if f.Total>>20 != 30 {
		t.Fatalf("each folder once: total %d", f.Total>>20)
	}
}

func TestPlaceLabelsAndOwners(t *testing.T) {
	if got := label("/System/Volumes/Data/opt", "/System/Volumes/Data/Users/me", "/System/Volumes/Data"); got != "/opt" {
		t.Fatalf("top of the disk: %q", got)
	}
	if got := label("/Users/me/.bun", "/Users/me", "/System/Volumes/Data"); got != "~/.bun" {
		t.Fatalf("home: %q", got)
	}
	for f, want := range map[*finding.Finding]string{
		{Owner: "Bun", Category: "package-cache"}:          "Bun",
		{Title: "App caches", Category: "app-cache"}:       "Various apps",
		{Title: "Large files", Category: "large-file"}:     "You",
		{Title: "node_modules", Category: "stale-project"}: "Your projects",
	} {
		if got := ownerOf(*f); got != want {
			t.Errorf("%+v: %q, want %q", f, got, want)
		}
	}
}
