package daemon

import (
	"testing"
	"time"
)

func TestNextWork(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	base := Conditions{Now: now, OnAC: true, Idle: 20 * time.Minute, Changed: 300, Known: true,
		LastScan: now.Add(-time.Hour), LastFull: now.Add(-24 * time.Hour)}
	cases := []struct {
		name string
		edit func(*Conditions)
		want Work
	}{
		{"changes waiting, on power, idle", func(*Conditions) {}, Update},
		{"on battery", func(c *Conditions) { c.OnAC = false }, Nothing},
		{"someone is working", func(c *Conditions) { c.Idle = 5 * time.Minute }, Nothing},
		{"scanned a few minutes ago", func(c *Conditions) { c.LastScan = now.Add(-10 * time.Minute) }, Nothing},
		{"nothing changed", func(c *Conditions) { c.Changed = 0 }, Nothing},
		{"changes unknown", func(c *Conditions) { c.Known = false }, Full},
		{"a week since the last full scan", func(c *Conditions) { c.LastFull = now.Add(-8 * 24 * time.Hour) }, Full},
		{"never scanned", func(c *Conditions) { c.LastScan, c.LastFull = time.Time{}, time.Time{} }, Nothing},
	}
	for _, tc := range cases {
		c := base
		tc.edit(&c)
		if got := NextWork(c); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestLowSpaceAlertsOncePerCrossing(t *testing.T) {
	var a Alert
	const gb = 1e9
	steps := []struct {
		free   float64
		notify bool
	}{{20 * gb, false}, {9 * gb, true}, {8 * gb, false}, {10.5 * gb, false}, {13 * gb, false}, {7 * gb, true}}
	for i, s := range steps {
		if got := a.Check(uint64(s.free), 10*gb); got != s.notify {
			t.Fatalf("step %d (%v GB free): notify %v, want %v", i, s.free/gb, got, s.notify)
		}
	}
}
