// Package appkit gives the background service an app's place in macOS when it
// runs from ebb.app: notifications from ebb whose click opens the window, and
// reopening the app opening the window too.
package appkit

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework UserNotifications
#include <stdlib.h>
#include "appkit_darwin.h"
*/
import "C"

import "unsafe"

var open func()

//export ebbOpenWindow
func ebbOpenWindow() {
	if open != nil {
		go open()
	}
}

// Bundled reports whether this process runs from ebb.app; nothing else here works without it.
func Bundled() bool { return C.ebb_bundled() != 0 }

// Run serves app events on the main thread and never returns: it must be
// called from the main goroutine, locked to the main thread before main runs.
func Run(onOpen func()) {
	open = onOpen
	C.ebb_run()
}

func Notify(title, body string) {
	t, b := C.CString(title), C.CString(body)
	defer C.free(unsafe.Pointer(t))
	defer C.free(unsafe.Pointer(b))
	C.ebb_notify(t, b)
}
