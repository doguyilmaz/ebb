package tree

// Edits for incremental rescans: a changed folder is cleared of its own files
// and listed again, gone folders are removed, and the arena is compacted so
// nothing removed is found again.

// ClearOwn takes a folder's own files out of the tree (stored files and the
// small-file total) so a fresh listing can be added back; subfolders stay.
// It returns the removed file nodes.
func (w *Writer) ClearOwn(dir ID) []ID {
	t := w.t
	// One pass: unlink every file child and keep the subfolders in order;
	// removing files one by one would rescan the sibling list each time.
	var removed []ID
	last := None
	for c := ID(t.first[dir]); c != None; {
		next := ID(t.next[c])
		if t.kind[c] == KindDir {
			if last == None {
				t.first[dir] = uint32(c)
			} else {
				t.next[last] = uint32(c)
			}
			last = c
		} else {
			removed = append(removed, c)
		}
		c = next
	}
	if last == None {
		t.first[dir] = uint32(None)
	} else {
		t.next[last] = uint32(None)
	}
	// What is left beyond the subfolders is the folder's own files: stored
	// ones just unlinked plus the small-file total.
	alloc, unique, logical, files := t.alloc[dir], t.unique[dir], t.logical[dir], t.files[dir]
	for c := ID(t.first[dir]); c != None; c = ID(t.next[c]) {
		alloc -= min(t.alloc[c], alloc)
		unique -= min(t.unique[c], unique)
		logical -= min(t.logical[c], logical)
		files -= min(t.files[c], files)
	}
	for p := dir; p != None; p = ID(t.parent[p]) {
		t.alloc[p] -= min(alloc, t.alloc[p])
		t.unique[p] -= min(unique, t.unique[p])
		t.logical[p] -= min(logical, t.logical[p])
		t.files[p] -= min(files, t.files[p])
	}
	t.smallCount[dir], t.smallAlloc[dir], t.markers[dir] = 0, 0, 0
	t.flags[dir] &^= FlagProject
	return removed
}

// Children and Node read the tree while writing, where the locking readers would deadlock.
func (w *Writer) Children(id ID) []ID { return w.t.children(id) }
func (w *Writer) Node(id ID) Node     { return w.t.node(id) }

func (w *Writer) groups(link bool) map[uint64]*group {
	if link {
		return w.t.links
	}
	return w.t.clones
}

// SharedElsewhere reports whether a hardlinked inode (link) or clone ID is
// already counted at a node that own does not claim: then this copy adds no
// bytes. A copy counted in the folder being listed again is that folder's own.
func (w *Writer) SharedElsewhere(link bool, key uint64, own func(ID) bool) bool {
	g, ok := w.groups(link)[key]
	return ok && !own(g.first)
}

// Rejoin registers a copy met while listing a folder again. The counted copy
// moves to its new node; a copy already known elsewhere is not counted twice,
// so seen never grows past the copies there are (an unknown new copy keeps
// reclaimable conservative).
func (w *Writer) Rejoin(link bool, key uint64, n uint32, alloc uint64, at ID, own func(ID) bool) {
	m := w.groups(link)
	if g, ok := m[key]; ok {
		if own(g.first) {
			g.first = at
			g.lca = w.t.lca(g.lca, at)
		}
		return
	}
	m[key] = &group{nlink: n, seen: 1, alloc: alloc, first: at, lca: at}
}

// Compact rebuilds the arena with only the nodes still reachable from the
// root, parents before children as on disk. Shared-data groups follow their
// nodes; a group whose counted copy is gone is dropped.
func (t *Tree) Compact() {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(t.parent)
	remap := make([]uint32, n)
	for i := range remap {
		remap[i] = uint32(None)
	}
	order := make([]ID, 0, n)
	order = append(order, 0)
	remap[0] = 0
	for i := 0; i < len(order); i++ {
		for c := ID(t.first[order[i]]); c != None; c = ID(t.next[c]) {
			remap[c] = uint32(len(order))
			order = append(order, c)
		}
	}
	if len(order) == n {
		return
	}
	nt := &Tree{links: map[uint64]*group{}, clones: map[uint64]*group{}}
	m := len(order)
	nt.parent, nt.first, nt.next, nt.nameOff = make([]uint32, m), make([]uint32, m), make([]uint32, m), make([]uint32, m)
	nt.nameLen, nt.depth, nt.markers = make([]uint16, m), make([]uint16, m), make([]uint16, m)
	nt.kind, nt.flags = make([]Kind, m), make([]Flag, m)
	nt.alloc, nt.unique, nt.logical, nt.smallAlloc = make([]uint64, m), make([]uint64, m), make([]uint64, m), make([]uint64, m)
	nt.files, nt.smallCount = make([]uint32, m), make([]uint32, m)
	nt.maxMtime, nt.ownMtime = make([]int64, m), make([]int64, m)
	at := func(v uint32) uint32 {
		if v == uint32(None) {
			return v
		}
		return remap[v]
	}
	for i, old := range order {
		nt.parent[i], nt.first[i], nt.next[i] = at(t.parent[old]), at(t.first[old]), at(t.next[old])
		nt.nameOff[i] = uint32(len(nt.names))
		nt.nameLen[i] = t.nameLen[old]
		nt.names = append(nt.names, t.name(old)...)
		nt.depth[i], nt.markers[i], nt.kind[i], nt.flags[i] = t.depth[old], t.markers[old], t.kind[old], t.flags[old]
		nt.alloc[i], nt.unique[i], nt.logical[i], nt.smallAlloc[i] = t.alloc[old], t.unique[old], t.logical[old], t.smallAlloc[old]
		nt.files[i], nt.smallCount[i], nt.maxMtime[i], nt.ownMtime[i] = t.files[old], t.smallCount[old], t.maxMtime[old], t.ownMtime[old]
	}
	for _, pair := range [][2]map[uint64]*group{{t.links, nt.links}, {t.clones, nt.clones}} {
		src, dst := pair[0], pair[1]
		for key, g := range src {
			first := remap[g.first]
			if first == uint32(None) {
				continue
			}
			lca := g.lca
			for lca != None && remap[lca] == uint32(None) {
				lca = ID(t.parent[lca])
			}
			if lca == None {
				lca = ID(0)
			}
			dst[key] = &group{nlink: g.nlink, seen: g.seen, alloc: g.alloc, first: ID(first), lca: ID(remap[lca])}
		}
	}
	t.parent, t.first, t.next, t.nameOff, t.nameLen, t.depth, t.markers = nt.parent, nt.first, nt.next, nt.nameOff, nt.nameLen, nt.depth, nt.markers
	t.kind, t.flags, t.alloc, t.unique, t.logical, t.smallAlloc = nt.kind, nt.flags, nt.alloc, nt.unique, nt.logical, nt.smallAlloc
	t.files, t.smallCount, t.maxMtime, t.ownMtime, t.names = nt.files, nt.smallCount, nt.maxMtime, nt.ownMtime, nt.names
	t.links, t.clones = nt.links, nt.clones
	t.keptMu.Lock()
	t.kept = nil
	t.keptMu.Unlock()
}
