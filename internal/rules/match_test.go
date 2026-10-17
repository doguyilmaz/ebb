package rules

import (
	"slices"
	"testing"

	"github.com/doguyilmaz/ebb/internal/tree"
)

func matchTree() *tree.Tree {
	tr := tree.New("/h")
	tr.Write(func(w *tree.Writer) {
		lib := w.AddDir(tr.Root(), []byte("Library"), 1, 0)
		as := w.AddDir(lib, []byte("Application Support"), 1, 0)
		for _, app := range []string{"Code", "Slack"} {
			a := w.AddDir(as, []byte(app), 1, 0)
			w.AddDir(a, []byte("Cache"), 1, 0)
		}
		dl := w.AddDir(tr.Root(), []byte("Downloads"), 1, 0)
		sub := w.AddDir(dl, []byte("old"), 1, 0)
		w.AddFile(sub, []byte("x.dmg"), tree.KindFile, 5<<20, 5<<20, 5<<20, 1, 0)
		w.AddFile(dl, []byte("y.pkg"), tree.KindFile, 5<<20, 5<<20, 5<<20, 1, 0)
		w.AddFile(dl, []byte("z.txt"), tree.KindFile, 5<<20, 5<<20, 5<<20, 1, 0)
	})
	return tr
}

func names(tr *tree.Tree, ids []tree.ID) []string {
	var out []string
	for _, id := range ids {
		out = append(out, tr.Path(id))
	}
	slices.Sort(out)
	return out
}

func TestResolveGlobSegment(t *testing.T) {
	tr := matchTree()
	got := names(tr, Resolve(tr, "/h/Library/Application Support/*/Cache"))
	if !slices.Equal(got, []string{"/h/Library/Application Support/Code/Cache", "/h/Library/Application Support/Slack/Cache"}) {
		t.Fatal(got)
	}
}

func TestResolveDoubleStarAndBraces(t *testing.T) {
	tr := matchTree()
	got := names(tr, Resolve(tr, "/h/Downloads/**/*.{dmg,pkg}"))
	if !slices.Equal(got, []string{"/h/Downloads/old/x.dmg", "/h/Downloads/y.pkg"}) {
		t.Fatal(got)
	}
}

func TestResolveLiteralAndMissing(t *testing.T) {
	tr := matchTree()
	if got := Resolve(tr, "/h/Library"); len(got) != 1 {
		t.Fatal(got)
	}
	if got := Resolve(tr, "/h/Nope/*"); len(got) != 0 {
		t.Fatal(got)
	}
	if got := Resolve(tr, "/elsewhere/x"); len(got) != 0 {
		t.Fatal(got)
	}
}
