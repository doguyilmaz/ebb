package server

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/app"
)

func TestExternalReloadNotifiesOpenViews(t *testing.T) {
	c, root := scanned(t)
	events := c.s.hub.subscribe()
	defer c.s.hub.unsubscribe(events)

	terminal, err := app.New(c.app.Env)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "a", "new.bin")
	if err := os.WriteFile(path, make([]byte, 3<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := terminal.Scan(context.Background(), app.ScanOptions{Root: root, Full: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.app.Last()
	if err != nil || got.Meta.ID != res.Meta.ID {
		t.Fatalf("server did not reload terminal scan: %v", err)
	}

	want := map[string]bool{
		api.EventScanDone:        false,
		api.EventFindingsChanged: false,
		api.EventHistoryChanged:  false,
	}
	for range want {
		select {
		case ev := <-events:
			if _, ok := want[ev.name]; ok {
				want[ev.name] = true
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("reload events incomplete: %#v", want)
		}
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("missing reload event %q", name)
		}
	}
}

func TestExternalDuplicatesReloadNotifiesOpenViews(t *testing.T) {
	c, root := scanned(t)
	if status, _ := c.get("/api/v1/duplicates", nil); status != 200 {
		t.Fatalf("initial duplicates request: %d", status)
	}
	events := c.s.hub.subscribe()
	defer c.s.hub.unsubscribe(events)

	terminal, err := app.New(c.app.Env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.FindDuplicates(context.Background(), root, 0, nil); err != nil {
		t.Fatal(err)
	}
	if status, _ := c.get("/api/v1/status", nil); status != 200 {
		t.Fatalf("status heartbeat: %d", status)
	}
	select {
	case ev := <-events:
		if ev.name != api.EventDupesDone {
			t.Fatalf("reload event %q, want %q", ev.name, api.EventDupesDone)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("missing duplicates reload event")
	}
}

func TestStatusDoesNotLoadUnopenedDuplicates(t *testing.T) {
	c, root := scanned(t)
	var notifications atomic.Int32
	c.app.SetExternalDuplicatesReloadHandler(func() { notifications.Add(1) })
	terminal, err := app.New(c.app.Env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.FindDuplicates(context.Background(), root, 0, nil); err != nil {
		t.Fatal(err)
	}
	events := c.s.hub.subscribe()
	defer c.s.hub.unsubscribe(events)
	if status, _ := c.get("/api/v1/status", nil); status != 200 {
		t.Fatalf("first status heartbeat: %d", status)
	}
	if _, err := terminal.FindDuplicates(context.Background(), root, 0, nil); err != nil {
		t.Fatal(err)
	}
	if status, _ := c.get("/api/v1/status", nil); status != 200 {
		t.Fatalf("second status heartbeat: %d", status)
	}
	select {
	case ev := <-events:
		t.Fatalf("unopened duplicates emitted %q", ev.name)
	case <-time.After(100 * time.Millisecond):
	}
	if got := notifications.Load(); got != 0 {
		t.Fatalf("unopened duplicates were loaded %d times", got)
	}
}
