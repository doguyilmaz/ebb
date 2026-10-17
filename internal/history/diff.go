package history

import (
	"cmp"
	"slices"
	"strings"
)

type Change struct {
	Path   string `json:"path"`
	Before uint64 `json:"before"`
	After  uint64 `json:"after"`
	Delta  int64  `json:"delta"`
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// Diff compares two snapshots by path; a path missing from one side counts
// as zero there. Only changes of at least minDelta bytes are returned,
// largest first.
func Diff(a, b *Snapshot, minDelta uint64) []Change {
	before := map[string]uint64{}
	for _, d := range a.Dirs {
		before[d.Path] = d.Alloc
	}
	var out []Change
	seen := map[string]bool{}
	add := func(p string, x, y uint64) {
		if d := int64(y) - int64(x); uint64(abs(d)) >= minDelta {
			out = append(out, Change{Path: p, Before: x, After: y, Delta: d})
		}
	}
	for _, d := range b.Dirs {
		seen[d.Path] = true
		add(d.Path, before[d.Path], d.Alloc)
	}
	for _, d := range a.Dirs {
		if !seen[d.Path] {
			add(d.Path, d.Alloc, 0)
		}
	}
	slices.SortFunc(out, func(x, y Change) int {
		if c := cmp.Compare(abs(y.Delta), abs(x.Delta)); c != 0 {
			return c
		}
		return cmp.Compare(x.Path, y.Path)
	})
	return out
}

// Explaining drops a change when a folder below it accounts for at least 80 %
// of it, leaving the deepest folders that actually explain the growth.
func Explaining(changes []Change) []Change {
	var out []Change
	for _, c := range changes {
		explained := slices.ContainsFunc(changes, func(o Change) bool {
			return strings.HasPrefix(o.Path, strings.TrimSuffix(c.Path, "/")+"/") && (o.Delta > 0) == (c.Delta > 0) && abs(o.Delta)*5 >= abs(c.Delta)*4
		})
		if !explained {
			out = append(out, c)
		}
	}
	return out
}
