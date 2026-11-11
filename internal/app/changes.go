package app

import (
	"context"
	"fmt"
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

// The live stream drops events when a burst of changes outruns it (a build,
// a checkout, a few thousand folders created at once), and says so with a
// UserDropped event. The history macOS keeps on disk still has them, so the
// watcher starts a new stream from the last event it received before the drop
// and lets the history fill the gap.
//
// Not at once, though: measured while a burst was still running, the history
// of its last second or two was not on disk yet, and a replay that reached it
// came back with drops of its own. The same replay a few seconds after the
// burst was complete. So the restart waits until drops have stopped for
// calmFor (at most calmMax after the first one), and a replay that is dropped
// is tried again from wherever it got to, after the same wait. A burst of 44k
// folders over 18 s took three recoveries.
//
// Recoveries are bounded all the same: more than maxRecoveries within
// recoveryWindow means an overload that does not end, and the set is lost;
// the next scan reads everything, as it always did.
const (
	calmFor        = 5 * time.Second
	calmMax        = 30 * time.Second
	maxRecoveries  = 8
	recoveryWindow = 10 * time.Minute
)

// phase says whether what the stream delivered so far is the whole truth.
type phase int

const (
	live       phase = iota // events arrive as they happen: the set is complete
	restarting              // a drop was seen: the stream is about to restart from the last good event
	replaying               // history is being replayed: the set is complete once it says it is done
)

// changes is the set of folders that changed since the last scan, in the
// tree's own path form, fed by FSEvents.
type changes struct {
	mu       sync.Mutex
	toTree   func(string) (string, bool)
	ignore   string
	dirs     map[string]bool
	subtrees map[string]bool
	lost     bool   // events were dropped or the history does not reach back: the next scan is complete
	lostWhy  string // the first reason it was lost
	ready    bool   // nothing is being replayed: the set is complete (or lost, which is settled too)
	last     uint64
	wake     chan struct{} // closed when ready, new each time the set stops being ready

	phase phase
	// good is the newest event ID received while the stream was whole; a
	// restart replays everything after it. Events do not always come in ID
	// order: after a replay drops, events below good still arrive for tens of
	// milliseconds, and only the old stream, which runs until the restart,
	// puts them in the set.
	good             uint64
	restartAt        uint64
	restart          chan struct{} // told once per recovery, read by the goroutine that owns the stream
	recoveries       []time.Time
	firstDrop        time.Time // of the recovery under way
	lastDrop         time.Time
	calmFor, calmMax time.Duration
	now              func() time.Time
}

func newChanges(toTree func(string) (string, bool), ignore string) *changes {
	return &changes{toTree: toTree, ignore: ignore, dirs: map[string]bool{}, subtrees: map[string]bool{}, wake: make(chan struct{}),
		phase: replaying, restart: make(chan struct{}, 1), calmFor: calmFor, calmMax: calmMax, now: time.Now}
}

// lose settles the set as lost: the next scan has to be complete. Nobody
// waits for a replay of a set that cannot be trusted anyway.
func (c *changes) lose(why string) {
	if !c.lost {
		c.lost, c.lostWhy = true, why
	}
	c.phase = live
	c.settle()
}

func (c *changes) settle() {
	if !c.ready {
		c.ready = true
		close(c.wake)
	}
}

// dropped handles an event that says events were lost.
func (c *changes) dropped(e fsevents.Event) {
	switch {
	case c.lost:
	case e.Flags&fsevents.KernelDropped != 0:
		c.lose("the kernel dropped events")
	case e.Flags&fsevents.IDsWrapped != 0:
		c.lose("the event IDs wrapped around")
	case e.Flags&fsevents.RootChanged != 0:
		c.lose("the scanned folder was moved or replaced")
	case c.phase == restarting:
		// The replay from the last good event covers this one too, once it is quiet.
		c.lastDrop = c.now()
	default:
		// Live, or a replay that was dropped itself: the events it did deliver
		// moved the last good event on, so the next try starts there.
		c.recover(e)
	}
}

// recover asks for a new stream that starts at the last good event.
func (c *changes) recover(e fsevents.Event) {
	now := c.now()
	c.recoveries = slices.DeleteFunc(c.recoveries, func(t time.Time) bool { return now.Sub(t) > recoveryWindow })
	if len(c.recoveries) >= maxRecoveries {
		c.lose(fmt.Sprintf("macOS dropped events more than %d times in %d minutes", maxRecoveries, int(recoveryWindow.Minutes())))
		return
	}
	from := c.good
	// A drop that names a point at or before the last good event restarts
	// from just before that point.
	if e.ID > 0 && e.ID <= from {
		from = e.ID - 1
	}
	if from == 0 {
		c.lose("macOS dropped events and there is no earlier point in the change history to replay from")
		return
	}
	c.recoveries = append(c.recoveries, now)
	c.phase, c.restartAt, c.firstDrop, c.lastDrop = restarting, from, now, now
	// A drop during a replay leaves the set not ready: whoever already waits
	// keeps the channel that closes once this recovery is over.
	if c.ready {
		c.ready, c.wake = false, make(chan struct{})
	}
	select {
	case c.restart <- struct{}{}:
	default:
	}
}

// untilCalm is how long the restart still has to wait for drops to stop.
func (c *changes) untilCalm() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	return min(c.lastDrop.Add(c.calmFor).Sub(now), c.firstDrop.Add(c.calmMax).Sub(now))
}

