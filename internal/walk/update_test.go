package walk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/doguyilmaz/ebb/internal/tree"
)

// flat lists every node by path with what a scan reports for it.
func flat(t *tree.Tree) map[string]string {
	out := map[string]string{}
	var walk func(id tree.ID)
	walk = func(id tree.ID) {
		n := t.Node(id)
		out[t.Path(id)] = fmt.Sprintf("kind=%d alloc=%d unique=%d files=%d small=%d", n.Kind, n.Alloc, n.Unique, n.Files, n.SmallCount)
		for _, c := range t.Children(id) {
			walk(c)
		}
	}
	walk(t.Root())
	return out
}

func sameTree(t *testing.T, got, want *tree.Tree) {
	t.Helper()
	g, w := flat(got), flat(want)
	var diff []string
	for p, v := range w {
		if g[p] != v {
			diff = append(diff, fmt.Sprintf("%s: got %q want %q", p, g[p], v))
		}
	}
	for p := range g {
		if _, ok := w[p]; !ok {
			diff = append(diff, p+": should be gone")
		}
	}
	slices.Sort(diff)
	if len(diff) > 0 {
		t.Fatalf("updated tree differs from a fresh scan:\n%s", fmt.Sprint(diff))
	}
}

func update(t *testing.T, tr *tree.Tree, dirs, subtrees []string) {
	t.Helper()
	if _, err := Update(context.Background(), tr, dirs, subtrees, Options{}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateEqualsFullScan(t *testing.T) {
	root := fixture(t)
	before := scan(t, root, Options{}).Tree
	mkfile(t, filepath.Join(root, "a", "new.bin"), 5<<20)
	if err := os.Remove(filepath.Join(root, "a", "b", "s2")); err != nil {
		t.Fatal(err)
	}
	mkfile(t, filepath.Join(root, "a", "b", "s1"), 9000)
	mkfile(t, filepath.Join(root, "fresh", "deep", "x.bin"), 2<<20)
	mkfile(t, filepath.Join(root, "fresh", "y"), 100)
	if err := os.RemoveAll(filepath.Join(root, "été")); err != nil {
		t.Fatal(err)
	}
	// Events name the folders whose listing changed; "fresh/deep" is not in the old tree at all.
	update(t, before, []string{filepath.Join(root, "a"), filepath.Join(root, "a", "b"), root, filepath.Join(root, "fresh", "deep")}, nil)
	sameTree(t, before, scan(t, root, Options{}).Tree)
	if got := before.Find("with space", 10); len(got) != 0 {
		t.Fatal("search finds a removed file")
	}
}

func TestUpdateSubtreeRescansBelow(t *testing.T) {
	root := fixture(t)
	before := scan(t, root, Options{}).Tree
	mkfile(t, filepath.Join(root, "proj", "node_modules", "x", "lib", "big.js"), 3<<20)
	mkfile(t, filepath.Join(root, "proj", "node_modules", "x", "index.js"), 7000)
	update(t, before, nil, []string{filepath.Join(root, "proj")})
	sameTree(t, before, scan(t, root, Options{}).Tree)
}

func TestUpdateKeepsHardlinksCountedOnce(t *testing.T) {
	root := fixture(t)
	before := scan(t, root, Options{}).Tree
	mkfile(t, filepath.Join(root, "linked", "other"), 100)
	update(t, before, []string{filepath.Join(root, "linked")}, nil)
	sameTree(t, before, scan(t, root, Options{}).Tree)
}
