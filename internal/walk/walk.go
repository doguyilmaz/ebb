// Package walk scans a directory tree in parallel. Workers only read
// directories; one builder goroutine owns every write to the tree, so the
// arena needs no fine-grained locking and readers can watch it fill live.
package walk

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/doguyilmaz/ebb/internal/tree"
)

type Options struct {
	Root           string
	Threads        int    // 0 → min(8, NumCPU)
	Gentle         bool   // throttled I/O for background scans
	SmallThreshold uint64 // 0 → 1 MiB
	Reader         string // "" picks the fastest reader for the platform; "generic" forces ReadDir+Lstat
	maxOpen        int64  // directories kept open at once; 0 derives it from the process limit
}

type Progress struct {
	Dirs, Files, Bytes, Problems uint64
	Path                         string
	Elapsed                      time.Duration
	Done                         bool
}

type Problem struct {
	Path string `json:"path"`
	Err  string `json:"err"`
}

type Result struct {
	Tree        *tree.Tree
	Problems    []Problem
	Elapsed     time.Duration
	Dirs, Files uint64
}

const (
	// Batches from huge directories are dropped instead of pooled so a few
	// outliers do not pin their buffers for the whole scan.
	maxPooledEntries = 16 << 10

	efMayShareBlocks = 0x1
	efIsSyncRoot     = 0x4
	sfDataless       = 0x40000000
)

type entry struct {
	nameOff   uint32
	nameLen   uint16
	kind      tree.Kind
	mount     bool
	alloc     uint64
	logical   uint64
	mtime     int64
	nlink     uint32
	ino       uint64
	flags     uint32
	ext       uint32
	cloneID   uint64
	cloneRefs uint32
	err       syscall.Errno
}

type batch struct {
	dir     tree.ID
	path    string
	entries []entry
	names   []byte
	err     error
	// fd stays open after reading so subfolders open relative to it; the
	// builder hands it to their jobs.
	fd            *dirFD
	refresh, deep bool
}

// dirFD is an open directory shared by the jobs of its subfolders. Opening a
// child with openat skips the kernel's lookup of every path component, which
// was the largest part of a scan.
type dirFD struct {
	fd   int
	refs atomic.Int32
	open *atomic.Int64
}

func (d *dirFD) release() {
	if d != nil && d.refs.Add(-1) == 0 {
		closeFD(d.fd)
		d.open.Add(-1)
	}
}

func (b *batch) name(e *entry) []byte { return b.names[e.nameOff : e.nameOff+uint32(e.nameLen)] }

func (b *batch) addName(name string) (uint32, uint16) {
	off := uint32(len(b.names))
	b.names = append(b.names, name...)
	return off, uint16(len(name))
}

type reader interface {
	// readDir lists j's directory into b. A reader that can keep the
	// directory open sets b.fd when s still has descriptors to spare.
	readDir(s *Session, j job, b *batch) error
}

type job struct {
	path   string
	dir    tree.ID
	parent *dirFD // open parent directory, or nil to open by path
	// refresh lists a folder already in the tree again; deep also lists
	// everything below it again.
	refresh, deep bool
}

type queue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	jobs   []job
	closed bool
}

func newQueue() *queue {
	q := &queue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *queue) push(j job) {
	q.mu.Lock()
	q.jobs = append(q.jobs, j)
	q.mu.Unlock()
	q.cond.Signal()
}

func (q *queue) pop() (job, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.jobs) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.jobs) == 0 {
		return job{}, false
	}
	j := q.jobs[len(q.jobs)-1]
	q.jobs = q.jobs[:len(q.jobs)-1]
	return j, true
}

func (q *queue) close() {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	q.cond.Broadcast()
}

type Session struct {
	ctx       context.Context
	opt       Options
	t         *tree.Tree
	q         *queue
	batches   chan *batch
	pool      sync.Pool
	threshold uint64
	start     time.Time

	dirs, files, bytes, nproblems atomic.Uint64
	current                       atomic.Pointer[string]

	root, ioRoot string
	removed      bool // a refresh took nodes out; the arena needs compacting
	update       bool // an update sorts only the folders it touched
	touched      []tree.ID
	openFDs      atomic.Int64
	fdBudget     int64
	pending      int
	problems     []Problem
	done         chan struct{}
	elapsed      time.Duration
}

