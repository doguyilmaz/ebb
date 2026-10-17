package walk

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/tree"
)

// Update brings a finished scan's tree up to date by listing again only what
// changed: each folder in dirs is listed one level (new subfolders are walked
// whole, missing ones removed), each in subtrees with everything below it. A
// path the tree does not know yet stands for its nearest known folder, whose
// listing finds it.
func Update(ctx context.Context, t *tree.Tree, dirs, subtrees []string, opt Options) (*Result, error) {
	return UpdateWith(ctx, t, dirs, subtrees, opt, 0, nil)
}

// UpdateWith is Update reporting progress every interval, like Scan.
func UpdateWith(ctx context.Context, t *tree.Tree, dirs, subtrees []string, opt Options, every time.Duration, fn func(Progress)) (*Result, error) {
	root := t.RootPath()
	ioRoot := root
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		ioRoot = resolved
	}
	deep := map[tree.ID]bool{}
	targets := map[tree.ID]string{}
	add := func(p string, below bool) {
		id, ok := nearest(t, p)
		if !ok {
			return
		}
		if _, seen := targets[id]; !seen {
			targets[id] = p
		}
		deep[id] = deep[id] || below
	}
	for _, p := range subtrees {
		add(p, true)
	}
	for _, p := range dirs {
		add(p, false)
	}
	var jobs []job
	for id := range targets {
		if coveredBelow(t, id, deep) {
			continue
		}
		rel := strings.TrimPrefix(t.Path(id), root)
		jobs = append(jobs, job{path: ioRoot + rel, dir: id, refresh: true, deep: deep[id]})
	}
	s := newSession(ctx, opt, t, root, ioRoot)
	s.run(jobs)
	res, err := s.follow(every, fn)
	if s.removed {
		t.Compact()
	}
	return res, err
}

// nearest is the folder at p, or the closest folder above it the tree knows.
func nearest(t *tree.Tree, p string) (tree.ID, bool) {
	root := strings.TrimSuffix(t.RootPath(), "/")
	p = strings.TrimSuffix(filepath.Clean(p), "/")
	for {
		if p != root && !strings.HasPrefix(p, root+"/") {
			return 0, false
		}
		if id, ok := t.Lookup(p); ok && t.Node(id).Kind == tree.KindDir {
			return id, true
		}
		if p == root {
			return 0, false
		}
		p = filepath.Dir(p)
	}
}

// coveredBelow reports whether a folder above id is listed again with
// everything below it, which already includes id.
func coveredBelow(t *tree.Tree, id tree.ID, deep map[tree.ID]bool) bool {
	for p := t.Node(id).Parent; p != tree.None; p = t.Node(p).Parent {
		if deep[p] {
			return true
		}
	}
	return false
}
