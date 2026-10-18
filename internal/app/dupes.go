package app

import (
	"context"
	"errors"
	"slices"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/dupes"
	"github.com/doguyilmaz/ebb/internal/state"
	"github.com/doguyilmaz/ebb/internal/tree"
)

var ErrSearching = errors.New("a duplicate search is running: try again when it is done")

// dupesRoot is where a search looks when no folder is given: the person's own files.
// dupesRoot is the folder to search: the one asked for, else home, else (when
// the last scan was of another disk or a folder outside home) that scan's root.
func (a *App) dupesRoot(path string) string {
	if path != "" {
		return path
	}
	home := a.Env.Home()
	if r, err := a.Last(); err == nil && !within(a.Links.Real(home), a.Links.Real(r.Meta.Root)) {
		return r.Meta.Root
	}
	return home
}

// FindDuplicates searches below path (empty: home) and waits for the result.
func (a *App) FindDuplicates(ctx context.Context, path string, minSize uint64, progress func(dupes.Progress)) (*dupes.Result, error) {
	if err := a.beginSearch(); err != nil {
		return nil, err
	}
	r, id, err := a.Resolve(a.dupesRoot(path))
	if err != nil {
		a.endSearch(nil, nil)
		return nil, err
	}
	res, err := a.findDuplicates(ctx, r, id, minSize, progress)
	a.endSearch(res, err)
	return res, err
}

// StartDuplicates searches in the background; done gets the result.
func (a *App) StartDuplicates(path string, minSize uint64, progress func(dupes.Progress), done func(*dupes.Result, error)) error {
	if err := a.beginSearch(); err != nil {
		return err
	}
	// Resolved once the search holds its turn, so no scan moves the tree in between.
	r, id, err := a.Resolve(a.dupesRoot(path))
	if err != nil {
		a.endSearch(nil, nil)
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.dupes.stop = cancel
	a.mu.Unlock()
	go func() {
		res, err := a.findDuplicates(ctx, r, id, minSize, func(p dupes.Progress) {
			a.mu.Lock()
			a.dupes.progress = &p
			a.mu.Unlock()
			if progress != nil {
				progress(p)
			}
		})
		cancel()
		a.endSearch(res, err)
		if done != nil {
			done(res, err)
		}
	}()
	return nil
}

func (a *App) findDuplicates(ctx context.Context, r *ScanResult, id tree.ID, minSize uint64, progress func(dupes.Progress)) (*dupes.Result, error) {
	res, err := dupes.Find(ctx, r.Tree, a.Links, id, max(minSize, 1<<20), progress)
	if err != nil {
		return nil, err
	}
	if err := state.SaveJSON(a.statePath("duplicates.json"), res); err != nil {
		return nil, err
	}
	return &res, nil
}

// beginSearch: a search reads the tree that scans and cleanups edit, so one
// thing runs at a time.
func (a *App) beginSearch() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case a.dupes.running:
		return ErrSearching
	case a.running:
		return ErrScanRunning
	case a.cleaning:
		return ErrCleaning
	}
	a.dupes.running, a.dupes.progress, a.dupes.err, a.dupes.stopped = true, nil, "", false
	return nil
}

func (a *App) endSearch(res *dupes.Result, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dupes.running, a.dupes.stop = false, nil
	if res != nil {
		a.dupes.last = res
	}
	switch {
	case errors.Is(err, context.Canceled):
		a.dupes.stopped = true
	case err != nil:
		a.dupes.err = err.Error()
	}
}

// StopDuplicates cancels the background search, if one runs.
func (a *App) StopDuplicates() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.dupes.stop == nil {
		return false
	}
	a.dupes.stop()
	return true
}

// Duplicates is the search as the window shows it: running or not, and the
// last result (also from before a restart).
func (a *App) Duplicates() api.DuplicatesState {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.dupes.last == nil && !a.dupes.loaded {
		a.dupes.loaded = true
		var res dupes.Result
		if state.LoadJSON(a.statePath("duplicates.json"), &res) == nil {
			a.dupes.last = &res
		}
	}
	return api.DuplicatesState{Running: a.dupes.running, Stopped: a.dupes.stopped, Progress: a.dupes.progress, Result: a.dupes.last, Error: a.dupes.err}
}

// forgetCopies drops copies a cleanup removed, and groups left with one.
func (a *App) forgetCopies(gone []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.dupes.last == nil || len(gone) == 0 {
		return
	}
	res := *a.dupes.last
	res.Groups, res.Wasted = nil, 0
	for _, g := range a.dupes.last.Groups {
		g.Copies = slices.DeleteFunc(slices.Clone(g.Copies), func(c dupes.Copy) bool {
			return slices.ContainsFunc(gone, func(p string) bool { return within(c.Path, p) })
		})
		if len(g.Copies) < 2 {
			continue
		}
		g.Wasted = 0
		for _, c := range g.Copies[1:] {
			g.Wasted += c.Bytes
		}
		res.Groups = append(res.Groups, g)
		res.Wasted += g.Wasted
	}
	a.dupes.last = &res
	_ = state.SaveJSON(a.statePath("duplicates.json"), res)
}
