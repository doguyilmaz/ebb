package walk

import (
	"os"
	"path/filepath"
	"testing"
)

func mkfile(t testing.TB, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = byte(i*7 + 1)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture builds a tree exercising the cases both readers must agree on.
func fixture(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "a", "big.bin"), 3<<20)
	for _, n := range []string{"s1", "s2", "s3"} {
		mkfile(t, filepath.Join(root, "a", "b", n), 1500)
	}
	mkfile(t, filepath.Join(root, "proj", "package.json"), 20)
	mkfile(t, filepath.Join(root, "proj", "node_modules", "x", "index.js"), 4000)
	mkfile(t, filepath.Join(root, "été", "with space"), 2<<20)
	mkfile(t, filepath.Join(root, "linked", "orig"), 2<<20)
	if err := os.Link(filepath.Join(root, "linked", "orig"), filepath.Join(root, "linked", "copy")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(root, "loop")); err != nil {
		t.Fatal(err)
	}
	return root
}
