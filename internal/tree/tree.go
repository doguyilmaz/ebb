// Package tree holds a scanned file system as a pointer-free arena: parallel
// slices indexed by node ID, so millions of nodes cost the garbage collector
// nothing to scan. A single writer appends nodes; sizes propagate up the
// ancestor chain as they arrive, so partial totals are always consistent.
package tree

import (
	"bytes"
	"cmp"
	"slices"
	"strings"
	"sync"
)

type ID uint32

const None ID = ^ID(0)

type Kind uint8

const (
	KindDir Kind = iota
	KindFile
	KindLink
	KindOther
	KindSmall // virtual node in slices only
)

type Flag uint8

const (
	FlagShared   Flag = 1 << iota // hardlink counted at another location
	FlagClone                     // may share blocks with another file (APFS clone)
	FlagMount                     // mount point, not descended
	FlagDenied                    // directory could not be read
	FlagSyncRoot                  // iCloud sync root
	FlagProject                   // directory contains a project marker file
)

type Node struct {
	ID, Parent    ID
	Name          string
	Kind          Kind
	Flags         Flag
	Alloc, Unique uint64
	Logical       uint64
	Files         uint32
	MaxMtime      int64
	OwnMtime      int64
	SmallCount    uint32
	SmallAlloc    uint64
	Markers       uint16
}

// group is data shared by several directory entries: a hardlinked inode or a
// set of pure APFS clones. Its bytes count once, at first; lca is the lowest
// directory containing every copy seen so far.
type group struct {
	nlink uint32
	seen  uint32
	alloc uint64
	first ID
	lca   ID
}

type Tree struct {
	mu       sync.RWMutex
	rootPath string

	parent, first, next, nameOff []uint32
	nameLen, depth, markers      []uint16
	kind                         []Kind
	flags                        []Flag
	alloc, unique, logical       []uint64
	smallAlloc                   []uint64
	files, smallCount            []uint32
	maxMtime, ownMtime           []int64
	names                        []byte

	links  map[uint64]*group // hardlinked inodes
	clones map[uint64]*group // APFS clone groups by clone ID

	keptMu sync.Mutex
	kept   []uint64 // per node: shared bytes a copy outside keeps alive; nil = stale
}

type Writer struct{ t *Tree }

func New(rootPath string) *Tree {
	t := &Tree{rootPath: rootPath, links: map[uint64]*group{}, clones: map[uint64]*group{}}
	t.add(None, []byte(rootPath), KindDir, 0)
	return t
}

func (t *Tree) add(parent ID, name []byte, k Kind, f Flag) ID {
	id := ID(len(t.parent))
	t.parent = append(t.parent, uint32(parent))
	t.first = append(t.first, uint32(None))
	t.next = append(t.next, uint32(None))
	t.nameOff = append(t.nameOff, uint32(len(t.names)))
	t.nameLen = append(t.nameLen, uint16(len(name)))
	var d uint16
	if parent != None {
		d = t.depth[parent] + 1
	}
	t.depth = append(t.depth, d)
	t.markers = append(t.markers, 0)
	t.names = append(t.names, name...)
	t.kind = append(t.kind, k)
	t.flags = append(t.flags, f)
	t.alloc = append(t.alloc, 0)
	t.unique = append(t.unique, 0)
	t.logical = append(t.logical, 0)
	t.smallAlloc = append(t.smallAlloc, 0)
	t.files = append(t.files, 0)
	t.smallCount = append(t.smallCount, 0)
	t.maxMtime = append(t.maxMtime, 0)
	t.ownMtime = append(t.ownMtime, 0)
	if parent != None {
		t.next[id] = t.first[parent]
		t.first[parent] = uint32(id)
	}
	return id
}

// grow adds bytes, file counts and the newest mtime to id and every ancestor.
func (t *Tree) grow(id ID, alloc, unique, logical uint64, files uint32, mtime int64) {
	for id != None {
		t.alloc[id] += alloc
		t.unique[id] += unique
		t.logical[id] += logical
		t.files[id] += files
		if mtime > t.maxMtime[id] {
			t.maxMtime[id] = mtime
		}
		id = ID(t.parent[id])
	}
}

