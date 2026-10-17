// Package history keeps a compact snapshot of every full scan (directories of
// 10 MB and more) so growth can be explained over time.
package history

import (
	"cmp"
	"compress/gzip"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/doguyilmaz/ebb/internal/state"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

const (
	keepRecent = 60
	idLayout   = "20060102T150405Z"
)

type Meta struct {
	ID      string        `json:"id"`
	Time    time.Time     `json:"time"`
	Root    string        `json:"root"`
	Used    uint64        `json:"used"`
	Free    uint64        `json:"free"`
	Scanned uint64        `json:"scanned"`
	Elapsed time.Duration `json:"elapsed"`
	Dirs    uint64        `json:"dirs"`
	Files   uint64        `json:"files"`
	// EventID and FSEvents (the volume's FSEvents history UUID) mark where the
	// scan stands in the volume's change history, for incremental updates.
	EventID     uint64 `json:"eventId,omitempty"`
	FSEvents    string `json:"fseventsUuid,omitempty"`
	Incremental bool   `json:"incremental,omitempty"`
	Changed     uint64 `json:"changed,omitempty"` // folders listed again by an incremental update
}

type Dir struct {
	Path     string
	Alloc    uint64
	Unique   uint64
	Files    uint32
	MaxMtime int64
}

type Snapshot struct {
	Meta   Meta
	Volume volume.Breakdown
	Dirs   []Dir
}

var ErrNone = errors.New("no snapshot")

type Store struct{ dir string }

func Open(stateDir string) *Store { return &Store{dir: filepath.Join(stateDir, "snapshots")} }

// FromTree keeps directories of at least min bytes, with display paths.
func FromTree(t *tree.Tree, min uint64, display func(string) string) []Dir {
	var ids []tree.ID
	t.WalkDirs(t.Root(), func(n tree.Node) bool {
		if n.Alloc < min {
			return false
		}
		ids = append(ids, n.ID)
		return true
	})
	dirs := make([]Dir, 0, len(ids))
	for _, id := range ids {
		n := t.Node(id)
		dirs = append(dirs, Dir{Path: display(t.Path(id)), Alloc: n.Alloc, Unique: n.Unique, Files: n.Files, MaxMtime: n.MaxMtime})
	}
	slices.SortFunc(dirs, func(a, b Dir) int { return cmp.Compare(a.Path, b.Path) })
	return dirs
}

func (s *Store) index() string { return filepath.Join(s.dir, "index.json") }

// List returns snapshot metadata, newest first.
func (s *Store) List() ([]Meta, error) {
	var metas []Meta
	if err := state.LoadJSON(s.index(), &metas); err != nil && !state.IsNotExist(err) {
		return nil, err
	}
	return metas, nil
}

func (s *Store) Save(snap *Snapshot) error {
	metas, err := s.List()
	if err != nil {
		return err
	}
	base := snap.Meta.Time.UTC().Format(idLayout)
	id := base
	for n := 2; slices.ContainsFunc(metas, func(m Meta) bool { return m.ID == id }); n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	snap.Meta.ID = id
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, ".snap-*")
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(f)
	if err := gob.NewEncoder(zw).Encode(snap); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return err
	}
	if err := os.Rename(f.Name(), s.file(id)); err != nil {
		return err
	}
	metas = append(metas, snap.Meta)
	slices.SortFunc(metas, func(a, b Meta) int { return b.Time.Compare(a.Time) })
	keep, drop := retain(metas)
	for _, m := range drop {
		os.Remove(s.file(m.ID))
	}
	return state.SaveJSON(s.index(), keep)
}

func (s *Store) file(id string) string { return filepath.Join(s.dir, id+".snap") }

// retain keeps the newest snapshots and one per ISO week before them.
func retain(newestFirst []Meta) (keep, drop []Meta) {
	seen := map[[2]int]bool{}
	for i, m := range newestFirst {
		if i < keepRecent {
			keep = append(keep, m)
			continue
		}
		y, w := m.Time.ISOWeek()
		if seen[[2]int{y, w}] {
			drop = append(drop, m)
			continue
		}
		seen[[2]int{y, w}] = true
		keep = append(keep, m)
	}
	return keep, drop
}

func (s *Store) Load(id string) (*Snapshot, error) {
	f, err := os.Open(s.file(filepath.Base(id)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := gob.NewDecoder(zr).Decode(&snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// Previous is the newest snapshot of root taken before t.
func (s *Store) Previous(root string, t time.Time) (*Snapshot, error) {
	metas, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, m := range metas {
		if m.Root == root && m.Time.Before(t) {
			return s.Load(m.ID)
		}
	}
	return nil, ErrNone
}
