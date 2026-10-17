package tree

import "fmt"

type SliceNode struct {
	ID       ID           `json:"id"`
	Name     string       `json:"name"`
	Kind     Kind         `json:"kind"`
	Alloc    uint64       `json:"alloc"`
	Unique   uint64       `json:"unique"`
	Files    uint32       `json:"files"`
	Modified int64        `json:"modified,omitempty"` // unix seconds, newest in subtree
	Other    int          `json:"other,omitempty"`
	Children []*SliceNode `json:"children,omitempty"`
}

// Slice is a level-of-detail view for charts: at most maxNodes nodes, depth
// levels below id, siblings under minShare of their parent merged into one
// "other" node. Levels are filled breadth-first so the top stays complete
// when the budget runs out.
func (t *Tree) Slice(id ID, depth int, minShare float64, maxNodes int) *SliceNode {
	t.mu.RLock()
	defer t.mu.RUnlock()
	root := t.sliceNode(id)
	budget := maxNodes - 1
	type item struct {
		id    ID
		node  *SliceNode
		level int
	}
	queue := []item{{id, root, 0}}
	for len(queue) > 0 && budget > 0 {
		it := queue[0]
		queue = queue[1:]
		if it.level >= depth || t.kind[it.id] != KindDir || t.alloc[it.id] == 0 {
			continue
		}
		parentAlloc := float64(t.alloc[it.id])
		var keep []*SliceNode
		other := &SliceNode{ID: None, Name: "other", Kind: KindOther}
		for c := ID(t.first[it.id]); c != None; c = ID(t.next[c]) {
			if float64(t.alloc[c])/parentAlloc >= minShare && t.alloc[c] > 0 {
				keep = append(keep, t.sliceNode(c))
			} else {
				other.add(t.alloc[c], t.unique[c], t.files[c])
			}
		}
		if n := t.smallCount[it.id]; n > 0 {
			small := &SliceNode{ID: it.id, Name: fmt.Sprintf("%d small files", n), Kind: KindSmall, Alloc: t.smallAlloc[it.id], Unique: t.smallAlloc[it.id], Files: n}
			if float64(small.Alloc)/parentAlloc >= minShare {
				keep = append(keep, small)
			} else {
				other.add(small.Alloc, small.Unique, small.Files)
			}
		}
		sortSlice(keep)
		room := budget
		if other.Other > 0 || len(keep) > room {
			room--
		}
		if room < 0 {
			room = 0
		}
		if len(keep) > room {
			for _, k := range keep[room:] {
				other.add(k.Alloc, k.Unique, k.Files)
			}
			keep = keep[:room]
		}
		it.node.Children = keep
		budget -= len(keep)
		if other.Other > 0 && budget > 0 {
			it.node.Children = append(it.node.Children, other)
			budget--
		}
		for _, k := range keep {
			if k.Kind == KindDir {
				queue = append(queue, item{k.ID, k, it.level + 1})
			}
		}
	}
	return root
}

func (s *SliceNode) add(alloc, unique uint64, files uint32) {
	s.Alloc += alloc
	s.Unique += unique
	s.Files += files
	s.Other++
}

func (t *Tree) sliceNode(id ID) *SliceNode {
	return &SliceNode{ID: id, Name: string(t.name(id)), Kind: t.kind[id], Alloc: t.alloc[id], Unique: t.unique[id], Files: t.files[id], Modified: t.maxMtime[id]}
}

func sortSlice(s []*SliceNode) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].Alloc > s[j-1].Alloc; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