// Write runs fn as the only writer; readers wait until it returns.
func (t *Tree) Write(fn func(w *Writer)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.keptMu.Lock()
	t.kept = nil
	t.keptMu.Unlock()
	fn(&Writer{t})
}

func (w *Writer) AddDir(parent ID, name []byte, mtime int64, flags Flag) ID {
	t := w.t
	id := t.add(parent, name, KindDir, flags)
	t.ownMtime[id] = mtime
	t.grow(id, 0, 0, 0, 0, mtime)
	return id
}

func (w *Writer) AddFile(parent ID, name []byte, kind Kind, alloc, unique, logical uint64, mtime int64, flags Flag) ID {
	t := w.t
	id := t.add(parent, name, kind, flags)
	t.ownMtime[id] = mtime
	if mtime > t.ownMtime[parent] {
		t.ownMtime[parent] = mtime
	}
	var files uint32
	if kind == KindFile {
		files = 1
	}
	t.grow(id, alloc, unique, logical, files, mtime)
	return id
}

func (w *Writer) AddSmall(dir ID, count uint32, alloc, unique, logical uint64, maxMtime int64) {
	t := w.t
	t.smallCount[dir] += count
	t.smallAlloc[dir] += alloc
	if maxMtime > t.ownMtime[dir] {
		t.ownMtime[dir] = maxMtime
	}
	t.grow(dir, alloc, unique, logical, count, maxMtime)
}

func (w *Writer) SetFlags(id ID, f Flag) { w.t.flags[id] |= f }

// Link registers a file with nlink > 1. It returns true the first time the
// inode is seen; the caller then counts its bytes, otherwise adds the file
// with zero bytes and FlagShared.
func (w *Writer) Link(ino uint64, nlink uint32, alloc uint64, dir ID) bool {
	return w.t.register(w.t.links, ino, nlink, alloc, dir)
}

// Clone registers a file that is a pure APFS clone (refs files share one data
// stream). It returns true the first time the clone ID is seen; later copies
// keep their allocated bytes, as du counts them, but add no unique bytes.
func (w *Writer) Clone(cloneID uint64, refs uint32, alloc uint64, dir ID) bool {
	return w.t.register(w.t.clones, cloneID, refs, alloc, dir)
}

func (t *Tree) register(m map[uint64]*group, key uint64, n uint32, alloc uint64, dir ID) bool {
	if g, ok := m[key]; ok {
		g.seen++
		g.lca = t.lca(g.lca, dir)
		return false
	}
	m[key] = &group{nlink: n, seen: 1, alloc: alloc, first: dir, lca: dir}
	return true
}

func (t *Tree) lca(a, b ID) ID {
	for t.depth[a] > t.depth[b] {
		a = ID(t.parent[a])
	}
	for t.depth[b] > t.depth[a] {
		b = ID(t.parent[b])
	}
	for a != b {
		a, b = ID(t.parent[a]), ID(t.parent[b])
	}
	return a
}

// Finish orders every directory's children by allocated size, largest first.
func (t *Tree) Finish() {
	t.mu.Lock()
	defer t.mu.Unlock()
	var kids []ID
	for p := range t.first {
		if t.first[p] == uint32(None) || t.next[t.first[p]] == uint32(None) {
			continue
		}
		kids = kids[:0]
		for c := ID(t.first[p]); c != None; c = ID(t.next[c]) {
			kids = append(kids, c)
		}
		slices.SortStableFunc(kids, func(a, b ID) int {
			if c := cmp.Compare(t.alloc[b], t.alloc[a]); c != 0 {
				return c
			}
			return bytes.Compare(t.name(a), t.name(b))
		})
		t.first[p] = uint32(kids[0])
		for i := range kids {
			nx := None
			if i+1 < len(kids) {
				nx = kids[i+1]
			}
			t.next[kids[i]] = uint32(nx)
		}
	}
}

func (t *Tree) name(id ID) []byte {
	off := t.nameOff[id]
	return t.names[off : off+uint32(t.nameLen[id])]
}

