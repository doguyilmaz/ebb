package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/doguyilmaz/ebb/internal/state"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/walk"
)

func (a *App) saveLast(r *ScanResult) error {
	var buf bytes.Buffer
	if err := r.Tree.Save(&buf); err != nil {
		return err
	}
	if err := state.WriteAtomic(a.statePath("last.tree"), buf.Bytes(), 0o600); err != nil {
		return err
	}
	return state.SaveJSON(a.statePath("last.json"), r)
}

// lastLoad is a read of the saved scan under way, shared by every caller that
// needs it meanwhile.
type lastLoad struct {
	done chan struct{}
	r    *ScanResult
	err  error
}

// Last returns the most recent scan, from memory or from the state folder.
// Callers that arrive while the saved scan is read wait for that read instead
// of loading a copy of their own: the tree of a whole disk takes ~100 MB in
// memory, and the window asks for several things at once when it opens.
func (a *App) Last() (r *ScanResult, err error) {
	a.mu.Lock()
	if cur := a.cur; cur != nil {
		a.mu.Unlock()
		return cur, nil
	}
	if l := a.loading; l != nil {
		a.mu.Unlock()
		<-l.done
		return l.r, l.err
	}
	// The error is what waiters get if the read panics.
	l := &lastLoad{done: make(chan struct{}), err: ErrNeverScanned}
	a.loading = l
	gen := a.curGen
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.loading = nil
		switch {
		case a.curGen != gen && a.cur != nil:
			// A scan finished during the read, and its result is newer.
			l.r, l.err = a.cur, nil
		case a.curGen == gen && l.err == nil:
			a.cur = l.r
		}
		a.mu.Unlock()
		r, err = l.r, l.err
		close(l.done)
	}()
	l.r, l.err = readLast(a)
	return l.r, l.err
}

// readLast reads the saved scan from the state folder. A variable so a test
// can hold a read open while other callers arrive.
var readLast = func(a *App) (*ScanResult, error) {
	var r ScanResult
	if err := state.LoadJSON(a.statePath("last.json"), &r); err != nil {
		if state.IsNotExist(err) {
			return nil, ErrNeverScanned
		}
		return nil, fmt.Errorf("%w (the saved scan could not be read: %v)", ErrNeverScanned, err)
	}
	f, err := os.Open(a.statePath("last.tree"))
	if err != nil {
		return nil, ErrNeverScanned
	}
	defer f.Close()
	if r.Tree, err = tree.Load(f); err != nil {
		return nil, fmt.Errorf("%w (the saved scan could not be read: %v)", ErrNeverScanned, err)
	}
	return &r, nil
}

// Current reuses the last scan when it covers the same root and is younger
// than maxAge, and scans otherwise.
func (a *App) Current(ctx context.Context, maxAge time.Duration, opt ScanOptions, progress func(walk.Progress)) (*ScanResult, error) {
	root := opt.Root
	if root == "" {
		root = a.DefaultRoot()
	}
	root, _ = filepath.Abs(a.Links.Real(root))
	if last, err := a.Last(); err == nil && a.Links.Display(last.Meta.Root) == a.Links.Display(root) && a.Env.Now().Sub(last.Meta.Time) < maxAge {
		return last, nil
	}
	return a.Scan(ctx, opt, progress)
}
