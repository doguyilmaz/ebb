//go:build unix

package walk

import (
	"os"
	"syscall"

	"github.com/doguyilmaz/ebb/internal/tree"
)

// genericReader lists a directory with ReadDir and stats each entry: portable
// and the reference the faster platform readers are checked against.
type genericReader struct{}

func (genericReader) readDir(path string, b *batch) error {
	var dirSt syscall.Stat_t
	if err := syscall.Stat(path, &dirSt); err != nil {
		return &os.PathError{Op: "stat", Path: path, Err: err}
	}
	des, err := os.ReadDir(path)
	if err != nil && len(des) == 0 {
		return err
	}
	var st syscall.Stat_t
	for _, de := range des {
		name := de.Name()
		e := entry{}
		e.nameOff, e.nameLen = b.addName(name)
		if err := syscall.Lstat(childPath(path, []byte(name)), &st); err != nil {
			if errno, ok := err.(syscall.Errno); ok {
				e.err = errno
			} else {
				e.err = syscall.EIO
			}
			b.entries = append(b.entries, e)
			continue
		}
		f := statFields(&st)
		dirDev := statFields(&dirSt).dev
		switch st.Mode & syscall.S_IFMT {
		case syscall.S_IFDIR:
			e.kind = tree.KindDir
			e.mount = f.dev != dirDev
		case syscall.S_IFREG:
			e.kind = tree.KindFile
		case syscall.S_IFLNK:
			e.kind = tree.KindLink
		default:
			e.kind = tree.KindOther
		}
		e.alloc, e.logical, e.nlink, e.ino, e.mtime, e.flags = f.alloc, f.logical, f.nlink, f.ino, f.mtime, f.flags
		b.entries = append(b.entries, e)
	}
	return nil
}

type fields struct {
	dev            uint64
	ino            uint64
	alloc, logical uint64
	nlink          uint32
	mtime          int64
	flags          uint32
}
