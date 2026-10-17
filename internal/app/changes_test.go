package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/doguyilmaz/ebb/internal/fsevents"
)

func TestChangesCollectsFoldersUnderTheRoot(t *testing.T) {
	c := newChanges(func(p string) (string, bool) {
		if strings.HasPrefix(p, "/Users") {
			return "/System/Volumes/Data" + p, true
		}
		return "", false
	}, "/System/Volumes/Data/Users/me/.local/state/ebb")
	c.add([]fsevents.Event{
		{Path: "/Users/me/Downloads", ID: 10},
		{Path: "/Users/me/Downloads", ID: 11},
		{Path: "/Applications-elsewhere", ID: 12},
		{Path: "/Users/me/node_modules", Flags: fsevents.MustScanSubDirs, ID: 13},
		{Path: "/Users/me/.local/state/ebb", ID: 14},
		{Path: "", Flags: fsevents.HistoryDone, ID: 15},
	})
	if n, lost, ready := c.summary(); n != 2 || lost || !ready {
		t.Fatalf("summary %d %v %v", n, lost, ready)
	}
	s := c.take()
	if fmt.Sprint(s.dirs, s.subtrees, s.last) != "[/System/Volumes/Data/Users/me/Downloads] [/System/Volumes/Data/Users/me/node_modules] 15" {
		t.Fatalf("took %v %v %d", s.dirs, s.subtrees, s.last)
	}
	if n, _, _ := c.summary(); n != 0 {
		t.Fatal("taking empties the set")
	}
	c.add([]fsevents.Event{{Path: "/Users/me", Flags: fsevents.KernelDropped, ID: 16}})
	if _, lost, _ := c.summary(); !lost {
		t.Fatal("dropped events mean the next scan is complete")
	}
}
