//go:build darwin

package fsevents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWatchReplaysHistory(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	since := Current()
	if since == 0 || UUID(dir) == "" {
		t.Fatalf("current %d, uuid %q", since, UUID(dir))
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var mu sync.Mutex
	var sawSub, done bool
	go func() {
		_ = Watch(ctx, dir, since, 100*time.Millisecond, func(evs []Event) {
			mu.Lock()
			defer mu.Unlock()
			for _, e := range evs {
				if e.Flags&HistoryDone != 0 {
					done = true
				}
				if strings.HasSuffix(e.Path, "/sub") && !strings.HasSuffix(e.Path, "/") {
					sawSub = true
				}
			}
			if done && sawSub {
				cancel()
			}
		}, nil)
	}()
	<-ctx.Done()
	mu.Lock()
	defer mu.Unlock()
	if !done || !sawSub {
		t.Fatalf("history done %v, saw the new folder %v", done, sawSub)
	}
}

func TestFlushDeliversBeforeLatency(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	sub := filepath.Join(dir, "a")
	saw := false
	seen := func() bool { mu.Lock(); defer mu.Unlock(); return saw }
	flush := make(chan chan struct{})
	go func() {
		_ = Watch(ctx, dir, 0, 10*time.Second, func(evs []Event) {
			mu.Lock()
			defer mu.Unlock()
			for _, e := range evs {
				saw = saw || e.Path == sub
			}
		}, flush)
	}()
	time.Sleep(300 * time.Millisecond)
	// The first event goes out at once; the next one waits for the latency.
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(sub, "f"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if seen() {
		t.Skip("the change arrived before the latency, so a flush proves nothing here")
	}
	done := make(chan struct{})
	flush <- done
	<-done
	if !seen() {
		t.Fatal("a flush did not deliver the change ahead of the stream's latency")
	}
}
