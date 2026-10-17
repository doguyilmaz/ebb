package tree

// Ages splits the bytes under id by how long before now they last changed:
// bucket i holds ages below cuts[i] (ascending, seconds), the last bucket the
// rest. Stored files use their own mtime; small files folded into a folder
// use that folder's newest mtime, so they lean recent.
func (t *Tree) Ages(id ID, now int64, cuts []int64) []uint64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]uint64, len(cuts)+1)
	put := func(bytes uint64, mtime int64) {
		if bytes == 0 {
			return
		}
		age, i := now-mtime, 0
		for i < len(cuts) && age >= cuts[i] {
			i++
		}
		out[i] += bytes
	}
	if t.kind[id] != KindDir {
		put(t.alloc[id], t.ownMtime[id])
		return out
	}
	stack := []ID{id}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		put(t.smallAlloc[cur], t.ownMtime[cur])
		for c := ID(t.first[cur]); c != None; c = ID(t.next[c]) {
			if t.kind[c] == KindDir {
				stack = append(stack, c)
			} else {
				put(t.alloc[c], t.ownMtime[c])
			}
		}
	}
	return out
}
