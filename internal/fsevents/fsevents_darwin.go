// Package fsevents tells which folders changed on a volume, live and since a
// saved point in its history, so a rescan can list only those folders again.
package fsevents

/*
#cgo CFLAGS: -O2 -Wall
#cgo LDFLAGS: -framework CoreServices
#include <stdlib.h>
#include "fsevents_darwin.h"
*/
import "C"

import (
	"context"
	"errors"
	"runtime/cgo"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"
)

// Current is the volume-wide event ID now; a later Watch from it misses nothing.
func Current() uint64 { return uint64(C.ebb_fse_current()) }

// UUID names the FSEvents history of the volume holding path. Event IDs only
// mean something within one history; a different UUID means it was reset.
func UUID(path string) string {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	buf := make([]byte, 64)
	if C.ebb_fse_uuid(cpath, (*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf))) != 0 {
		return ""
	}
	return C.GoString((*C.char)(unsafe.Pointer(&buf[0])))
}

// Watch reports folders that change under root until ctx ends. With since set
// it first replays history after that ID, then marks the end with a
// HistoryDone event, then goes on live. fn runs on the stream's own thread,
// one batch at a time. A request on flush hands fn everything that happened
// so far, latency or not, and is answered by closing it.
func Watch(ctx context.Context, root string, since uint64, latency time.Duration, fn func([]Event), flush <-chan chan struct{}) error {
	var last atomic.Int64
	h := cgo.NewHandle(func(evs []Event) {
		fn(evs)
		last.Store(time.Now().UnixNano())
	})
	defer h.Delete()
	croot := C.CString(root)
	defer C.free(unsafe.Pointer(croot))
	w := C.ebb_watch_start(croot, C.uint64_t(since), C.double(latency.Seconds()), C.uintptr_t(h))
	if w == nil {
		return errors.New("fsevents: could not watch " + root)
	}
	for {
		select {
		case <-ctx.Done():
			C.ebb_watch_stop(w)
			return nil
		case done := <-flush:
			C.ebb_watch_flush(w)
			settle(&last)
			close(done)
		}
	}
}

// settle waits for the events a flush released: the stream hands them over
// a moment after the flush returns, so wait until it has been quiet briefly.
func settle(last *atomic.Int64) {
	start := time.Now()
	for time.Since(start) < 250*time.Millisecond {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) >= 30*time.Millisecond && time.Since(time.Unix(0, last.Load())) >= 20*time.Millisecond {
			return
		}
	}
}

//export ebbFSEvents
func ebbFSEvents(handle C.uintptr_t, n C.size_t, paths **C.char, flags *C.uint32_t, ids *C.uint64_t) {
	fn := cgo.Handle(handle).Value().(func([]Event))
	ps := unsafe.Slice(paths, int(n))
	fs := unsafe.Slice(flags, int(n))
	is := unsafe.Slice(ids, int(n))
	out := make([]Event, int(n))
	for i := range out {
		p := C.GoString(ps[i])
		if len(p) > 1 {
			p = strings.TrimSuffix(p, "/")
		}
		out[i] = Event{Path: p, Flags: uint32(fs[i]), ID: uint64(is[i])}
	}
	fn(out)
}
