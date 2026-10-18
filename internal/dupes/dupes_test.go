package dupes

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/volume"
	"github.com/doguyilmaz/ebb/internal/walk"
)

func write(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func content(seed byte, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = seed + byte(i%251)
	}
	return b
}

func TestFindGroupsIdenticalFilesOnly(t *testing.T) {
	root := t.TempDir()
	a := content(1, 3<<20)
	write(t, filepath.Join(root, "Downloads", "movie.mp4"), a)
	write(t, filepath.Join(root, "Documents", "movie copy.mp4"), a)
	// Same size, same first and last 64 KiB, another middle: not a copy.
	b := bytes.Clone(a)
	b[len(b)/2] ^= 0xff
	write(t, filepath.Join(root, "Documents", "edited.mp4"), b)
	// Copies inside bundles, dot-folders and ~/Library belong to something else.
	write(t, filepath.Join(root, "Applications", "Tool.app", "Contents", "movie.mp4"), a)
	write(t, filepath.Join(root, "Parallels", "Win.pvm", "disk.hds", "movie.mp4"), a)
	write(t, filepath.Join(root, "Backup.sparsebundle", "bands", "0"), a)
	write(t, filepath.Join(root, ".cache", "movie.mp4"), a)
	write(t, filepath.Join(root, "Library", "Caches", "movie.mp4"), a)
	// A hardlink already shares its data.
	if err := os.Link(filepath.Join(root, "Downloads", "movie.mp4"), filepath.Join(root, "Downloads", "link.mp4")); err != nil {
		t.Fatal(err)
	}
	small := content(9, 512<<10)
	write(t, filepath.Join(root, "small1"), small)
	write(t, filepath.Join(root, "small2"), small)

	scan, err := walk.Scan(context.Background(), walk.Options{Root: root}, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	var phases []string
	res, err := Find(context.Background(), scan.Tree, volume.Firmlinks{}, scan.Tree.Root(), 1<<20, func(p Progress) { phases = append(phases, p.Phase) })
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Groups) != 1 {
		t.Fatalf("groups %+v", res.Groups)
	}
	var names []string
	for _, c := range res.Groups[0].Copies {
		names = append(names, filepath.Base(c.Path))
	}
	slices.Sort(names)
	if len(names) != 2 || !slices.Contains(names, "movie copy.mp4") || slices.Contains(names, "edited.mp4") {
		t.Fatalf("copies %q", names)
	}
	if res.Wasted < 3<<20 || res.Groups[0].Size != 3<<20 {
		t.Fatalf("wasted %d size %d", res.Wasted, res.Groups[0].Size)
	}
	if !slices.Contains(phases, "hash") {
		t.Fatalf("progress %q", phases)
	}
}

func TestFindStopsWhenCancelled(t *testing.T) {
	root := t.TempDir()
	a := content(3, 2<<20)
	for _, n := range []string{"a", "b", "c"} {
		write(t, filepath.Join(root, n), a)
	}
	scan, err := walk.Scan(context.Background(), walk.Options{Root: root}, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Find(ctx, scan.Tree, volume.Firmlinks{}, scan.Tree.Root(), 1<<20, nil); err == nil {
		t.Fatal("a cancelled search ends with its error")
	}
}

func TestDigestStopsMidFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "big")
	write(t, p, content(4, 3<<20))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := digest(ctx, p, 3<<20, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("a stopped search still read the whole file: %v", err)
	}
}
