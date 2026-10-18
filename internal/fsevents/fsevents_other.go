//go:build !darwin

package fsevents

import (
	"context"
	"errors"
	"time"
)

// ErrUnsupported: without FSEvents every scan is complete.
var ErrUnsupported = errors.New("fsevents: not available on this system")

func Current() uint64    { return 0 }
func UUID(string) string { return "" }

func Watch(context.Context, string, uint64, time.Duration, func([]Event), <-chan chan struct{}) error {
	return ErrUnsupported
}
