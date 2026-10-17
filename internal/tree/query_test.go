package tree

import (
	"bytes"
	"fmt"
	"testing"
)

func filesTree() *Tree {
	tr := New("/r")
	tr.Write(func(w *Writer) {
		d := w.AddDir(tr.Root(), []byte("d"), 1, 0)
		w.AddFile(d, []byte("big.bin"), KindFile, 9<<20, 9<<20, 9<<20, 1, 0)
		w.AddFile(tr.Root(), []byte("mid.iso"), KindFile, 5<<20, 5<<20, 5<<20, 1, 0)
		w.AddFile(d, []byte("small.txt"), KindFile, 2<<20, 2<<20, 2<<20, 1, 0)
	})
	tr.Finish()
	return tr
}

func TestTopFiles(t *testing.T) {
	tr := filesTree()
	top := tr.Top(tr.Root(), 2)
	if len(top) != 2 || tr.Node(top[0]).Name != "big.bin" || tr.Node(top[1]).Name != "mid.iso" {
		t.Fatalf("top %v", top)
	}
	d, _ := tr.Lookup("/r/d")
	if top := tr.Top(d, 5); len(top) != 2 {
		t.Fatalf("top under d: %v", top)
	}
}

func TestFindGlobAndSubstring(t *testing.T) {
	tr := filesTree()
	if got := tr.Find("*.bin", 10); len(got) != 1 || tr.Node(got[0]).Name != "big.bin" {
		t.Fatalf("glob %v", got)
	}
	if got := tr.Find("BIG", 10); len(got) != 1 {
		t.Fatalf("substring %v", got)
	}
	if got := tr.Find("i", 1); len(got) != 1 {
		t.Fatalf("limit %v", got)
	}
}

func TestWalkDirsSkipsSubtree(t *testing.T) {
	tr := New("/r")
	tr.Write(func(w *Writer) {
		a := w.AddDir(tr.Root(), []byte("a"), 1, 0)
		w.AddDir(a, []byte("deep"), 1, 0)
		w.AddDir(tr.Root(), []byte("b"), 1, 0)
	})
	var seen []string
	tr.WalkDirs(tr.Root(), func(n Node) bool {
		seen = append(seen, n.Name)
		return n.Name != "a"
	})
	if fmt.Sprint(seen) != "[/r b a]" && fmt.Sprint(seen) != "[/r a b]" {
		t.Fatalf("seen %v", seen)
	}
}

func TestSliceMergesSmallSiblings(t *testing.T) {
	tr := New("/r")
	tr.Write(func(w *Writer) {
		for i, pct := range []uint64{90, 5, 3, 2} {
			w.AddFile(tr.Root(), []byte(fmt.Sprintf("f%d", i)), KindFile, pct<<20, pct<<20, pct<<20, 1, 0)
		}
	})
	tr.Finish()
	s := tr.Slice(tr.Root(), 1, 0.04, 100)
	if len(s.Children) != 3 {
		t.Fatalf("want 2 real children + other, got %d", len(s.Children))
	}
	other := s.Children[2]
	if other.Other != 2 || other.Alloc != 5<<20 || other.ID != None {
		t.Fatalf("other %+v", other)
	}
}

func TestSliceSmallFilesBucket(t *testing.T) {
	tr := New("/r")
	tr.Write(func(w *Writer) {
		w.AddFile(tr.Root(), []byte("f"), KindFile, 4<<20, 4<<20, 4<<20, 1, 0)
		w.AddSmall(tr.Root(), 10, 4<<20, 4<<20, 4<<20, 1)
	})
	tr.Finish()
	s := tr.Slice(tr.Root(), 1, 0.01, 100)
	var small *SliceNode
	for _, c := range s.Children {
		if c.Kind == KindSmall {
			small = c
		}
	}
	if small == nil || small.Files != 10 || small.Name != "10 small files" {
		t.Fatalf("small bucket missing: %+v", s.Children)
	}
}

func TestSliceBudget(t *testing.T) {
	tr := New("/r")
	tr.Write(func(w *Writer) {
		for i := 0; i < 5; i++ {
			d := w.AddDir(tr.Root(), []byte(fmt.Sprintf("d%d", i)), 1, 0)
			for j := 0; j < 200; j++ {
				w.AddFile(d, []byte(fmt.Sprintf("f%d", j)), KindFile, 1<<20, 1<<20, 1<<20, 1, 0)
			}
		}
	})
	tr.Finish()
	s := tr.Slice(tr.Root(), 3, 0, 10)
	count := 0
	var walk func(n *SliceNode)
	walk = func(n *SliceNode) {
		count++
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(s)
	if count > 10 {
		t.Fatalf("budget exceeded: %d nodes", count)
	}
	if len(s.Children) != 5 {
		t.Fatalf("depth 1 must be complete before depth 2: %d", len(s.Children))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tr := New("/r")
	var leaf ID
	tr.Write(func(w *Writer) {
		a := w.AddDir(tr.Root(), []byte("a"), 10, FlagProject)
		leaf = w.AddFile(a, []byte("é.bin"), KindFile, 3<<20, 1<<20, 3<<20, 20, FlagClone)
		w.AddSmall(a, 2, 100, 100, 90, 30)
		w.Link(9, 2, 3<<20, a)
	})
	tr.Finish()
	var buf bytes.Buffer
	if err := tr.Save(&buf); err != nil {
		t.Fatal(err)
	}
	got, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Len() != tr.Len() || got.RootPath() != "/r" {
		t.Fatal("shape")
	}
	for id := ID(0); int(id) < tr.Len(); id++ {
		if got.Node(id) != tr.Node(id) {
			t.Fatalf("node %d: %+v != %+v", id, got.Node(id), tr.Node(id))
		}
	}
	if got.Path(leaf) != "/r/a/é.bin" || got.Reclaimable(tr.Root()) != tr.Reclaimable(tr.Root()) {
		t.Fatal("path or links lost")
	}
}
