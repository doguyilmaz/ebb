package app

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/fsevents"
)

// maxUpdate is how many changed folders an update lists again before a full
// scan is simply faster.
const maxUpdate = 60_000

// changes is the set of folders that changed since the last scan, in the
// tree's own path form, fed by FSEvents.
type changes struct {
	mu       sync.Mutex
	toTree   func(string) (string, bool)
	ignore   string
	dirs     map[string]bool
	subtrees map[string]bool
	lost     bool // events were dropped or the history does not reach back: the next scan is complete
	ready    bool // the replay of history up to now is over
	last     uint64
	wake     chan struct{}
}

func newChanges(toTree func(string) (string, bool), ignore string) *changes {
	return &changes{toTree: toTree, ignore: ignore, dirs: map[string]bool{}, subtrees: map[string]bool{}, wake: make(chan struct{})}
}

func (c *changes) add(evs []fsevents.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range evs {
		c.last = max(c.last, e.ID)
		switch {
		case e.Flags&fsevents.HistoryDone != 0:
			if !c.ready {
				c.ready = true
				close(c.wake)
			}
			continue
		case fsevents.Lost(e.Flags):
			c.lost = true
			continue
		}
		p, ok := c.toTree(e.Path)
		// ebb's own state folder changes with every save; it is not news.
		if !ok || within(p, c.ignore) {
			continue
		}
		if e.Flags&fsevents.MustScanSubDirs != 0 {
			c.subtrees[p] = true
		} else {
			c.dirs[p] = true
		}
		// Past this a full scan is faster, and the set stops growing.
		if len(c.dirs)+len(c.subtrees) > maxUpdate {
			c.lost = true
			c.dirs, c.subtrees = map[string]bool{}, map[string]bool{}
		}
	}
}

type changeSet struct {
	dirs, subtrees []string
	last           uint64
	lost           bool
}

// take hands over what changed so far and starts collecting afresh.
func (c *changes) take() changeSet {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := changeSet{last: c.last, lost: c.lost}
	for p := range c.dirs {
		s.dirs = append(s.dirs, p)
	}
	for p := range c.subtrees {
		s.subtrees = append(s.subtrees, p)
	}
	slices.Sort(s.dirs)
	slices.Sort(s.subtrees)
	c.dirs, c.subtrees = map[string]bool{}, map[string]bool{}
	return s
}

// putBack returns a set an update could not finish, so nothing is missed.
func (c *changes) putBack(s changeSet) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range s.dirs {
		c.dirs[p] = true
	}
	for _, p := range s.subtrees {
		c.subtrees[p] = true
	}
}

func (c *changes) summary() (folders int, lost, ready bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.dirs) + len(c.subtrees), c.lost, c.ready
}

func (c *changes) waitReady(ctx context.Context, limit time.Duration) bool {
	select {
	case <-c.wake:
		return true
	case <-ctx.Done():
	case <-time.After(limit):
	}
	return false
}

type watcher struct {
	root  string
	set   *changes
	stop  context.CancelFunc
	flush chan chan struct{}
	ended chan struct{}
}

// sync hands the set every event so far: the stream delivers them up to a
// second late, and a scan right after a change must see it.
func (w *watcher) sync(ctx context.Context) {
	done := make(chan struct{})
	timeout := time.After(5 * time.Second)
	select {
	case w.flush <- done:
	case <-w.ended:
		return
	case <-ctx.Done():
		return
	case <-timeout:
		return
	}
	select {
	case <-done:
	case <-ctx.Done():
	case <-timeout:
	}
}

// pathMapper turns FSEvents paths (firmlinked display form, /Users/…) into
// the tree's form of the scanned root.
func (a *App) pathMapper(root string) func(string) (string, bool) {
	ioRoot := root
	if r, err := filepath.EvalSymlinks(root); err == nil {
		ioRoot = r
	}
	return func(p string) (string, bool) {
		for _, c := range []string{p, a.Links.Real(p)} {
			if rest, ok := strings.CutPrefix(c, ioRoot); ok && (rest == "" || rest[0] == '/') {
				return root + rest, true
			}
		}
		return "", false
	}
}

// follow starts collecting changes under root after the event ID since; with
// since 0 there is no history to trust, so the set starts out lost.
func (a *App) follow(ctx context.Context, root string, since uint64, uuid string) *watcher {
	toTree := a.pathMapper(root)
	ignore, ok := toTree(a.statePath(""))
	if !ok {
		ignore = a.statePath("")
	}
	set := newChanges(toTree, ignore)
	if since == 0 || uuid == "" || fsevents.UUID(root) != uuid {
		set.lost = true
		since = 0
	}
	if since == 0 {
		set.ready = true
		close(set.wake)
	}
	wctx, stop := context.WithCancel(ctx)
	w := &watcher{root: root, set: set, stop: stop, flush: make(chan chan struct{}), ended: make(chan struct{})}
	go func() {
		defer close(w.ended)
		if err := fsevents.Watch(wctx, root, since, time.Second, set.add, w.flush); err != nil {
			set.mu.Lock()
			set.lost = true
			set.mu.Unlock()
		}
	}()
	return w
}

// Watch keeps track of what changes on disk while ebb serves, so "scan again"
// lists only those folders. It replays what changed since the last scan,
// then follows live, and returns when ctx ends.
func (a *App) Watch(ctx context.Context) {
	a.mu.Lock()
	a.watchCtx = ctx
	a.mu.Unlock()
	if last, err := a.Last(); err == nil {
		a.rewatch(last)
	}
	<-ctx.Done()
	a.mu.Lock()
	if a.watch != nil {
		a.watch.stop()
		a.watch = nil
	}
	a.mu.Unlock()
}

// rewatch follows a new baseline after a full scan, while a server watches.
func (a *App) rewatch(r *ScanResult) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.watchCtx == nil || a.watchCtx.Err() != nil {
		return
	}
	if a.watch != nil {
		a.watch.stop()
	}
	a.watch = a.follow(a.watchCtx, r.Tree.RootPath(), r.Meta.EventID, r.Meta.FSEvents)
}

// changesSince finds what changed after the last scan: from the live watcher
// when one runs, otherwise by replaying the history once. ok is false when
// only a full scan can be trusted.
func (a *App) changesSince(ctx context.Context, last *ScanResult) (changeSet, *changes, bool) {
	root := last.Tree.RootPath()
	if last.Meta.EventID == 0 || last.Meta.FSEvents == "" {
		return changeSet{}, nil, false
	}
	a.mu.Lock()
	w := a.watch
	a.mu.Unlock()
	if w == nil || w.root != root {
		w = a.follow(ctx, root, last.Meta.EventID, last.Meta.FSEvents)
		defer w.stop()
	}
	set := w.set
	if !set.waitReady(ctx, time.Minute) {
		return changeSet{}, nil, false
	}
	w.sync(ctx)
	if _, lost, _ := set.summary(); lost {
		return changeSet{}, nil, false
	}
	return set.take(), set, true
}

// Changes says how many folders changed since the last scan, for the UI.
func (a *App) Changes() *api.Changes {
	a.mu.Lock()
	w := a.watch
	a.mu.Unlock()
	if w == nil {
		return nil
	}
	n, lost, ready := w.set.summary()
	return &api.Changes{Folders: n, Lost: lost, Ready: ready}
}