// Start validates the root and begins scanning in the background.
func Start(ctx context.Context, opt Options) (*Session, error) {
	root, err := filepath.Abs(opt.Root)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	// Reads go through the resolved root so no path component is a symlink;
	// the tree keeps the name the caller used.
	ioRoot := root
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		ioRoot = resolved
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}
	s := newSession(ctx, opt, tree.New(root), root, ioRoot)
	s.run([]job{{path: ioRoot, dir: s.t.Root()}})
	return s, nil
}

func newSession(ctx context.Context, opt Options, t *tree.Tree, root, ioRoot string) *Session {
	if opt.Threads <= 0 {
		opt.Threads = min(8, runtime.NumCPU())
	}
	opt.Threads = min(opt.Threads, 64)
	if opt.SmallThreshold == 0 {
		opt.SmallThreshold = 1 << 20
	}
	s := &Session{
		ctx: ctx, opt: opt, t: t, q: newQueue(), batches: make(chan *batch, 256),
		threshold: opt.SmallThreshold, start: time.Now(), done: make(chan struct{}), root: root, ioRoot: ioRoot,
	}
	s.pool.New = func() any { return &batch{} }
	s.fdBudget = opt.maxOpen
	if s.fdBudget == 0 {
		s.fdBudget = fdBudget()
	}
	return s
}

// run queues the first jobs, then starts the readers and the builder.
func (s *Session) run(jobs []job) {
	setIOPolicy(s.opt.Gentle)
	r := newReader(s.opt.Reader)
	s.pending = len(jobs)
	for _, j := range jobs {
		s.q.push(j)
	}
	if len(jobs) == 0 {
		s.q.close()
	}
	var workers sync.WaitGroup
	for range s.opt.Threads {
		workers.Go(func() { s.work(r) })
	}
	go func() {
		workers.Wait()
		close(s.batches)
	}()
	go s.build()
}

func (s *Session) work(r reader) {
	for {
		j, ok := s.q.pop()
		if !ok {
			return
		}
		b := s.pool.Get().(*batch)
		b.dir, b.path, b.entries, b.names, b.err, b.fd = j.dir, j.path, b.entries[:0], b.names[:0], nil, nil
		b.refresh, b.deep = j.refresh, j.deep
		if err := s.ctx.Err(); err != nil {
			b.err = err
		} else {
			b.err = r.readDir(s, j, b)
		}
		j.parent.release()
		s.batches <- b
	}
}

// keep wraps an open directory for its subfolders' jobs, or reports false
// when too many are open already.
func (s *Session) keep(fd int) (*dirFD, bool) {
	if s.openFDs.Add(1) > s.fdBudget {
		s.openFDs.Add(-1)
		return nil, false
	}
	d := &dirFD{fd: fd, open: &s.openFDs}
	d.refs.Store(1)
	return d, true
}

func (s *Session) build() {
	for b := range s.batches {
		s.apply(b)
		b.fd.release()
		b.fd = nil
		if cap(b.entries) <= maxPooledEntries {
			s.pool.Put(b)
		}
		s.pending--
		if s.pending == 0 {
			s.q.close()
		}
	}
	if s.update {
		s.t.FinishDirs(s.touched)
	} else {
		s.t.Finish()
	}
	s.elapsed = time.Since(s.start)
	close(s.done)
}

func closeFD(fd int) { syscall.Close(fd) }

// fdBudget is how many directories may stay open at once: half the process
// limit, so the rest of ebb (server, history) never runs out.
func fdBudget() int64 {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		return 64
	}
	return max(64, min(4096, int64(lim.Cur/2)))
}

func childPath(dir string, name []byte) string {
	if strings.HasSuffix(dir, "/") {
		return dir + string(name)
	}
	return dir + "/" + string(name)
}

func (s *Session) problem(path string, err error) {
	if rest, ok := strings.CutPrefix(path, s.ioRoot); ok && s.ioRoot != s.root {
		path = s.root + rest
	}
	msg := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Op + ": " + pe.Err.Error()
	}
	s.problems = append(s.problems, Problem{Path: path, Err: msg})
	s.nproblems.Add(1)
}

