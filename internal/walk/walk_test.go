package walk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/tree"
)

var generic = Options{Reader: "generic"}

func scan(t *testing.T, root string, opt Options) *Result {
	t.Helper()
	opt.Root = root
	res, err := Scan(context.Background(), opt, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestScanTotalsAndSmallFolding(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "a", "big"), 3<<20)
	for i := 0; i < 5; i++ {
		mkfile(t, filepath.Join(root, "a", fmt.Sprintf("s%d", i)), 1000)
	}
	res := scan(t, root, generic)
	a, ok := res.Tree.Lookup(filepath.Join(root, "a"))
	if !ok {
		t.Fatal("a missing")
	}
	n := res.Tree.Node(a)
	if n.Files != 6 || n.SmallCount != 5 || len(res.Tree.Children(a)) != 1 {
		t.Fatalf("%+v children=%d", n, len(res.Tree.Children(a)))
	}
	if n.Alloc < 3<<20 {
		t.Fatalf("alloc %d", n.Alloc)
	}
	if res.Dirs != 2 || res.Files != 6 {
		t.Fatalf("counts dirs=%d files=%d", res.Dirs, res.Files)
	}
}

func TestScanPermissionDenied(t *testing.T) {
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	mkfile(t, filepath.Join(locked, "x"), 2<<20)
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	res := scan(t, root, generic)
	id, ok := res.Tree.Lookup(locked)
	if !ok || res.Tree.Node(id).Flags&tree.FlagDenied == 0 || len(res.Problems) != 1 {
		t.Fatalf("denied dir not reported: ok=%v problems=%v", ok, res.Problems)
	}
	if res.Problems[0].Path != locked {
		t.Fatalf("problem path %q", res.Problems[0].Path)
	}
}

func TestScanOddNames(t *testing.T) {
	root := t.TempDir()
	names := []string{"été", "with space", "new\nline", strings.Repeat("x", 255)}
	for _, n := range names {
		mkfile(t, filepath.Join(root, n), 2<<20)
	}
	res := scan(t, root, generic)
	for _, n := range names {
		if _, ok := res.Tree.Lookup(filepath.Join(root, n)); !ok {
			t.Fatalf("missing %q", n)
		}
	}
}

func TestScanPathTooLong(t *testing.T) {
	root := t.TempDir()
	r, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	depth := len(root)
	for depth < 1200 {
		name := strings.Repeat("d", 200)
		if err := r.Mkdir(name, 0o755); err != nil {
			t.Skipf("cannot build deep path: %v", err)
		}
		next, err := r.OpenRoot(name)
		if err != nil {
			t.Skipf("cannot descend: %v", err)
		}
		r.Close()
		r = next
		depth += 201
	}
	r.Close()
	res := scan(t, root, generic)
	if len(res.Problems) == 0 {
		t.Fatal("a path beyond PATH_MAX must surface as a problem, not vanish")
	}
}

func TestScanSymlinkLoopNotFollowed(t *testing.T) {
	root := fixture(t)
	res := scan(t, root, generic)
	id, ok := res.Tree.Lookup(filepath.Join(root, "loop"))
	if ok && res.Tree.Node(id).Kind == tree.KindDir {
		t.Fatal("symlink followed")
	}
}

func TestScanHardlinksOnce(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "orig"), 4<<20)
	if err := os.Link(filepath.Join(root, "orig"), filepath.Join(root, "copy")); err != nil {
		t.Fatal(err)
	}
	res := scan(t, root, generic)
	if got := res.Tree.Node(res.Tree.Root()).Alloc; got > 4<<20+64<<10 {
		t.Fatalf("hardlink counted twice: %d", got)
	}
}

func TestScanProjectMarker(t *testing.T) {
	root := fixture(t)
	res := scan(t, root, generic)
	id, _ := res.Tree.Lookup(filepath.Join(root, "proj"))
	if res.Tree.Node(id).Flags&tree.FlagProject == 0 {
		t.Fatal("package.json must flag the project")
	}
	a, _ := res.Tree.Lookup(filepath.Join(root, "a"))
	if res.Tree.Node(a).Flags&tree.FlagProject != 0 {
		t.Fatal("a is not a project")
	}
}

func manyDirs(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	for i := 0; i < n; i++ {
		if err := os.MkdirAll(filepath.Join(root, fmt.Sprintf("p%d", i%50), fmt.Sprintf("d%d", i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestScanCancel(t *testing.T) {
	root := manyDirs(t, 5000)
	ctx, cancel := context.WithCancel(context.Background())
	s, err := Start(ctx, Options{Root: root, Reader: "generic", Threads: 2})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	done := make(chan error, 1)
	go func() { _, err := s.Wait(); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not stop the scan")
	}
}

func TestScanLiveReads(t *testing.T) {
	root := manyDirs(t, 3000)
	s, err := Start(context.Background(), Options{Root: root, Reader: "generic"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = s.Tree().Node(s.Tree().Root())
				_ = s.Progress()
			}
		}
	}()
	res, err := s.Wait()
	close(stop)
	wg.Wait()
	if err != nil || res.Dirs != 3051 {
		t.Fatalf("dirs=%d err=%v", res.Dirs, err)
	}
}

func TestScanRootMustBeDir(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "f"), 10)
	if _, err := Start(context.Background(), Options{Root: filepath.Join(root, "f")}); err == nil {
		t.Fatal("a file root must be rejected")
	}
}
