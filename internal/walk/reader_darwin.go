package walk

/*
#cgo CFLAGS: -O2 -Wall
#include "bulk_darwin.h"
*/
import "C"

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/doguyilmaz/ebb/internal/tree"
)

func setIOPolicy(gentle bool) {
	throttle := 0
	if gentle {
		throttle = 1
	}
	C.ebb_set_iopolicy(C.int(throttle))
}

// benchReader overrides the reader in benchmarks.
var benchReader reader

func newReader(kind string) reader {
	if benchReader != nil {
		return benchReader
	}
	if kind == "generic" {
		return genericReader{}
	}
	return newBulkReader()
}

const (
	attrBufSize = 256 << 10
	maxEntries  = 8192
)

type scratch struct {
	attr  []byte
	names []byte
	out   []C.ebb_entry
	name  []byte // NUL-terminated child name for openat
}

type bulkReader struct {
	scratch sync.Pool
	// noClones is set when the kernel rejects the clone attributes (older
	// macOS); scans then fall back to the basic attribute set.
	noClones atomic.Bool
}

// oNoFollowAny refuses a symlink anywhere in the path (macOS 11+).
const oNoFollowAny = 0x20000000

func newBulkReader() *bulkReader {
	r := &bulkReader{}
	r.scratch.New = func() any {
		return &scratch{attr: make([]byte, attrBufSize), names: make([]byte, attrBufSize), out: make([]C.ebb_entry, maxEntries)}
	}
	return r
}

const openFlags = syscall.O_RDONLY | syscall.O_DIRECTORY | syscall.O_CLOEXEC | oNoFollowAny

func (r *bulkReader) open(j job) (int, error) {
	if j.parent == nil {
		return syscall.Open(j.path, openFlags, 0)
	}
	sc := r.scratch.Get().(*scratch)
	defer r.scratch.Put(sc)
	sc.name = append(append(sc.name[:0], j.path[strings.LastIndexByte(j.path, '/')+1:]...), 0)
	fd := int(C.ebb_openat(C.int(j.parent.fd), (*C.char)(unsafe.Pointer(&sc.name[0])), C.int(openFlags)))
	if fd < 0 {
		return -1, syscall.Errno(-fd)
	}
	return fd, nil
}

func (r *bulkReader) readDir(s *Session, j job, b *batch) error {
	path := j.path
	fd, err := r.open(j)
	if err != nil {
		return &os.PathError{Op: "open", Path: path, Err: err}
	}
	if err := r.read(fd, path, b); err != nil {
		syscall.Close(fd)
		return err
	}
	if d, ok := s.keep(fd); ok {
		b.fd = d
	} else {
		syscall.Close(fd)
	}
	return nil
}

func (r *bulkReader) read(fd int, path string, b *batch) error {
	sc := r.scratch.Get().(*scratch)
	defer r.scratch.Put(sc)
	for {
		var used C.size_t
		wantClone := C.int(1)
		if r.noClones.Load() {
			wantClone = 0
		}
		n := C.ebb_bulk(C.int(fd), (*C.char)(unsafe.Pointer(&sc.attr[0])), C.size_t(len(sc.attr)),
			&sc.out[0], C.int(len(sc.out)), (*C.char)(unsafe.Pointer(&sc.names[0])), C.size_t(len(sc.names)), &used, wantClone)
		if n == -C.int(syscall.EINVAL) && wantClone == 1 {
			r.noClones.Store(true)
			continue
		}
		if n < 0 {
			return &os.PathError{Op: "getattrlistbulk", Path: path, Err: syscall.Errno(-n)}
		}
		if n == 0 {
			return nil
		}
		base := uint32(len(b.names))
		b.names = append(b.names, sc.names[:used]...)
		for i := range int(n) {
			ce := &sc.out[i]
			b.entries = append(b.entries, entry{
				nameOff: base + uint32(ce.name_off), nameLen: uint16(ce.name_len), kind: tree.Kind(ce.kind), mount: ce.mount != 0,
				alloc: uint64(ce.alloc), logical: uint64(ce.logical),
				mtime: int64(ce.mtime), nlink: uint32(ce.nlink), ino: uint64(ce.ino), flags: uint32(ce.flags), ext: uint32(ce.ext),
				cloneID: uint64(ce.clone_id), cloneRefs: uint32(ce.clone_refs),
				err: syscall.Errno(ce.err),
			})
		}
	}
}
