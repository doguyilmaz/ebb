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

// saveLast writes r as the last scan and returns last.json as written, so
// Last can tell a scan this process saved from one another ebb saved.
func (a *App) saveLast(r *ScanResult) (os.FileInfo, error) {
	var buf bytes.Buffer
	if err := r.Tree.Save(&buf); err != nil {
		return nil, err
	}
	if err := state.WriteAtomic(a.statePath("last.tree"), buf.Bytes(), 0o600); err != nil {
		return nil, err
	}
	if err := state.SaveJSON(a.statePath("last.json"), r); err != nil {
		return nil, err
	}
	return a.savedLast(), nil
}

// savedLast is last.json as it is on disk now; nil when it is not there.
func (a *App) savedLast() os.FileInfo {
	fi, err := os.Stat(a.statePath("last.json"))
	if err != nil {
		return nil
	}
	return fi
}

// sameSave reports whether two looks at last.json saw the same save. Every
// save writes a new file and renames it into place, so a newer save is
// another file.
func sameSave(x, y os.FileInfo) bool {
	return os.SameFile(x, y) && x.Size() == y.Size() && x.ModTime().Equal(y.ModTime())
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
//
// A scan or a cleanup run in a terminal saves its result from a process of
// its own, so the one in memory is read again once last.json is another save
// than the one it came from. Not while this process scans or cleans: that
// sets a result of its own, newer than what is on disk, and a cleanup edits
// the one in memory.
func (a *App) Last() (r *ScanResult, err error) {
	a.mu.Lock()
	// Looked at under the lock, so it is never older than a save this
	// process made and recorded meanwhile.
	saved := a.savedLast()
	cur := a.cur
	if cur != nil && (sameSave(saved, a.curFile) || a.running || a.cleaning) {
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
	reloaded := false
	defer func() {
		a.mu.Lock()
		a.loading = nil
		switch {
		case a.curGen != gen:
			// A scan finished during the read, and its result is newer.
			if a.cur != nil {
				l.r, l.err = a.cur, nil
			}
		case cur == nil:
			if l.err == nil {
				a.cur, a.curFile = l.r, saved
			}
		case l.err == nil && !a.running && !a.cleaning:
			a.cur, a.curFile, a.curGen = l.r, saved, a.curGen+1
			// Packages are kept per scan ID, and a cleanup in another
			// process saves what is left under the same ID.
			a.packages.scan = ""
			reloaded = true
		default:
			// The newer save could not be read, or a scan or cleanup began
			// here meanwhile: keep the result in memory.
			l.r, l.err = cur, nil
		}
		a.mu.Unlock()
		r, err = l.r, l.err
		close(l.done)
		if reloaded {
			// Follow changes from where the new scan left off, as a server
			// started now would.
			a.rewatch(l.r)
		}
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