func (s *Session) apply(b *batch) {
	if b.err != nil {
		if errors.Is(b.err, context.Canceled) || errors.Is(b.err, context.DeadlineExceeded) {
			return
		}
		if b.refresh && errors.Is(b.err, fs.ErrNotExist) {
			s.t.Write(func(w *tree.Writer) {
				s.touched = append(s.touched, w.Node(b.dir).Parent)
				w.Remove(b.dir)
			})
			s.removed = true
			return
		}
		s.t.Write(func(w *tree.Writer) { w.SetFlags(b.dir, tree.FlagDenied) })
		if !s.ownTrash(b) {
			s.problem(b.path, b.err)
		}
		return
	}
	p := b.path
	if rest, ok := strings.CutPrefix(p, s.ioRoot); ok && s.ioRoot != s.root {
		p = s.root + rest
	}
	s.current.Store(&p)
	if s.update {
		s.touched = append(s.touched, b.dir)
	}
	var files, bytes uint64
	s.t.Write(func(w *tree.Writer) {
		var smallN uint32
		var smallAlloc, smallUnique, smallLogical uint64
		var smallMtime int64
		var markers uint16
		// A refresh lists a known folder again: its files are cleared and
		// added back, its subfolders stay unless they are gone. Copies of
		// shared data first counted here are this folder's own.
		var keep map[string]tree.ID
		var own func(tree.ID) bool
		if b.refresh {
			keep = map[string]tree.ID{}
			for _, c := range w.Children(b.dir) {
				if n := w.Node(c); n.Kind == tree.KindDir {
					keep[n.Name] = c
				}
			}
			mine := map[tree.ID]bool{b.dir: true}
			for _, id := range w.ClearOwn(b.dir) {
				mine[id] = true
			}
			own = func(id tree.ID) bool { return mine[id] }
		}
		shared := func(link bool, key uint64) bool {
			switch {
			case own != nil:
				return w.SharedElsewhere(link, key, own)
			case link:
				return w.Linked(key)
			default:
				return w.Cloned(key)
			}
		}
		join := func(link bool, key uint64, n uint32, alloc uint64, at tree.ID) {
			switch {
			case own != nil:
				w.Rejoin(link, key, n, alloc, at, own)
			case link:
				w.Link(key, n, alloc, at)
			default:
				w.Clone(key, n, alloc, at)
			}
		}
		for i := range b.entries {
			e := &b.entries[i]
			name := b.name(e)
			if e.err != 0 {
				s.problem(childPath(b.path, name), e.err)
				continue
			}
			if e.kind == tree.KindDir {
				if n := string(name); strings.HasSuffix(n, ".xcodeproj") || strings.HasSuffix(n, ".xcworkspace") {
					markers |= tree.MarkerXcode
				}
				if id, ok := keep[string(name)]; ok {
					delete(keep, string(name))
					if b.deep && !e.mount && e.flags&sfDataless == 0 {
						s.push(b, name, id, true)
					}
					continue
				}
				if e.mount {
					w.AddDir(b.dir, name, e.mtime, tree.FlagMount)
					continue
				}
				if e.flags&sfDataless != 0 {
					// An iCloud folder whose contents are not on this Mac:
					// listing it would download it.
					w.AddDir(b.dir, name, e.mtime, 0)
					continue
				}
				var df tree.Flag
				if e.ext&efIsSyncRoot != 0 {
					df = tree.FlagSyncRoot
				}
				s.push(b, name, w.AddDir(b.dir, name, e.mtime, df), false)
				continue
			}
			markers |= tree.MarkerBit(string(name))
			alloc := e.alloc
			var flags tree.Flag
			if e.flags&sfDataless != 0 {
				alloc = 0
			}
			big := e.alloc >= s.threshold
			linked := e.nlink > 1 && e.kind == tree.KindFile
			if linked && shared(true, e.ino) {
				alloc = 0
				flags |= tree.FlagShared
			}
			unique := alloc
			cloned := e.ext&efMayShareBlocks != 0 && alloc > 0 && e.cloneRefs > 1
			if e.ext&efMayShareBlocks != 0 && alloc > 0 {
				flags |= tree.FlagClone
			}
			if cloned && shared(false, e.cloneID) {
				unique = 0
			}
			files++
			bytes += alloc
			// Shared data is charged to the file's own node when it has one,
			// so a single file's reclaimable size accounts for its other copies.
			at := b.dir
			if big {
				at = w.AddFile(b.dir, name, e.kind, alloc, unique, e.logical, e.mtime, flags)
			}
			if linked {
				join(true, e.ino, e.nlink, e.alloc, at)
			}
			if cloned {
				join(false, e.cloneID, e.cloneRefs, e.alloc, at)
			}
			if big {
				continue
			}
			smallN++
			smallAlloc += alloc
			smallUnique += unique
			smallLogical += e.logical
			smallMtime = max(smallMtime, e.mtime)
		}
		if smallN > 0 {
			w.AddSmall(b.dir, smallN, smallAlloc, smallUnique, smallLogical, smallMtime)
		}
		if markers != 0 {
			w.SetMarkers(b.dir, markers)
		}
		for _, id := range keep {
			w.Remove(id)
			s.removed = true
		}
	})
	s.files.Add(files)
	s.bytes.Add(bytes)
}

