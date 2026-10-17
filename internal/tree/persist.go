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
	version = uint32(1)
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
	for _, v := range []any{t.parent, t.first, t.next, t.nameOff, t.nameLen, t.kind, t.flags, t.alloc, t.unique,
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
	if err := put(uint64(len(t.links))); err != nil {
		return err
	}
	for ino, l := range t.links {
		for _, v := range []any{ino, l.nlink, l.alloc, uint32(len(l.dirs)), l.dirs} {
			if err := put(v); err != nil {
				return err
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
	if err := get(&rootLen); err != nil {
		return nil, err
	}
	root := make([]byte, rootLen)
	if _, err := io.ReadFull(br, root); err != nil {
		return nil, err
	}
	var n uint64
	if err := get(&n); err != nil {
		return nil, err
	}
	t := &Tree{rootPath: string(root), links: map[uint64]*link{},
		parent: make([]uint32, n), first: make([]uint32, n), next: make([]uint32, n), nameOff: make([]uint32, n),
		nameLen: make([]uint16, n), kind: make([]Kind, n), flags: make([]Flag, n), alloc: make([]uint64, n),
		unique: make([]uint64, n), logical: make([]uint64, n), smallAlloc: make([]uint64, n), files: make([]uint32, n),
		smallCount: make([]uint32, n), maxMtime: make([]int64, n), ownMtime: make([]int64, n)}
	for _, v := range []any{t.parent, t.first, t.next, t.nameOff, t.nameLen, t.kind, t.flags, t.alloc, t.unique,
		t.logical, t.smallAlloc, t.files, t.smallCount, t.maxMtime, t.ownMtime} {
		if err := get(v); err != nil {
			return nil, err
		}
	}
	if err := get(&n); err != nil {
		return nil, err
	}
	t.names = make([]byte, n)
	if _, err := io.ReadFull(br, t.names); err != nil {
		return nil, err
	}
	if err := get(&n); err != nil {
		return nil, err
	}
	for range n {
		var ino, alloc uint64
		var nlink, nd uint32
		if err := get(&ino); err != nil {
			return nil, err
		}
		if err := get(&nlink); err != nil {
			return nil, err
		}
		if err := get(&alloc); err != nil {
			return nil, err
		}
		if err := get(&nd); err != nil {
			return nil, err
		}
		dirs := make([]ID, nd)
		if err := get(dirs); err != nil {
			return nil, err
		}
		t.links[ino] = &link{nlink: nlink, alloc: alloc, dirs: dirs}
	}
	return t, nil
}
