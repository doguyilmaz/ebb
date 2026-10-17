package walk

/*
#cgo CFLAGS: -O2 -Wall
#include "bulk_darwin.h"
*/
import "C"

import (
	"os"
	"sync"
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
}

type bulkReader struct {
	scratch sync.Pool
}

func newBulkReader() *bulkReader {
	r := &bulkReader{}
	r.scratch.New = func() any {
		return &scratch{attr: make([]byte, attrBufSize), names: make([]byte, attrBufSize), out: make([]C.ebb_entry, maxEntries)}
	}
	return r
}

func (r *bulkReader) readDir(path string, b *batch) error {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return &os.PathError{Op: "open", Path: path, Err: err}
	}
	defer syscall.Close(fd)
	sc := r.scratch.Get().(*scratch)
	defer r.scratch.Put(sc)
	for {
		var used C.size_t
		n := C.ebb_bulk(C.int(fd), (*C.char)(unsafe.Pointer(&sc.attr[0])), C.size_t(len(sc.attr)),
			&sc.out[0], C.int(len(sc.out)), (*C.char)(unsafe.Pointer(&sc.names[0])), C.size_t(len(sc.names)), &used)
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
