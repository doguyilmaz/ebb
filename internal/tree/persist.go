package tree

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"io"
)

const (
	magic   = "EBBT"
	version = uint32(3)
)

var le = binary.LittleEndian

// Save writes the tree as gzip(BestSpeed) of fixed-width little-endian arrays.
func (t *Tree) Save(w io.Writer) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	zw, _ := gzip.NewWriterLevel(w, gzip.BestSpeed)
	bw := bufio.NewWriterSize(zw, 1<<20)
	put := func(v any) error { return binary.Write(bw, le, v) }
	if _, err := bw.WriteString(magic); err != nil {
		return err
	}
	for _, v := range []any{version, uint32(len(t.rootPath))} {
		if err := put(v); err != nil {
			return err
		}
	}
	if _, err := bw.WriteString(t.rootPath); err != nil {
		return err
	}
	if err := put(uint64(len(t.parent))); err != nil {
		return err
	}
	for _, v := range []any{t.parent, t.first, t.next, t.nameOff, t.nameLen, t.depth, t.markers, t.kind, t.flags, t.alloc, t.unique,
		t.logical, t.smallAlloc, t.files, t.smallCount, t.maxMtime, t.ownMtime} {
		if err := put(v); err != nil {
			return err
		}
	}
	if err := put(uint64(len(t.names))); err != nil {
		return err
	}
	if _, err := bw.Write(t.names); err != nil {
		return err
	}
	for _, m := range []map[uint64]*group{t.links, t.clones} {
		if err := put(uint64(len(m))); err != nil {
			return err
		}
		for key, g := range m {
			for _, v := range []any{key, g.nlink, g.seen, g.alloc, g.first, g.lca} {
				if err := put(v); err != nil {
					return err
				}
			}
		}
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	return zw.Close()
}

func Load(r io.Reader) (*Tree, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	br := bufio.NewReaderSize(zr, 1<<20)
	get := func(v any) error { return binary.Read(br, le, v) }
	head := make([]byte, len(magic))
	if _, err := io.ReadFull(br, head); err != nil || string(head) != magic {
		return nil, errors.New("not an ebb tree file")
	}
	var ver, rootLen uint32
	if err := get(&ver); err != nil || ver != version {
		return nil, errors.New("unsupported tree file version")
	}
	if err := get(&rootLen); err != nil || rootLen > 4096 {
		return nil, errors.New("corrupt tree file: root path")
	}
	root := make([]byte, rootLen)
	if _, err := io.ReadFull(br, root); err != nil {
		return nil, err
	}
	var n uint64
	if err := get(&n); err != nil || n == 0 || n > maxNodes {
		return nil, errors.New("corrupt tree file: node count")
	}
	t := &Tree{rootPath: string(root), links: map[uint64]*group{}, clones: map[uint64]*group{},
		parent: make([]uint32, n), first: make([]uint32, n), next: make([]uint32, n), nameOff: make([]uint32, n),
		nameLen: make([]uint16, n), depth: make([]uint16, n), markers: make([]uint16, n), kind: make([]Kind, n), flags: make([]Flag, n), alloc: make([]uint64, n),
		unique: make([]uint64, n), logical: make([]uint64, n), smallAlloc: make([]uint64, n), files: make([]uint32, n),
		smallCount: make([]uint32, n), maxMtime: make([]int64, n), ownMtime: make([]int64, n)}
	for _, v := range []any{t.parent, t.first, t.next, t.nameOff, t.nameLen, t.depth, t.markers, t.kind, t.flags, t.alloc, t.unique,
		t.logical, t.smallAlloc, t.files, t.smallCount, t.maxMtime, t.ownMtime} {
		if err := get(v); err != nil {
			return nil, err
		}
	}
	nodes := n
	if err := get(&n); err != nil || n > maxNames {
		return nil, errors.New("corrupt tree file: names")
	}
	t.names = make([]byte, n)
	if _, err := io.ReadFull(br, t.names); err != nil {
		return nil, err
	}
	for _, m := range []map[uint64]*group{t.links, t.clones} {
		if err := get(&n); err != nil || n > nodes*64 {
			return nil, errors.New("corrupt tree file: shared groups")
		}
		for range n {
			var key uint64
			g := &group{}
			for _, v := range []any{&key, &g.nlink, &g.seen, &g.alloc, &g.first, &g.lca} {
				if err := get(v); err != nil {
					return nil, err
				}
			}
			if uint64(g.first) >= nodes || uint64(g.lca) >= nodes {
				return nil, errors.New("corrupt tree file: shared group location")
			}
			m[key] = g
		}
	}
	if _, err := io.Copy(io.Discard, br); err != nil {
		return nil, err
	}
	if err := t.validate(); err != nil {
		return nil, err
	}
	return t, nil
}

const (
	maxNodes = 1 << 31
	maxNames = 1 << 34
)

// validate checks every index a reader will follow, so a damaged file fails
// to load instead of crashing a query later.
func (t *Tree) validate() error {
	n := uint32(len(t.parent))
	none := uint32(None)
	for i := range n {
		if (i == 0) != (t.parent[i] == none) || (t.parent[i] != none && t.parent[i] >= i) ||
			(t.first[i] != none && t.first[i] >= n) || (t.next[i] != none && t.next[i] >= n) ||
			uint64(t.nameOff[i])+uint64(t.nameLen[i]) > uint64(len(t.names)) || t.kind[i] > KindSmall {
			return errors.New("corrupt tree file: node links")
		}
	}
	return nil
}
