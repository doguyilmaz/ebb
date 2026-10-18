//go:build darwin

package walk

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/doguyilmaz/ebb/internal/tree"
)

func TestBulkMatchesGeneric(t *testing.T) {
	root := fixture(t)
	g := scan(t, root, generic)
	b := scan(t, root, Options{})
	gr, br := g.Tree.Node(g.Tree.Root()), b.Tree.Node(b.Tree.Root())
	if gr.Alloc != br.Alloc || gr.Files != br.Files || gr.Logical != br.Logical || g.Tree.Len() != b.Tree.Len() {
		t.Fatalf("generic %+v (%d nodes) vs bulk %+v (%d nodes)", gr, g.Tree.Len(), br, b.Tree.Len())
	}
	for _, p := range []string{"proj", "été/with space", "linked/orig", "a/b"} {
		gi, ok1 := g.Tree.Lookup(filepath.Join(root, p))
		bi, ok2 := b.Tree.Lookup(filepath.Join(root, p))
		if !ok1 || !ok2 {
			t.Fatalf("%s: generic %v bulk %v", p, ok1, ok2)
		}
		gn, bn := g.Tree.Node(gi), b.Tree.Node(bi)
		if gn.Alloc != bn.Alloc || gn.Flags != bn.Flags || gn.MaxMtime != bn.MaxMtime || gn.SmallCount != bn.SmallCount {
			t.Fatalf("%s: generic %+v vs bulk %+v", p, gn, bn)
		}
	}
}

func TestBulkPermissionAndOddNames(t *testing.T) {
	root := t.TempDir()
	names := []string{"new\nline", strings.Repeat("y", 255), "日本語"}
	for _, n := range names {
		mkfile(t, filepath.Join(root, n), 2<<20)
	}
	res := scan(t, root, Options{})
	for _, n := range names {
		if _, ok := res.Tree.Lookup(filepath.Join(root, n)); !ok {
			t.Fatalf("bulk lost %q", n)
		}
	}
}

func TestCloneUniqueBytes(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "orig"), 8<<20)
	if err := exec.Command("cp", "-c", filepath.Join(root, "orig"), filepath.Join(root, "clone")).Run(); err != nil {
		t.Skip("cp -c unsupported here")
	}
	res := scan(t, root, Options{})
	n := res.Tree.Node(res.Tree.Root())
	if n.Alloc < 16<<20 {
		t.Fatalf("allocated should count both copies: %d", n.Alloc)
	}
	if n.Unique >= 16<<20 {
		t.Fatalf("unique must drop shared blocks: %d", n.Unique)
	}
	id, _ := res.Tree.Lookup(filepath.Join(root, "clone"))
	if res.Tree.Node(id).Flags&tree.FlagClone == 0 {
		t.Fatalf("clone flag missing: %+v", res.Tree.Node(id))
	}
}

func TestDuMatches(t *testing.T) {
	root := runtime.GOROOT()
	out, err := exec.Command("du", "-skx", root).Output()
	if err != nil {
		t.Skip("du unavailable")
	}
	duKB, _ := strconv.ParseUint(strings.Fields(string(out))[0], 10, 64)
	res := scan(t, root, Options{})
	gotKB := res.Tree.Node(res.Tree.Root()).Alloc / 1024
	diff := float64(int64(gotKB)-int64(duKB)) / float64(duKB)
	if diff > 0.01 || diff < -0.01 {
		t.Fatalf("ebb %d KB vs du %d KB (%.2f%%)", gotKB, duKB, diff*100)
	}
}

func TestBulkPermissionDenied(t *testing.T) {
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	mkfile(t, filepath.Join(locked, "x"), 2<<20)
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	res := scan(t, root, Options{})
	id, ok := res.Tree.Lookup(locked)
	if !ok || res.Tree.Node(id).Flags&tree.FlagDenied == 0 || len(res.Problems) != 1 || res.Problems[0].Path != locked {
		t.Fatalf("bulk reader: %v %+v", ok, res.Problems)
	}
}

func TestOwnTrashOnAnUnlistableTrashes(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(root, ".Trashes", strconv.Itoa(os.Getuid()))
	if err := os.MkdirAll(mine, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mine, "old.mov"), make([]byte, 3<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	// As on a real disk: anyone may enter .Trashes, nobody may list it.
	trashes := filepath.Join(root, ".Trashes")
	if err := os.Chmod(trashes, 0o1333); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(trashes, 0o755) })
	res := scan(t, root, Options{})
	id, ok := res.Tree.Lookup(mine)
	if !ok || res.Tree.Node(id).Files != 1 || res.Tree.Node(id).Alloc < 3<<20 {
		t.Fatalf("own trash not read: found %v", ok)
	}
	for _, p := range res.Problems {
		if strings.Contains(p.Path, ".Trashes") {
			t.Fatalf("reported as unreadable: %+v", p)
		}
	}
}
