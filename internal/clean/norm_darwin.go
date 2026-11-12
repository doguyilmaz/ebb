package clean

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
#include "norm_darwin.h"
*/
import "C"

import "unsafe"

// composed spells s with composed letters (Unicode NFC). APFS takes "ü" and
// "u" followed by a combining diaeresis for the same name: Finder and most
// apps store the second, while a name typed in a terminal or a text field
// is usually the first. Names in plain ASCII, most of them, need no call.
func composed(s string) string {
	ascii := true
	for i := 0; i < len(s) && ascii; i++ {
		ascii = s[i] < 0x80
	}
	if ascii {
		return s
	}
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	out := C.ebb_composed(cs)
	if out == nil {
		return s
	}
	defer C.free(unsafe.Pointer(out))
	return C.GoString(out)
}
