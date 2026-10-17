package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanAgainListsOnlyWhatChanged(t *testing.T) {
	a, root := testApp(t)
	ctx := context.Background()
	first, err := a.Scan(ctx, ScanOptions{Root: root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Meta.EventID == 0 || first.Meta.FSEvents == "" || first.Meta.Incremental {
		t.Fatalf("a first scan is full and records where it stands: %+v", first.Meta)
	}
	write(t, filepath.Join(root, "docs", "b.bin"), 4<<20)
	write(t, filepath.Join(root, "new", "c.bin"), 3<<20)
	if err := os.Remove(filepath.Join(root, "docs", "a.txt")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)

	again, err := a.Scan(ctx, ScanOptions{Root: root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Meta.Incremental || again.Meta.Changed == 0 || again.Meta.EventID <= first.Meta.EventID {
		t.Fatalf("scan again should update in place: %+v", again.Meta)
	}
	full, err := a.Scan(ctx, ScanOptions{Root: root, Full: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if full.Meta.Incremental {
		t.Fatal("asked for a full scan")
	}
	got, want := again.Tree.Node(again.Tree.Root()), full.Tree.Node(full.Tree.Root())
	if got.Alloc != want.Alloc || got.Unique != want.Unique || got.Files != want.Files {
		t.Fatalf("updated %+v, full %+v", got, want)
	}
	if len(again.Findings) != len(full.Findings) {
		t.Fatalf("findings after update %d, after full scan %d", len(again.Findings), len(full.Findings))
	}
}
