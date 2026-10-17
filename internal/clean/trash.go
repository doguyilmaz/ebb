package clean

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/doguyilmaz/ebb/internal/sys"
)

// folderTrash moves items into a Trash folder by renaming; the sandbox and
// non-macOS builds use it.
type folderTrash struct{ dir string }

func (t folderTrash) Trash(path string) (string, error) {
	if err := os.MkdirAll(t.dir, 0o700); err != nil {
		return "", err
	}
	base := filepath.Base(path)
	dst := filepath.Join(t.dir, base)
	for i := 2; ; i++ {
		if _, err := os.Lstat(dst); os.IsNotExist(err) {
			break
		}
		ext := filepath.Ext(base)
		dst = filepath.Join(t.dir, fmt.Sprintf("%s %d%s", strings.TrimSuffix(base, ext), i, ext))
	}
	return dst, os.Rename(path, dst)
}

func newTrasher(env sys.Env) Trasher {
	if sys.Sandboxed(env) {
		return folderTrash{filepath.Join(env.Home(), ".Trash")}
	}
	return systemTrash(env)
}

// NewTrasher returns the Trash for this machine (or the sandbox's).
func NewTrasher(env sys.Env) Trasher { return newTrasher(env) }