// ownTrash: a disk's .Trashes cannot be listed, but the folder inside it
// that holds this user's trashed items can, by name.
func (s *Session) ownTrash(b *batch) bool {
	if filepath.Base(b.path) != ".Trashes" || !errors.Is(b.err, fs.ErrPermission) || s.t.Node(b.dir).Parent != s.t.Root() {
		return false
	}
	name := strconv.Itoa(os.Getuid())
	// Lstat: a link here (on a disk anyone could have prepared) is not followed.
	fi, err := os.Lstat(childPath(b.path, []byte(name)))
	if err != nil || !fi.IsDir() {
		return true
	}
	var id tree.ID
	known := false
	s.t.Write(func(w *tree.Writer) {
		for _, c := range w.Children(b.dir) {
			if w.Node(c).Name == name {
				id, known = c, true
				return
			}
		}
		id = w.AddDir(b.dir, []byte(name), fi.ModTime().Unix(), 0)
	})
	s.push(&batch{path: b.path}, []byte(name), id, known)
	return true
}

// push queues a subfolder of b's folder; refresh lists one already in the tree again, with everything below.
func (s *Session) push(b *batch, name []byte, id tree.ID, refresh bool) {
	s.pending++
	s.dirs.Add(1)
	if b.fd != nil {
		b.fd.refs.Add(1)
	}
	s.q.push(job{path: childPath(b.path, name), dir: id, parent: b.fd, refresh: refresh, deep: refresh})
}

func (s *Session) Tree() *tree.Tree { return s.t }

func (s *Session) Progress() Progress {
	p := Progress{Dirs: s.dirs.Load() + 1, Files: s.files.Load(), Bytes: s.bytes.Load(), Problems: s.nproblems.Load(), Elapsed: time.Since(s.start)}
	if c := s.current.Load(); c != nil {
		p.Path = *c
	}
	select {
	case <-s.done:
		p.Done, p.Elapsed = true, s.elapsed
	default:
	}
	return p
}

// Wait blocks until the scan ends; it returns ctx.Err() when it was cancelled.
func (s *Session) Wait() (*Result, error) {
	<-s.done
	res := &Result{Tree: s.t, Problems: s.problems, Elapsed: s.elapsed, Dirs: s.dirs.Load() + 1, Files: s.files.Load()}
	return res, s.ctx.Err()
}

// Scan runs a whole scan, reporting progress every interval when fn is set.
func Scan(ctx context.Context, opt Options, every time.Duration, fn func(Progress)) (*Result, error) {
	s, err := Start(ctx, opt)
	if err != nil {
		return nil, err
	}
	return s.follow(every, fn)
}

// follow waits for the session, reporting progress every interval when fn is set.
func (s *Session) follow(every time.Duration, fn func(Progress)) (*Result, error) {
	if fn != nil && every > 0 {
		tick := time.NewTicker(every)
		defer tick.Stop()
	loop:
		for {
			select {
			case <-s.done:
				break loop
			case <-tick.C:
				fn(s.Progress())
			}
		}
	}
	res, err := s.Wait()
	if fn != nil {
		fn(s.Progress())
	}
	return res, err
}
