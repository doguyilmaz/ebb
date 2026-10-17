package history

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/tree"
)

func snap(at time.Time, dirs ...Dir) *Snapshot {
	return &Snapshot{Meta: Meta{Time: at, Root: "/r", Used: 100}, Dirs: dirs}
}

func TestSaveListLoad(t *testing.T) {
	s := Open(t.TempDir())
	t1 := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	if err := s.Save(snap(t1, Dir{Path: "/r/a", Alloc: 5})); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(snap(t1.Add(time.Hour), Dir{Path: "/r/a", Alloc: 7})); err != nil {
		t.Fatal(err)
	}
	metas, err := s.List()
	if err != nil || len(metas) != 2 || !metas[0].Time.After(metas[1].Time) {
		t.Fatalf("list %+v %v", metas, err)
	}
	if metas[0].ID != "20261010T100000Z" {
		t.Fatalf("id %q", metas[0].ID)
	}
	got, err := s.Load(metas[1].ID)
	if err != nil || len(got.Dirs) != 1 || got.Dirs[0].Alloc != 5 {
		t.Fatalf("load %+v %v", got, err)
	}
	prev, err := s.Previous("/r", t1.Add(30*time.Minute))
	if err != nil || prev.Meta.ID != metas[1].ID {
		t.Fatalf("previous %+v %v", prev, err)
	}
	if _, err := s.Previous("/other", t1.Add(2*time.Hour)); err == nil {
		t.Fatal("other roots never count as previous")
	}
}

func TestSameSecondDoesNotOverwrite(t *testing.T) {
	s := Open(t.TempDir())
	at := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	s.Save(snap(at))
	s.Save(snap(at.Add(100 * time.Millisecond)))
	if metas, _ := s.List(); len(metas) != 2 || metas[0].ID == metas[1].ID {
		t.Fatalf("%+v", metas)
	}
}

func TestRetentionKeepsWeekly(t *testing.T) {
	s := Open(t.TempDir())
	start := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	var times []time.Time
	for i := range 100 {
		at := start.Add(time.Duration(i) * 48 * time.Hour)
		times = append(times, at)
		if err := s.Save(snap(at)); err != nil {
			t.Fatal(err)
		}
	}
	metas, _ := s.List()
	for _, at := range times[40:] {
		if !slices.ContainsFunc(metas, func(m Meta) bool { return m.Time.Equal(at) }) {
			t.Fatalf("newest 60 must all be kept; %v missing", at)
		}
	}
	weeks := map[string]int{}
	for _, m := range metas[60:] {
		y, w := m.Time.ISOWeek()
		weeks[fmt.Sprint(y, w)]++
	}
	for wk, n := range weeks {
		if n != 1 {
			t.Fatalf("week %s kept %d snapshots", wk, n)
		}
	}
	if len(metas) <= 60 || len(metas) >= 100 {
		t.Fatalf("kept %d", len(metas))
	}
}

func TestDiffAndExplaining(t *testing.T) {
	a := snap(time.Now(), Dir{Path: "/r/a", Alloc: 10 << 30}, Dir{Path: "/r/a/b", Alloc: 2 << 30}, Dir{Path: "/r/c", Alloc: 5 << 30})
	b := snap(time.Now(), Dir{Path: "/r/a", Alloc: 20 << 30}, Dir{Path: "/r/a/b", Alloc: 11 << 30}, Dir{Path: "/r/new", Alloc: 3 << 30})
	changes := Diff(a, b, 1<<30)
	if len(changes) != 4 || changes[0].Path != "/r/a" || changes[0].Delta != 10<<30 {
		t.Fatalf("diff %+v", changes)
	}
	exp := Explaining(changes)
	var got []string
	for _, c := range exp {
		got = append(got, c.Path)
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"/r/a/b", "/r/c", "/r/new"}) {
		t.Fatalf("explaining %v", got)
	}
}

func TestFromTree(t *testing.T) {
	tr := tree.New("/r")
	tr.Write(func(w *tree.Writer) {
		a := w.AddDir(tr.Root(), []byte("a"), 1, 0)
		w.AddFile(a, []byte("f"), tree.KindFile, 20<<20, 20<<20, 20<<20, 5, 0)
		s := w.AddDir(tr.Root(), []byte("small"), 1, 0)
		w.AddSmall(s, 1, 1000, 1000, 1000, 1)
	})
	dirs := FromTree(tr, 10<<20, func(p string) string { return "D" + p })
	if len(dirs) != 2 || dirs[0].Path != "D/r" || dirs[1].Path != "D/r/a" || dirs[1].MaxMtime != 5 {
		t.Fatalf("%+v", dirs)
	}
}
