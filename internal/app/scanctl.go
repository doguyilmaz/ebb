package app

import (
	"context"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/walk"
)

func (a *App) begin(root string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		return ErrScanRunning
	}
	a.running, a.scanRoot, a.scanErr, a.progress = true, root, "", nil
	return nil
}

func (a *App) end(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.running = false
	if err != nil {
		a.scanErr = err.Error()
	}
}

// StartScan runs a scan in the background. progress is called every 100 ms
// and done once at the end.
func (a *App) StartScan(opt ScanOptions, progress func(api.Progress), done func(*ScanResult, error)) error {
	root := opt.Root
	if root == "" {
		root = a.DefaultRoot()
	}
	if err := a.begin(a.Links.Display(root)); err != nil {
		return err
	}
	go func() {
		res, err := a.scan(context.Background(), opt, func(p walk.Progress) {
			ap := api.Progress{Dirs: p.Dirs, Files: p.Files, Bytes: p.Bytes, Problems: p.Problems, Path: a.Links.Display(p.Path), ElapsedMs: p.Elapsed.Milliseconds()}
			a.mu.Lock()
			a.progress = &ap
			a.mu.Unlock()
			if progress != nil {
				progress(ap)
			}
		})
		a.end(err)
		if done != nil {
			done(res, err)
		}
	}()
	return nil
}

func (a *App) ScanState() api.ScanState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return api.ScanState{Running: a.running, Root: a.scanRoot, Progress: a.progress, Error: a.scanErr}
}

// Elapsed since the last scan finished, for staleness checks.
func (a *App) LastAge() time.Duration {
	r, err := a.Last()
	if err != nil {
		return -1
	}
	return a.Env.Now().Sub(r.Meta.Time)
}