func (t *Tree) RootPath() string { return t.rootPath }
func (t *Tree) Root() ID         { return 0 }

func (t *Tree) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.parent)
}

func (t *Tree) Node(id ID) Node {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.node(id)
}

func (t *Tree) node(id ID) Node {
	return Node{
		ID: id, Parent: ID(t.parent[id]), Name: string(t.name(id)), Kind: t.kind[id], Flags: t.flags[id],
		Alloc: t.alloc[id], Unique: t.unique[id], Logical: t.logical[id], Files: t.files[id],
		MaxMtime: t.maxMtime[id], OwnMtime: t.ownMtime[id], SmallCount: t.smallCount[id], SmallAlloc: t.smallAlloc[id],
		Markers: t.markers[id],
	}
}

func (t *Tree) Children(id ID) []ID {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.children(id)
}

func (t *Tree) children(id ID) []ID {
	var out []ID
	for c := ID(t.first[id]); c != None; c = ID(t.next[c]) {
		out = append(out, c)
	}
	return out
}

func (t *Tree) Path(id ID) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.path(id)
}

func (t *Tree) path(id ID) string {
	if id == 0 {
		return t.rootPath
	}
	var parts [][]byte
	for ; id != 0; id = ID(t.parent[id]) {
		parts = append(parts, t.name(id))
	}
	var b strings.Builder
	b.WriteString(strings.TrimSuffix(t.rootPath, "/"))
	for i := len(parts) - 1; i >= 0; i-- {
		b.WriteByte('/')
		b.Write(parts[i])
	}
	return b.String()
}

// Lookup finds the node at an absolute path under RootPath.
func (t *Tree) Lookup(path string) (ID, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lookup(path)
}

func (t *Tree) lookup(path string) (ID, bool) {
	root := strings.TrimSuffix(t.rootPath, "/")
	if path == t.rootPath || path == root {
		return 0, true
	}
	rest, ok := strings.CutPrefix(path, root+"/")
	if !ok {
		return None, false
	}
	id := ID(0)
	for part := range strings.SplitSeq(strings.Trim(rest, "/"), "/") {
		found := None
		for c := ID(t.first[id]); c != None; c = ID(t.next[c]) {
			if string(t.name(c)) == part {
				found = c
				break
			}
		}
		if found == None {
			return None, false
		}
		id = found
	}
	return id, true
}

// IsAncestor reports whether a is b or lies above it.
func (t *Tree) IsAncestor(a, b ID) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.isAncestor(a, b)
}

func (t *Tree) isAncestor(a, b ID) bool {
	for ; b != None; b = ID(t.parent[b]) {
		if a == b {
			return true
		}
	}
	return false
}

// Reclaimable is what deleting id would free: its unique bytes, minus shared
// data (hardlinks, clone groups) counted inside id that another copy outside
// it keeps alive.
func (t *Tree) Reclaimable(id ID) uint64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	t.keptMu.Lock()
	if t.kept == nil {
		t.kept = t.computeKept()
	}
	kept := t.kept[id]
	t.keptMu.Unlock()
	return t.unique[id] - min(kept, t.unique[id])
}

// computeKept charges each group's bytes to the directories between where
// they were counted and the lowest folder holding every copy; when copies
// live outside the scan, all the way to the root.
func (t *Tree) computeKept() []uint64 {
	kept := make([]uint64, len(t.parent))
	for _, m := range []map[uint64]*group{t.links, t.clones} {
		for _, g := range m {
			stop := None
			if g.seen >= g.nlink {
				stop = g.lca
			}
			for x := g.first; x != stop; x = ID(t.parent[x]) {
				kept[x] += g.alloc
			}
		}
	}
	return kept
}

// Linked and Cloned report whether a hardlinked inode or clone ID was seen.
func (w *Writer) Linked(ino uint64) bool     { _, ok := w.t.links[ino]; return ok }
func (w *Writer) Cloned(cloneID uint64) bool { _, ok := w.t.clones[cloneID]; return ok }
