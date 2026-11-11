package history

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// explainingPairs is the first Explaining, which compared every pair of
// changes (only the prefix is built once per change here, to keep the test
// quick). It is the reference: the fast one must return exactly this.
func explainingPairs(changes []Change) []Change {
	var out []Change
	for _, c := range changes {
		below := strings.TrimSuffix(c.Path, "/") + "/"
		explained := slices.ContainsFunc(changes, func(o Change) bool {
			return strings.HasPrefix(o.Path, below) && (o.Delta > 0) == (c.Delta > 0) && abs(o.Delta)*5 >= abs(c.Delta)*4
		})
		if !explained {
			out = append(out, c)
		}
	}
	return out
}

func ch(path string, delta int64) Change {
	before := uint64(1 << 40)
	return Change{Path: path, Before: before, After: uint64(int64(before) + delta), Delta: delta}
}

func paths(cs []Change) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Path)
	}
	return out
}

// sameAsPairs runs both versions and fails on any difference, nil included.
func sameAsPairs(t *testing.T, changes []Change) []Change {
	t.Helper()
	got, want := Explaining(changes), explainingPairs(changes)
	if (got == nil) != (want == nil) {
		t.Fatalf("got nil=%v, want nil=%v", got == nil, want == nil)
	}
	if !slices.Equal(got, want) {
		i := 0
		for i < min(len(got), len(want)) && got[i] == want[i] {
			i++
		}
		t.Fatalf("got %d changes, want %d; first difference at %d", len(got), len(want), i)
	}
	return got
}

func TestExplainingEdgeCases(t *testing.T) {
	for name, tc := range map[string]struct {
		in   []Change
		want []string
	}{
		"nothing":         {nil, nil},
		"empty":           {[]Change{}, nil},
		"single":          {[]Change{ch("/a", 5)}, []string{"/a"}},
		"the root slash":  {[]Change{ch("/", 100), ch("/a", 50), ch("/b", 30)}, []string{"/a", "/b"}},
		"the data volume": {[]Change{ch("/System/Volumes/Data", 100), ch("/System/Volumes/Data/Users", 90), ch("/System/Volumes/Data/Users/me", 50)}, []string{"/System/Volumes/Data/Users", "/System/Volumes/Data/Users/me"}},
		// A trailing slash makes a path its own descendant, as it always did.
		"trailing slash": {[]Change{ch("/a/", 100), ch("/a/b", 10), ch("/c/", -3), ch("/d", 100), ch("/d/", 90)}, []string{"/a/b"}},
		"siblings that share a prefix": {[]Change{ch("/a/b", 100), ch("/a/bc", 100), ch("/a/b-c", 100), ch("/a/b.c", 100), ch("/a/b/c", 10)},
			[]string{"/a/b", "/a/bc", "/a/b-c", "/a/b.c", "/a/b/c"}},
		// "/a/b-c" sorts between "/a/b" and "/a/b/c".
		"a sibling between a folder and its children": {[]Change{ch("/a/b", 100), ch("/a/b-c", 1), ch("/a/b/c", 80)}, []string{"/a/b-c", "/a/b/c"}},
		"opposite signs": {[]Change{ch("/a", 100), ch("/a/b", -100), ch("/c", -100), ch("/c/d", 100)}, []string{"/a", "/a/b", "/c", "/c/d"}},
		"zero deltas count as shrinking": {[]Change{ch("/a", 0), ch("/a/b", 0), ch("/c", 0), ch("/c/d", -5), ch("/e", -5), ch("/e/f", 0), ch("/g", 0), ch("/g/h", 5)},
			[]string{"/a/b", "/c/d", "/e", "/e/f", "/g", "/g/h"}},
		"the 80 % boundary": {[]Change{ch("/a", 100), ch("/a/b", 80), ch("/c", 100), ch("/c/d", 79), ch("/e", -100), ch("/e/f", -80), ch("/g", -100), ch("/g/h", -79),
			ch("/i", 100), ch("/i/j", 10), ch("/i/j/k", 80)},
			[]string{"/a/b", "/c", "/c/d", "/e/f", "/g", "/g/h", "/i/j/k"}},
		"duplicate-looking paths": {[]Change{ch("/a/b", 100), ch("/a/b", 100), ch("/a//b", 90), ch("/a", 100), ch("/a/", 5)}, []string{"/a/b", "/a/b", "/a//b"}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := paths(sameAsPairs(t, tc.in)); !slices.Equal(got, tc.want) {
				t.Fatalf("kept %q, want %q", got, tc.want)
			}
		})
	}
}

// randomChanges builds a folder tree of n folders under root and the changes
// Diff would report for it: a folder changes by what its subfolders changed
// plus its own files, in Diff's order. Names repeat and share prefixes, so
// some paths come twice and siblings sort between a folder and its children.
func randomChanges(r *rand.Rand, root string, n int, size int64) []Change {
	names := []string{"a", "b", "ab", "a-b", "a.b", "a b", "Library", "Caches", "node_modules", "v1", "v10", "x"}
	path, parent := []string{root}, []int{-1}
	for len(path) < n {
		// Mostly below a recent folder, so the tree gets deep like a home folder.
		p := len(path) - 1 - r.IntN(min(len(path), 30))
		if r.IntN(5) == 0 {
			p = r.IntN(len(path))
		}
		path = append(path, strings.TrimSuffix(path[p], "/")+"/"+names[r.IntN(len(names))])
		parent = append(parent, p)
	}
	delta := make([]int64, n)
	for i := range delta {
		if r.IntN(8) > 0 {
			delta[i] = r.Int64N(2*size+1) - size
		}
	}
	for i := n - 1; i > 0; i-- {
		delta[parent[i]] += delta[i]
	}
	out := make([]Change, n)
	for i := range out {
		out[i] = ch(path[i], delta[i])
	}
	slices.SortFunc(out, func(x, y Change) int {
		if c := cmp.Compare(abs(y.Delta), abs(x.Delta)); c != 0 {
			return c
		}
		return cmp.Compare(x.Path, y.Path)
	})
	return out
}

func TestExplainingMatchesPairs(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for _, tc := range []struct {
		root string
		size int64
	}{
		{"/", 1 << 30},
		{"/System/Volumes/Data", 1 << 30},
		{"/Users/me", 1 << 30},
		// Small sizes hit the 80 % boundary and zero often.
		{"/", 5},
		{"/Users/me", 5},
	} {
		for _, n := range []int{2, 10, 300, 3000} {
			changes := randomChanges(r, tc.root, n, tc.size)
			kept := sameAsPairs(t, changes)
			if n == 3000 && (len(kept) == 0 || len(kept) == n) {
				t.Fatalf("%s: kept %d of %d, the data explains nothing", tc.root, len(kept), n)
			}
			// The order of the input is kept, whatever it is.
			r.Shuffle(len(changes), func(i, j int) { changes[i], changes[j] = changes[j], changes[i] })
			sameAsPairs(t, changes)
		}
	}
}

func BenchmarkExplaining(b *testing.B) {
	changes := randomChanges(rand.New(rand.NewPCG(3, 4)), "/System/Volumes/Data", 10_000, 1<<30)
	for b.Loop() {
		Explaining(changes)
	}
}
