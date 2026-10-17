package clean

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
#include "trash_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/doguyilmaz/ebb/internal/sys"
)

type finderTrash struct{}

func (finderTrash) Trash(path string) (string, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	buf := make([]byte, 4096)
	if rc := C.ebb_trash(cpath, (*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf))); rc != 0 {
		return "", fmt.Errorf("could not move %s to the Trash (code %d)", path, int(rc))
	}
	return C.GoString((*C.char)(unsafe.Pointer(&buf[0]))), nil
}

func systemTrash(sys.Env) Trasher { return finderTrash{} }
