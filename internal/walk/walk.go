// Package walk scans a directory tree in parallel. Workers only read
// directories; one builder goroutine owns every write to the tree, so the
// arena needs no fine-grained locking and readers can watch it fill live.
package walk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	Markers        []string
}

var DefaultMarkers = []string{"package.json", "Cargo.toml", "go.mod", "build.gradle", "build.gradle.kts",
	"settings.gradle", "Podfile", "pubspec.yaml", "pyproject.toml", "requirements.txt", "Gemfile", "composer.json"}

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
	efMayShareBlocks = 0x1
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
}

func (b *batch) name(e *entry) []byte { return b.names[e.nameOff : e.nameOff+uint32(e.nameLen)] }

func (b *batch) addName(name string) (uint32, uint16) {
	off := uint32(len(b.names))
	b.names = append(b.names, name...)
	return off, uint16(len(name))
}

type reader interface {
	readDir(path string, b *batch) error
}

type job struct {
	path string
	dir  tree.ID
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
	markers   map[string]bool
	threshold uint64
	start     time.Time

	dirs, files, bytes, nproblems atomic.Uint64
	current                       atomic.Pointer[string]

	pending  int
	problems []Problem
	done     chan struct{}
	elapsed  time.Duration
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
	if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}
	if opt.Threads <= 0 {
		opt.Threads = min(8, runtime.NumCPU())
	}
	if opt.SmallThreshold == 0 {
		opt.SmallThreshold = 1 << 20
	}
	if opt.Markers == nil {
		opt.Markers = DefaultMarkers
	}
	s := &Session{
		ctx: ctx, opt: opt, t: tree.New(root), q: newQueue(), batches: make(chan *batch, 256),
		markers: map[string]bool{}, threshold: opt.SmallThreshold, start: time.Now(), done: make(chan struct{}),
	}
	for _, m := range opt.Markers {
		s.markers[m] = true
	}
	s.pool.New = func() any { return &batch{} }
	setIOPolicy(opt.Gentle)
	r := newReader(opt.Reader)

	var workers sync.WaitGroup
	for range opt.Threads {
		workers.Go(func() { s.work(r) })
	}
	go func() {
		workers.Wait()
		close(s.batches)
	}()
	s.pending = 1
	s.q.push(job{root, s.t.Root()})
	go s.build()
	return s, nil
}

func (s *Session) work(r reader) {
	for {
		j, ok := s.q.pop()
		if !ok {
			return
		}
		b := s.pool.Get().(*batch)
		b.dir, b.path, b.entries, b.names, b.err = j.dir, j.path, b.entries[:0], b.names[:0], nil
		if err := s.ctx.Err(); err != nil {
			b.err = err
		} else {
			b.err = r.readDir(j.path, b)
		}
		s.batches <- b
	}
}

func (s *Session) build() {
	for b := range s.batches {
		s.apply(b)
		s.pool.Put(b)
		s.pending--
		if s.pending == 0 {
			s.q.close()
		}
	}
	s.t.Finish()
	s.elapsed = time.Since(s.start)
	close(s.done)
}

func childPath(dir string, name []byte) string {
	if strings.HasSuffix(dir, "/") {
		return dir + string(name)
	}
	return dir + "/" + string(name)
}

func (s *Session) problem(path string, err error) {
	s.problems = append(s.problems, Problem{Path: path, Err: err.Error()})
	s.nproblems.Add(1)
}

func (s *Session) apply(b *batch) {
	if b.err != nil {
		if errors.Is(b.err, context.Canceled) || errors.Is(b.err, context.DeadlineExceeded) {
			return
		}
		s.problem(b.path, b.err)
		s.t.Write(func(w *tree.Writer) { w.SetFlags(b.dir, tree.FlagDenied) })
		return
	}
	p := b.path
	s.current.Store(&p)
	var files, bytes uint64
	s.t.Write(func(w *tree.Writer) {
		var smallN uint32
		var smallAlloc, smallUnique, smallLogical uint64
		var smallMtime int64
		project := false
		for i := range b.entries {
			e := &b.entries[i]
			name := b.name(e)
			if e.err != 0 {
				s.problem(childPath(b.path, name), e.err)
				continue
			}
			if e.kind == tree.KindDir {
				if e.mount {
					w.AddDir(b.dir, name, e.mtime, tree.FlagMount)
					continue
				}
				if n := string(name); strings.HasSuffix(n, ".xcodeproj") || strings.HasSuffix(n, ".xcworkspace") {
					project = true
				}
				id := w.AddDir(b.dir, name, e.mtime, 0)
				s.pending++
				s.dirs.Add(1)
				s.q.push(job{childPath(b.path, name), id})
				continue
			}
			if s.markers[string(name)] {
				project = true
			}
			alloc := e.alloc
			var flags tree.Flag
			if e.flags&sfDataless != 0 {
				alloc = 0
			}
			big := e.alloc >= s.threshold
			if e.nlink > 1 && e.kind == tree.KindFile && !w.Link(e.ino, e.nlink, alloc, b.dir) {
				alloc = 0
				flags |= tree.FlagShared
			}
			unique := alloc
			if e.ext&efMayShareBlocks != 0 && alloc > 0 {
				flags |= tree.FlagClone
				if e.cloneRefs > 1 && !w.Clone(e.cloneID, e.cloneRefs, alloc, b.dir) {
					unique = 0
				}
			}
			files++
			bytes += alloc
			if big {
				w.AddFile(b.dir, name, e.kind, alloc, unique, e.logical, e.mtime, flags)
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
		if project {
			w.SetFlags(b.dir, tree.FlagProject)
		}
	})
	s.files.Add(files)
	s.bytes.Add(bytes)
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
