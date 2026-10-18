package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/sys"
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

func TestFailedUpdateGoesBackToTheSavedScan(t *testing.T) {
	a, root := testApp(t)
	ctx := context.Background()
	first, err := a.Scan(ctx, ScanOptions{Root: root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	before := first.Tree.Node(first.Tree.Root()).Alloc
	write(t, filepath.Join(root, "new", "c.bin"), 3<<20)
	time.Sleep(1500 * time.Millisecond)

	snaps := filepath.Join(sys.StateDir(a.Env), "snapshots")
	if err := os.Chmod(snaps, 0o500); err != nil {
		t.Fatal(err)
	}
	_, err = a.Scan(ctx, ScanOptions{Root: root}, nil)
	os.Chmod(snaps, 0o755)
	if err == nil {
		t.Fatal("saving the snapshot should have failed")
	}
	back, err := a.Last()
	if err != nil {
		t.Fatal(err)
	}
	if back == first || back.Tree.Node(back.Tree.Root()).Alloc != before {
		t.Fatal("after a failed update the saved scan is what counts, not the half-edited tree")
	}
	again, err := a.Scan(ctx, ScanOptions{Root: root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	full, err := a.Scan(ctx, ScanOptions{Root: root, Full: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Meta.Incremental || again.Tree.Node(again.Tree.Root()) != full.Tree.Node(full.Tree.Root()) {
		t.Fatalf("the changes are kept for the next update: %+v vs %+v", again.Tree.Node(again.Tree.Root()), full.Tree.Node(full.Tree.Root()))
	}
}
