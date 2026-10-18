// Package dupes finds files with the same content among those the last scan
// stored (1 MiB and up), in the person's own folders. Hashing reads files, so
// it runs only when asked: same size first, then the first and last 64 KiB,
// then all of it.
package dupes

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

type Copy struct {
	Path  string `json:"path"`
	Mtime int64  `json:"mtime"`
	// Bytes is what removing this copy frees (less when part of it is shared).
	Bytes uint64 `json:"bytes"`
}

type Group struct {
	Size   uint64 `json:"size"` // of one copy
	Copies []Copy `json:"copies"`
	// Wasted is what removing every copy but the first would free.
	Wasted uint64 `json:"wasted"`
}

type Result struct {
	Root    string        `json:"root"`
	Time    time.Time     `json:"time"`
	Groups  []Group       `json:"groups"`
	Wasted  uint64        `json:"wasted"`
	Hashed  int           `json:"hashed"` // files read
	Elapsed time.Duration `json:"elapsed"`
}

type Progress struct {
	Phase string `json:"phase"` // sample | hash
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

// Folders whose files belong to something else are skipped: bundles and
// libraries that break when a file inside goes, app data, dependencies and
// every dot-folder (tool data, VCS, the Trash).
var skipNames = []string{"Library", "node_modules"}

var bundleExts = []string{".app", ".bundle", ".framework", ".photoslibrary", ".musiclibrary", ".imovielibrary", ".tvlibrary",
	".fcpbundle", ".logicx", ".band", ".xcarchive", ".lrlibrary", ".lrdata", ".aplibrary", ".pkg", ".plugin", ".kext", ".xcodeproj"}

func skipped(name string) bool {
	return strings.HasPrefix(name, ".") || slices.Contains(skipNames, name) || slices.Contains(bundleExts, strings.ToLower(filepath.Ext(name)))
}

const sample = 64 << 10

type cand struct {
	id    tree.ID
	path  string
	size  uint64
	mtime int64
	bytes uint64
}

// Find looks below root (a folder in the tree) for files at least minSize
// big that share their content. Hardlinks and pure clones share their data
// already and are left out.
func Find(ctx context.Context, t *tree.Tree, links volume.Firmlinks, root tree.ID, minSize uint64, progress func(Progress)) (Result, error) {
	start := time.Now()
	if progress == nil {
		progress = func(Progress) {}
	}
	// The walk holds the tree's read lock: folders first, their files after.
	var dirs []tree.ID
	t.WalkDirs(root, func(n tree.Node) bool {
		if n.ID != root && skipped(n.Name) {
			return false
		}
		dirs = append(dirs, n.ID)
		return true
	})
	bySize := map[uint64][]cand{}
	for _, d := range dirs {
		for _, c := range t.Children(d) {
			f := t.Node(c)
			if f.Kind != tree.KindFile || f.Logical < minSize || f.Flags&tree.FlagShared != 0 || f.Flags&tree.FlagClone != 0 && f.Unique == 0 {
				continue
			}
			bySize[f.Logical] = append(bySize[f.Logical], cand{id: c, size: f.Logical, mtime: f.MaxMtime})
		}
	}
	var sized [][]cand
	total := 0
	for _, cs := range bySize {
		if len(cs) > 1 {
			for i := range cs {
				cs[i].path = t.Path(cs[i].id)
				cs[i].bytes = t.Reclaimable(cs[i].id)
			}
			sized = append(sized, cs)
			total += len(cs)
		}
	}
	res := Result{Root: links.Display(t.Path(root)), Time: start}
	sampled, err := regroup(ctx, sized, total, "sample", progress, func(c cand) (string, error) { return digest(c.path, c.size, true) })
	if err != nil {
		return res, err
	}
	total = 0
	for _, g := range sampled {
		total += len(g)
	}
	full, err := regroup(ctx, sampled, total, "hash", progress, func(c cand) (string, error) {
		// A file of at most two samples was read whole already.
		if c.size <= 2*sample {
			return "same", nil
		}
		return digest(c.path, c.size, false)
	})
	if err != nil {
		return res, err
	}
	res.Hashed = total
	for _, g := range full {
		// The copy to keep comes first: the one whose removal frees least (it
		// shares its data elsewhere), then the newest.
		slices.SortFunc(g, func(a, b cand) int {
			return cmp.Or(cmp.Compare(a.bytes, b.bytes), cmp.Compare(b.mtime, a.mtime), cmp.Compare(a.path, b.path))
		})
		grp := Group{Size: g[0].size}
		for i, c := range g {
			grp.Copies = append(grp.Copies, Copy{Path: links.Display(c.path), Mtime: c.mtime, Bytes: c.bytes})
			if i > 0 {
				grp.Wasted += c.bytes
			}
		}
		res.Groups = append(res.Groups, grp)
		res.Wasted += grp.Wasted
	}
	slices.SortFunc(res.Groups, func(a, b Group) int {
		return cmp.Or(cmp.Compare(b.Wasted, a.Wasted), cmp.Compare(a.Copies[0].Path, b.Copies[0].Path))
	})
	res.Elapsed = time.Since(start)
	return res, nil
}

// regroup splits each group by key, keeping groups of two or more; files
// that cannot be read drop out.
func regroup(ctx context.Context, groups [][]cand, total int, phase string, progress func(Progress), key func(cand) (string, error)) ([][]cand, error) {
	type job struct {
		g, i int
	}
	keys := make([][]string, len(groups))
	jobs := make(chan job)
	var mu sync.Mutex
	done := 0
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for j := range jobs {
				k, err := key(groups[j.g][j.i])
				if err != nil {
					k = ""
				}
				mu.Lock()
				keys[j.g][j.i] = k
				done++
				if done%64 == 0 || done == total {
					progress(Progress{Phase: phase, Done: done, Total: total})
				}
				mu.Unlock()
			}
		})
	}
	var err error
feed:
	for g := range groups {
		keys[g] = make([]string, len(groups[g]))
		for i := range groups[g] {
			select {
			case jobs <- job{g, i}:
			case <-ctx.Done():
				err = ctx.Err()
				break feed
			}
		}
	}
	close(jobs)
	wg.Wait()
	if err != nil {
		return nil, err
	}
	var out [][]cand
	for g, cs := range groups {
		by := map[string][]cand{}
		for i, c := range cs {
			if k := keys[g][i]; k != "" {
				by[k] = append(by[k], c)
			}
		}
		for _, sub := range by {
			if len(sub) > 1 {
				out = append(out, sub)
			}
		}
	}
	return out, nil
}

// digest hashes the first and last 64 KiB (sampled) or the whole file. It
// bypasses the page cache, so a search through gigabytes does not push out
// what other apps keep in memory.
func digest(path string, size uint64, sampled bool) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	noCache(f)
	h := sha256.New()
	buf := make([]byte, 1<<20)
	if !sampled {
		if _, err := io.CopyBuffer(h, f, buf); err != nil {
			return "", err
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	n, err := io.ReadFull(f, buf[:min(sample, int(size))])
	if err != nil {
		return "", err
	}
	h.Write(buf[:n])
	if size > 2*sample {
		if _, err := f.ReadAt(buf[:sample], int64(size)-sample); err != nil {
			return "", err
		}
		h.Write(buf[:sample])
	} else if size > sample {
		m, err := io.ReadFull(f, buf[:int(size)-sample])
		if err != nil {
			return "", err
		}
		h.Write(buf[:m])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
