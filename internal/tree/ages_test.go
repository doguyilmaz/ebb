package tree

import (
	"fmt"
	"testing"
)

func TestAgesBuckets(t *testing.T) {
	const day = 86400
	now := int64(1000 * day)
	tr := New("/r")
	tr.Write(func(w *Writer) {
		d := w.AddDir(tr.Root(), []byte("d"), now, 0)
		w.AddFile(d, []byte("new.bin"), KindFile, 4<<20, 4<<20, 4<<20, now-2*day, 0)
		w.AddFile(tr.Root(), []byte("old.iso"), KindFile, 8<<20, 8<<20, 8<<20, now-400*day, 0)
		w.AddSmall(d, 3, 1<<20, 1<<20, 1<<20, now-40*day)
	})
	tr.Finish()
	got := tr.Ages(tr.Root(), now, []int64{7 * day, 30 * day, 365 * day})
	// The small files share their folder's newest mtime, which new.bin made recent.
	if fmt.Sprint(got) != fmt.Sprint([]uint64{5 << 20, 0, 0, 8 << 20}) {
		t.Fatalf("ages %v", got)
	}
	var sum uint64
	for _, b := range got {
		sum += b
	}
	if sum != tr.Node(tr.Root()).Alloc {
		t.Fatalf("buckets sum %d, alloc %d", sum, tr.Node(tr.Root()).Alloc)
	}
	d, _ := tr.Lookup("/r/d")
	if got := tr.Ages(d, now, []int64{7 * day}); fmt.Sprint(got) != fmt.Sprint([]uint64{5 << 20, 0}) {
		t.Fatalf("ages under d %v", got)
	}
}
