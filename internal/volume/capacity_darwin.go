package volume

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
#include "capacity_darwin.h"
*/
import "C"

import "unsafe"

func capacity(path string) (available, important uint64, ok bool) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var a, i C.int64_t
	if C.ebb_capacity(cpath, &a, &i) != 0 {
		return 0, 0, false
	}
	return uint64(a), uint64(i), true
}
