package tree

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"testing"
)

func build(t *testing.T) (*Tree, ID, ID) {
	t.Helper()
	tr := New("/r")
	var a, b ID
	tr.Write(func(w *Writer) {
		a = w.AddDir(tr.Root(), []byte("a"), 100, 0)
		b = w.AddDir(a, []byte("b"), 200, 0)
		w.AddFile(b, []byte("big.bin"), KindFile, 5<<20, 5<<20, 5<<20, 300, 0)
		w.AddSmall(a, 3, 3000, 3000, 2900, 400)
	})
	tr.Finish()
	return tr, a, b
}

func TestPropagation(t *testing.T) {
	tr, a, b := build(t)
	if got := tr.Node(tr.Root()).Alloc; got != 5<<20+3000 {
		t.Fatalf("root alloc %d", got)
	}
	if n := tr.Node(a); n.Files != 4 || n.MaxMtime != 400 || n.SmallCount != 3 || n.OwnMtime != 400 {
		t.Fatalf("a %+v", n)
	}
	if n := tr.Node(b); n.Alloc != 5<<20 || n.OwnMtime != 300 || n.Files != 1 {
		t.Fatalf("b %+v", n)
	}
	if tr.Path(b) != "/r/a/b" {
		t.Fatal(tr.Path(b))
	}
	if id, ok := tr.Lookup("/r/a/b/big.bin"); !ok || tr.Node(id).Kind != KindFile {
		t.Fatal("lookup file")
	}
	if id, ok := tr.Lookup("/r"); !ok || id != tr.Root() {
		t.Fatal("lookup root")
	}
	if _, ok := tr.Lookup("/r/missing"); ok {
		t.Fatal("lookup missing")
	}
	if !tr.IsAncestor(a, b) || tr.IsAncestor(b, a) || !tr.IsAncestor(a, a) {
		t.Fatal("ancestry")
	}
}

func TestChildrenSortedBySize(t *testing.T) {
	tr := New("/r")
	tr.Write(func(w *Writer) {
		w.AddFile(tr.Root(), []byte("s"), KindFile, 2<<20, 2<<20, 2<<20, 1, 0)
		w.AddFile(tr.Root(), []byte("l"), KindFile, 9<<20, 9<<20, 9<<20, 1, 0)
		w.AddFile(tr.Root(), []byte("m"), KindFile, 5<<20, 5<<20, 5<<20, 1, 0)
	})
	tr.Finish()
	c := tr.Children(tr.Root())
	if len(c) != 3 || tr.Node(c[0]).Name != "l" || tr.Node(c[1]).Name != "m" || tr.Node(c[2]).Name != "s" {
		t.Fatal("not sorted by size")
	}
}

func TestHardlinkCountedOnce(t *testing.T) {
	tr := New("/r")
	tr.Write(func(w *Writer) {
		x := w.AddDir(tr.Root(), []byte("x"), 1, 0)
		y := w.AddDir(tr.Root(), []byte("y"), 1, 0)
		if !w.Link(42, 2, 4<<20, x) {
			t.Fatal("first sighting must count")
		}
		w.AddFile(x, []byte("f"), KindFile, 4<<20, 4<<20, 4<<20, 1, 0)
		if w.Link(42, 2, 4<<20, y) {
			t.Fatal("second sighting must not count")
		}
		w.AddFile(y, []byte("f"), KindFile, 0, 0, 4<<20, 1, FlagShared)
	})
	if got := tr.Node(tr.Root()).Alloc; got != 4<<20 {
		t.Fatalf("double counted: %d", got)
	}
}

func TestReclaimableHardlinkOutside(t *testing.T) {
	tr := New("/r")
	var x ID
	tr.Write(func(w *Writer) {
		x = w.AddDir(tr.Root(), []byte("x"), 1, 0)
		y := w.AddDir(tr.Root(), []byte("y"), 1, 0)
		w.Link(42, 2, 4<<20, x)
		w.AddFile(x, []byte("f"), KindFile, 4<<20, 4<<20, 4<<20, 1, 0)
		w.Link(42, 2, 4<<20, y)
		w.AddFile(y, []byte("f"), KindFile, 0, 0, 4<<20, 1, FlagShared)
	})
	if got := tr.Reclaimable(x); got != 0 {
		t.Fatalf("x reclaimable %d, want 0: y still links the inode", got)
	}
	if got := tr.Reclaimable(tr.Root()); got != 4<<20 {
		t.Fatalf("root reclaimable %d", got)
	}
}

func TestReclaimableLinkNotAllSeen(t *testing.T) {
	tr := New("/r")
	var x ID
	tr.Write(func(w *Writer) {
		x = w.AddDir(tr.Root(), []byte("x"), 1, 0)
		w.Link(7, 3, 2<<20, x) // two more links live outside the scanned tree
		w.AddFile(x, []byte("f"), KindFile, 2<<20, 2<<20, 2<<20, 1, 0)
	})
	if got := tr.Reclaimable(tr.Root()); got != 0 {
		t.Fatalf("links outside the scan keep the data alive: got %d", got)
	}
}

func TestSetFlags(t *testing.T) {
	tr := New("/r")
	var d ID
	tr.Write(func(w *Writer) {
		d = w.AddDir(tr.Root(), []byte("d"), 1, 0)
		w.SetFlags(d, FlagDenied)
	})
	if tr.Node(d).Flags&FlagDenied == 0 || tr.Len() != 2 {
		t.Fatal("flags")
	}
}

