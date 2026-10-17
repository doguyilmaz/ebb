package app

import (
	"bytes"
	"context"
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

// Last returns the most recent scan, from memory or from the state folder.
func (a *App) Last() (*ScanResult, error) {
	a.mu.Lock()
	cur := a.cur
	a.mu.Unlock()
	if cur != nil {
		return cur, nil
	}
	var r ScanResult
	if err := state.LoadJSON(a.statePath("last.json"), &r); err != nil {
		if state.IsNotExist(err) {
			return nil, ErrNeverScanned
		}
		return nil, err
	}
	f, err := os.Open(a.statePath("last.tree"))
	if err != nil {
		return nil, ErrNeverScanned
	}
	defer f.Close()
	if r.Tree, err = tree.Load(f); err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.cur = &r
	a.mu.Unlock()
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
