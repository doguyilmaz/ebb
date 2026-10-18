package tree

import (
	"bytes"
	"container/heap"
	"path"
	"strings"
)

// Top returns the n largest stored files under id, largest first.
func (t *Tree) Top(id ID, n int) []ID { return t.TopFunc(id, n, nil) }

// TopFunc is Top over the files keep accepts (nil keeps all), given their
// allocated size and last change.
func (t *Tree) TopFunc(id ID, n int, keep func(alloc uint64, mtime int64) bool) []ID {
	t.mu.RLock()
	defer t.mu.RUnlock()
	h := &idHeap{t: t}
	stack := []ID{id}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for c := ID(t.first[cur]); c != None; c = ID(t.next[c]) {
			switch t.kind[c] {
			case KindDir:
				stack = append(stack, c)
			case KindFile:
				if keep != nil && !keep(t.alloc[c], t.ownMtime[c]) {
					continue
				}
				if h.Len() < n {
					heap.Push(h, c)
				} else if n > 0 && t.alloc[c] > t.alloc[h.ids[0]] {
					h.ids[0] = c
					heap.Fix(h, 0)
				}
			}
		}
	}
	out := make([]ID, h.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(h).(ID)
	}
	return out
}

type idHeap struct {
	t   *Tree
	ids []ID
}

func (h *idHeap) Len() int           { return len(h.ids) }
func (h *idHeap) Less(i, j int) bool { return h.t.alloc[h.ids[i]] < h.t.alloc[h.ids[j]] }
func (h *idHeap) Swap(i, j int)      { h.ids[i], h.ids[j] = h.ids[j], h.ids[i] }
func (h *idHeap) Push(x any)         { h.ids = append(h.ids, x.(ID)) }
func (h *idHeap) Pop() any {
	x := h.ids[len(h.ids)-1]
	h.ids = h.ids[:len(h.ids)-1]
	return x
}

// Find matches names: a glob when pattern has *, ? or [, otherwise a
// case-insensitive substring. Results follow scan order.
func (t *Tree) Find(pattern string, limit int) []ID {
	t.mu.RLock()
	defer t.mu.RUnlock()
	glob := strings.ContainsAny(pattern, "*?[")
	needle := []byte(strings.ToLower(pattern))
	var out []ID
	for id := ID(1); int(id) < len(t.parent) && len(out) < limit; id++ {
		name := t.name(id)
		var ok bool
		if glob {
			ok, _ = path.Match(pattern, string(name))
		} else {
			ok = bytes.Contains(bytes.ToLower(name), needle)
		}
		if ok {
			out = append(out, id)
		}
	}
	return out
}

// WalkDirs visits id and every directory below it in pre-order; returning
// false skips that directory's subtree. The read lock is held for the whole
// walk, so fn must not call back into the Tree.
func (t *Tree) WalkDirs(id ID, fn func(n Node) bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	stack := []ID{id}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !fn(t.node(cur)) {
			continue
		}
		for c := ID(t.first[cur]); c != None; c = ID(t.next[c]) {
			if t.kind[c] == KindDir {
				stack = append(stack, c)
			}
		}
	}
}

// FlagAbove reports whether id or a folder above it carries f.
func (t *Tree) FlagAbove(id ID, f Flag) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for ; id != None; id = ID(t.parent[id]) {
		if t.flags[id]&f != 0 {
			return true
		}
	}
	return false
}

// FlagBelow reports whether a folder inside id carries f.
func (t *Tree) FlagBelow(id ID, f Flag) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	stack := []ID{id}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for c := ID(t.first[cur]); c != None; c = ID(t.next[c]) {
			if t.kind[c] != KindDir {
				continue
			}
			if t.flags[c]&f != 0 {
				return true
			}
			stack = append(stack, c)
		}
	}
	return false
}
