package history

import (
	"cmp"
	"slices"
	"sort"
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
// of it, leaving the deepest folders that actually explain the growth. Only a
// change the same way explains: growth by growth, shrinking by shrinking (no
// change at all counts as shrinking).
func Explaining(changes []Change) []Change {
	// Everything below P is a path starting with "P/", and those paths form one
	// run in byte order. With the changes sorted by path, each change needs two
	// binary searches and one lookup of the largest change in its run, where
	// comparing every pair took seconds for a month of history.
	n := len(changes)
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(x, y int) int { return strings.Compare(changes[x].Path, changes[y].Path) })
	// Max trees over the sorted changes: leaf n+i holds the size of the change
	// at order[i] if it went that way and -1 otherwise.
	grew, shrank := make([]int64, 2*n), make([]int64, 2*n)
	for i, j := range order {
		grew[n+i], shrank[n+i] = -1, -1
		if d := changes[j].Delta; d > 0 {
			grew[n+i] = d
		} else {
			shrank[n+i] = -d
		}
	}
	for i := n - 1; i > 0; i-- {
		grew[i] = max(grew[2*i], grew[2*i+1])
		shrank[i] = max(shrank[2*i], shrank[2*i+1])
	}
	var out []Change
	for _, c := range changes {
		below := strings.TrimSuffix(c.Path, "/") + "/"
		lo := sort.Search(n, func(i int) bool { return changes[order[i]].Path >= below })
		hi := lo + sort.Search(n-lo, func(i int) bool { return !strings.HasPrefix(changes[order[lo+i]].Path, below) })
		same := shrank
		if c.Delta > 0 {
			same = grew
		}
		// An empty run or one without a change that way yields -1, which never explains.
		if largest(same, n, lo, hi)*5 < abs(c.Delta)*4 {
			out = append(out, c)
		}
	}
	return out
}

// largest returns the biggest leaf in [lo, hi) of a max tree with n leaves,
// or -1 for an empty range.
func largest(t []int64, n, lo, hi int) int64 {
	m := int64(-1)
	for lo, hi = lo+n, hi+n; lo < hi; lo, hi = lo/2, hi/2 {
		if lo&1 == 1 {
			m = max(m, t[lo])
			lo++
		}
		if hi&1 == 1 {
			hi--
			m = max(m, t[hi])
		}
	}
	return m
}
