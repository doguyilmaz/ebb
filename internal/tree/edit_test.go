package tree

import "testing"

// editTree: /r/d holds a stored file, three small files and a subfolder s with
// its own stored file; /r/gone is a second folder.
func editTree() (*Tree, ID, ID, ID) {
	tr := New("/r")
	var d, s, gone ID
	tr.Write(func(w *Writer) {
		d = w.AddDir(tr.Root(), []byte("d"), 1, 0)
		w.AddFile(d, []byte("f1"), KindFile, 2<<20, 2<<20, 2<<20, 5, 0)
		w.AddSmall(d, 3, 1<<20, 1<<20, 1<<20, 6)
		w.SetMarkers(d, MarkerBit("package.json"))
		s = w.AddDir(d, []byte("s"), 1, 0)
		w.AddFile(s, []byte("f2"), KindFile, 3<<20, 3<<20, 3<<20, 7, 0)
		gone = w.AddDir(tr.Root(), []byte("gone"), 1, 0)
		w.AddFile(gone, []byte("g"), KindFile, 4<<20, 4<<20, 4<<20, 8, 0)
	})
	tr.Finish()
	return tr, d, s, gone
}

func TestClearOwnKeepsSubfolders(t *testing.T) {
	tr, d, s, _ := editTree()
	tr.Write(func(w *Writer) { w.ClearOwn(d) })
	n := tr.Node(d)
	if n.Alloc != 3<<20 || n.Files != 1 || n.SmallCount != 0 || n.Markers != 0 {
		t.Fatalf("d after clear %+v", n)
	}
	if got := tr.Children(d); len(got) != 1 || got[0] != s {
		t.Fatalf("children %v", got)
	}
	if root := tr.Node(tr.Root()); root.Alloc != 7<<20 || root.Files != 2 {
		t.Fatalf("root %+v", root)
	}
}

func TestCompactDropsRemovedNodes(t *testing.T) {
	tr, d, _, gone := editTree()
	tr.Write(func(w *Writer) {
		w.Link(42, 2, 4<<20, ID(tr.first[gone]))
		w.Link(43, 2, 3<<20, d)
		w.Remove(gone)
	})
	before := tr.Len()
	tr.Compact()
	if tr.Len() != before-2 {
		t.Fatalf("len %d, want %d", tr.Len(), before-2)
	}
	if got := tr.Find("g", 10); len(got) != 0 {
		for _, id := range got {
			if tr.Node(id).Name == "g" {
				t.Fatalf("search still finds the removed file")
			}
		}
	}
	if _, ok := tr.Lookup("/r/d/s/f2"); !ok {
		t.Fatal("kept nodes must stay reachable")
	}
	if len(tr.links) != 1 || tr.links[43] == nil {
		t.Fatalf("groups %v", tr.links)
	}
	if id, _ := tr.Lookup("/r/d"); tr.links[43].first != id {
		t.Fatal("group must point at the new id")
	}
	if err := tr.validate(); err != nil {
		t.Fatal(err)
	}
	if root := tr.Node(tr.Root()); root.Alloc != 6<<20 {
		t.Fatalf("root alloc %d", root.Alloc)
	}
	_ = tr.Reclaimable(tr.Root())
}

func TestRejoinKeepsTheCountedCopy(t *testing.T) {
	tr, d, _, gone := editTree()
	var f ID
	tr.Write(func(w *Writer) {
		f = ID(tr.first[d])
		w.Link(7, 2, 2<<20, f)
		w.Link(7, 2, 2<<20, gone)
	})
	own := func(id ID) bool { return id == d || id == f }
	tr.Write(func(w *Writer) {
		if w.SharedElsewhere(true, 7, own) {
			t.Fatal("the copy counted in d is d's own")
		}
		w.Rejoin(true, 7, 2, 2<<20, d, own)
		if g := tr.links[7]; g.first != d || g.seen != 2 {
			t.Fatalf("group %+v", g)
		}
		if !w.SharedElsewhere(true, 7, func(id ID) bool { return id == gone }) {
			t.Fatal("seen from gone, the copy is counted elsewhere")
		}
	})
}
