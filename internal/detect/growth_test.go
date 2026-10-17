package detect

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/history"
)

func TestGrowthFinding(t *testing.T) {
	tr := home(t, "a/b/x=3500", "a/other=500", "c/y=2000")
	c := ctxFor(t, tr)
	c.History = history.Open(t.TempDir())
	prev := &history.Snapshot{Meta: history.Meta{Time: c.Now.Add(-7 * 24 * time.Hour), Root: "/h"}, Dirs: []history.Dir{
		{Path: "/h", Alloc: 3000 << 20}, {Path: "/h/a", Alloc: 1000 << 20}, {Path: "/h/a/b", Alloc: 500 << 20}, {Path: "/h/c", Alloc: 1950 << 20}}}
	if err := c.History.Save(prev); err != nil {
		t.Fatal(err)
	}
	fs, err := growth{}.Detect(context.Background(), c)
	if err != nil || len(fs) != 1 {
		t.Fatalf("%+v %v", fs, err)
	}
	f := fs[0]
	if len(f.Targets) != 1 || f.Targets[0].Path != "/h/a/b" || len(f.Actions) != 0 {
		t.Fatalf("only the folder that explains the growth, informational: %+v", f)
	}
	if !strings.Contains(f.Title, "2026-10-10") || !strings.Contains(f.Evidence[0], "+3.1 GB") {
		t.Fatalf("%+v", f)
	}
}

func TestGrowthWithoutHistory(t *testing.T) {
	if fs, err := (growth{}).Detect(context.Background(), ctxFor(t, home(t, "a/x=1"))); err != nil || len(fs) != 0 {
		t.Fatalf("%v %v", fs, err)
	}
}