func TestCloneGroupUniqueOnce(t *testing.T) {
	tr := New("/r")
	var a, b ID
	tr.Write(func(w *Writer) {
		a = w.AddDir(tr.Root(), []byte("a"), 1, 0)
		b = w.AddDir(tr.Root(), []byte("b"), 1, 0)
		if !w.Clone(77, 2, 8<<20, a) {
			t.Fatal("first clone must count")
		}
		w.AddFile(a, []byte("orig"), KindFile, 8<<20, 8<<20, 8<<20, 1, FlagClone)
		if w.Clone(77, 2, 8<<20, b) {
			t.Fatal("second clone must not count")
		}
		w.AddFile(b, []byte("copy"), KindFile, 8<<20, 0, 8<<20, 1, FlagClone)
	})
	root := tr.Node(tr.Root())
	if root.Alloc != 16<<20 || root.Unique != 8<<20 {
		t.Fatalf("alloc counts both copies like du, unique only one: %+v", root)
	}
	if got := tr.Reclaimable(a); got != 0 {
		t.Fatalf("deleting one clone frees nothing while its partner lives: %d", got)
	}
	if got := tr.Reclaimable(tr.Root()); got != 8<<20 {
		t.Fatalf("both clones inside: %d", got)
	}
}

func TestReclaimableNestedLinks(t *testing.T) {
	tr := New("/r")
	var a, b, c ID
	tr.Write(func(w *Writer) {
		a = w.AddDir(tr.Root(), []byte("a"), 1, 0)
		b = w.AddDir(a, []byte("b"), 1, 0)
		c = w.AddDir(a, []byte("c"), 1, 0)
		w.Link(5, 2, 1<<20, b)
		w.AddFile(b, []byte("f"), KindFile, 1<<20, 1<<20, 1<<20, 1, 0)
		w.Link(5, 2, 1<<20, c)
		w.AddFile(c, []byte("f"), KindFile, 0, 0, 1<<20, 1, FlagShared)
	})
	tr.Finish()
	if tr.Reclaimable(b) != 0 || tr.Reclaimable(c) != 0 || tr.Reclaimable(a) != 1<<20 || tr.Reclaimable(tr.Root()) != 1<<20 {
		t.Fatalf("b=%d c=%d a=%d", tr.Reclaimable(b), tr.Reclaimable(c), tr.Reclaimable(a))
	}
	tr.Write(func(w *Writer) {
		d := w.AddDir(tr.Root(), []byte("d"), 1, 0)
		w.Link(6, 2, 1<<20, d)
		w.AddFile(d, []byte("g"), KindFile, 1<<20, 1<<20, 1<<20, 1, 0)
	})
	if tr.Reclaimable(tr.Root()) != 1<<20 {
		t.Fatalf("a write must invalidate the cached figures: %d", tr.Reclaimable(tr.Root()))
	}
}

func TestReclaimableFileTarget(t *testing.T) {
	tr := New("/r")
	var f ID
	tr.Write(func(w *Writer) {
		d := w.AddDir(tr.Root(), []byte("d"), 1, 0)
		e := w.AddDir(tr.Root(), []byte("e"), 1, 0)
		f = w.AddFile(d, []byte("vm.img"), KindFile, 4<<20, 4<<20, 4<<20, 1, 0)
		w.Link(9, 2, 4<<20, f)
		g := w.AddFile(e, []byte("vm.img"), KindFile, 0, 0, 4<<20, 1, FlagShared)
		w.Link(9, 2, 4<<20, g)
	})
	if got := tr.Reclaimable(f); got != 0 {
		t.Fatalf("deleting one hardlink of a file frees nothing: %d", got)
	}
	if got := tr.Reclaimable(tr.Root()); got != 4<<20 {
		t.Fatalf("root %d", got)
	}
}

func TestLoadRejectsCorrupt(t *testing.T) {
	tr := New("/r")
	tr.Write(func(w *Writer) {
		a := w.AddDir(tr.Root(), []byte("a"), 1, 0)
		w.AddFile(a, []byte("f"), KindFile, 2<<20, 2<<20, 2<<20, 1, 0)
	})
	var good bytes.Buffer
	tr.Save(&good)
	raw := gunzip(t, good.Bytes())
	corrupt := func(mut func([]byte) []byte) error {
		_, err := Load(bytes.NewReader(gzipBytes(mut(append([]byte(nil), raw...)))))
		return err
	}
	headLen := 4 + 4 + 4 + 2 // magic, version, root length, "/r"
	if err := corrupt(func(b []byte) []byte { binary.LittleEndian.PutUint64(b[headLen:], 1<<40); return b }); err == nil {
		t.Fatal("absurd node count accepted")
	}
	if err := corrupt(func(b []byte) []byte { binary.LittleEndian.PutUint32(b[headLen+8+4:], 999); return b }); err == nil {
		t.Fatal("parent index out of range accepted")
	}
	if err := corrupt(func(b []byte) []byte { return b[:len(b)/2] }); err == nil {
		t.Fatal("truncated file accepted")
	}
}

func gunzip(t *testing.T, b []byte) []byte {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(zr)
	return out
}

func gzipBytes(b []byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(b)
	zw.Close()
	return buf.Bytes()
}

func TestRemoveSubtractsUpChain(t *testing.T) {
	tr, a, b := build(t)
	before := tr.Node(tr.Root())
	tr.Write(func(w *Writer) { w.Remove(b) })
	root, an := tr.Node(tr.Root()), tr.Node(a)
	if root.Alloc != before.Alloc-5<<20 || an.Alloc != 3000 || an.Files != 3 {
		t.Fatalf("root %+v a %+v", root, an)
	}
	if _, ok := tr.Lookup("/r/a/b"); ok || len(tr.Children(a)) != 0 {
		t.Fatal("removed node still reachable")
	}
}