// restarted is called once the old stream has stopped: the new one replays
// from the returned event ID. ok is false when the set was lost meanwhile.
func (c *changes) restarted() (from uint64, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lost || c.phase != restarting {
		return 0, false
	}
	c.phase = replaying
	return c.restartAt, true
}

func (c *changes) add(evs []fsevents.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range evs {
		c.last = max(c.last, e.ID)
		switch {
		case e.Flags&fsevents.HistoryDone != 0:
			if c.phase == replaying {
				c.phase = live
				c.settle()
			}
			continue
		case fsevents.Lost(e.Flags):
			c.dropped(e)
			continue
		}
		// What arrives between a drop and the restart goes into the set but
		// does not move good: the stream is no longer whole.
		if c.phase != restarting {
			c.good = max(c.good, e.ID)
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
			c.lose(fmt.Sprintf("more than %d folders changed", maxUpdate))
			c.dirs, c.subtrees = map[string]bool{}, map[string]bool{}
		}
	}
}

type changeSet struct {
	dirs, subtrees []string
	last           uint64
}

// takeSettled hands over what changed so far, when it is complete, and starts
// collecting afresh. ok is false when only a full scan can be trusted; wait is
// true when a recovery began meanwhile and the set is not complete yet.
func (c *changes) takeSettled() (s changeSet, ok, wait bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case c.lost:
		return changeSet{}, false, false
	case !c.ready:
		return changeSet{}, false, true
	}
	s = changeSet{last: c.last}
	for p := range c.dirs {
		s.dirs = append(s.dirs, p)
	}
	for p := range c.subtrees {
		s.subtrees = append(s.subtrees, p)
	}
	slices.Sort(s.dirs)
	slices.Sort(s.subtrees)
	c.dirs, c.subtrees = map[string]bool{}, map[string]bool{}
	return s, true, false
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

func (c *changes) report() api.Changes {
	c.mu.Lock()
	defer c.mu.Unlock()
	return api.Changes{Folders: len(c.dirs) + len(c.subtrees), Lost: c.lost, Ready: c.ready, LostWhy: c.lostWhy}
}

// waitReady waits until no replay is running, the first one after the last
// scan or one that follows a drop.
func (c *changes) waitReady(ctx context.Context) bool {
	c.mu.Lock()
	wake := c.wake
	c.mu.Unlock()
	select {
	case <-wake:
		return true
	case <-ctx.Done():
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

// watchFunc is fsevents.Watch; tests play scripted streams instead.
type watchFunc func(ctx context.Context, root string, since uint64, latency time.Duration, fn func([]fsevents.Event), flush <-chan chan struct{}) error

// run feeds the set from a stream of events under root, starting after the
// event ID since. When the stream reports dropped events it is replaced, once
// drops have stopped, by one that starts at the last event received before the
// drop. A stream is never stopped from its own callback: the callback only
// asks, this goroutine stops and starts. The old stream keeps running (and
// answering flush requests) until the new one starts.
func (w *watcher) run(ctx context.Context, since uint64, watch watchFunc) {
	defer close(w.ended)
	for {
		sctx, cancel := context.WithCancel(ctx)
		end := make(chan error, 1)
		go func() { end <- watch(sctx, w.root, since, time.Second, w.set.add, w.flush) }()
		var calm <-chan time.Time
	stream:
		for {
			select {
			case err := <-end:
				cancel()
				if err != nil {
					w.set.mu.Lock()
					w.set.lose("could not follow the volume: " + err.Error())
					w.set.mu.Unlock()
				}
				return
			case <-w.set.restart:
				calm = time.After(max(0, w.set.untilCalm()))
			case <-calm:
				if d := w.set.untilCalm(); d > 0 {
					calm = time.After(d)
					continue
				}
				break stream
			}
		}
		cancel()
		<-end
		from, ok := w.set.restarted()
		if !ok {
			return
		}
		since = from
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
	switch {
	case since == 0 || uuid == "":
		set.lose("the last scan did not record where the volume's change history stood")
		since = 0
	case fsevents.UUID(root) != uuid:
		set.lose("the volume's change history was reset since the last scan")
		since = 0
	default:
		set.good = since
	}
	return startWatcher(ctx, root, since, set, fsevents.Watch)
}

func startWatcher(ctx context.Context, root string, since uint64, set *changes, watch watchFunc) *watcher {
	wctx, stop := context.WithCancel(ctx)
	w := &watcher{root: root, set: set, stop: stop, flush: make(chan chan struct{}), ended: make(chan struct{})}
	go w.run(wctx, since, watch)
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
	// A drop can come in with the flush; then the set is incomplete again
	// until the replay that fills the gap is done.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	for {
		if !set.waitReady(ctx) {
			return changeSet{}, nil, false
		}
		w.sync(ctx)
		if s, ok, wait := set.takeSettled(); !wait {
			return s, set, ok
		}
	}
}

// Changes says how many folders changed since the last scan, for the UI.
func (a *App) Changes() *api.Changes {
	a.mu.Lock()
	w := a.watch
	a.mu.Unlock()
	if w == nil {
		return nil
	}
	ch := w.set.report()
	return &ch
}
