package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/apps"
	"github.com/doguyilmaz/ebb/internal/clean"
	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/tree"
)

type planEntry struct {
	plan *clean.Plan
	ran  bool
}

var ErrNoPlan = errors.New("no such plan (plans expire after 10 minutes)")

func (a *App) OpLog() *clean.OpLog { return clean.OpenOpLog(sys.StateDir(a.Env)) }

func (a *App) guard(r *ScanResult) clean.Guard {
	return clean.Guard{Home: a.Env.Home(), Roots: []string{r.Tree.RootPath()}, Protect: a.reloadConfig().Clean.Protect, Keep: clean.OwnDirs(a.Env)}
}

func (a *App) Plan(req clean.Request) (*clean.Plan, error) {
	r, err := a.Last()
	if err != nil {
		return nil, err
	}
	cfg := a.reloadConfig()
	pl := &clean.Planner{Env: a.Env, Tree: r.Tree, Links: a.Links, Findings: a.View(r).Findings, Rules: a.Rules,
		Protect: cfg.Clean.Protect, Roots: []string{r.Tree.RootPath()}, AutoExecute: cfg.MCP.AutoExecute}
	if req.App != "" {
		pl.Apps, _ = apps.Source{Env: a.Env, Tree: r.Tree, Links: a.Links}.Installed(context.Background())
	}
	p, err := pl.Plan(req)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.plans == nil {
		a.plans = map[string]*planEntry{}
	}
	now := a.Env.Now()
	for id, e := range a.plans {
		if now.After(e.plan.Expires) {
			delete(a.plans, id)
		}
	}
	a.plans[p.ID] = &planEntry{plan: p}
	return p, nil
}

func (a *App) GetPlan(id string) (*clean.Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	e, ok := a.plans[id]
	if !ok {
		return nil, ErrNoPlan
	}
	p := *e.plan
	return &p, nil
}

// Pending lists plans made elsewhere (an agent, the CLI) that wait for the
// person's approval, oldest first.
func (a *App) Pending() []clean.Plan {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.Env.Now()
	out := []clean.Plan{}
	for _, e := range a.plans {
		if p := *e.plan; p.NeedsApproval && !p.Approved && !e.ran && p.Source != "ui" && now.Before(p.Expires) {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(x, y clean.Plan) int { return x.Created.Compare(y.Created) })
	return out
}

// Approve records the person's yes for a plan made elsewhere (the MCP server).
func (a *App) Approve(id string) (*clean.Plan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	e, ok := a.plans[id]
	if !ok {
		return nil, ErrNoPlan
	}
	e.plan.Approved = true
	p := *e.plan
	return &p, nil
}

// Execute runs a plan once, then updates the saved scan so every view shows
// what is left.
func (a *App) Execute(ctx context.Context, id string, fn func(clean.Event)) (clean.Result, error) {
	// A scan edits the tree this updates afterwards, and two cleanups would
	// race on it and on the findings: one thing at a time, settled under one lock.
	a.mu.Lock()
	e, ok := a.plans[id]
	switch {
	case !ok:
		a.mu.Unlock()
		return clean.Result{}, ErrNoPlan
	case e.ran:
		a.mu.Unlock()
		return clean.Result{}, fmt.Errorf("plan %s already ran", id)
	case a.running:
		a.mu.Unlock()
		return clean.Result{}, fmt.Errorf("%w: clean when it is done", ErrScanRunning)
	case a.cleaning:
		a.mu.Unlock()
		return clean.Result{}, ErrCleaning
	case a.dupes.running:
		a.mu.Unlock()
		return clean.Result{}, ErrSearching
	}
	a.cleaning = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.cleaning = false
		a.mu.Unlock()
	}()
	r, err := a.Last()
	if err != nil {
		return clean.Result{}, err
	}
	ex := &clean.Executor{Env: a.Env, Log: a.OpLog(), Trash: clean.NewTrasher(a.Env), Guard: a.guard(r), Links: a.Links}
	res, err := ex.Execute(ctx, e.plan, fn)
	if errors.Is(err, clean.ErrNeedsApproval) || errors.Is(err, clean.ErrExpired) {
		return res, err
	}
	a.mu.Lock()
	e.ran = true
	a.mu.Unlock()
	if uerr := a.afterClean(r, res); uerr != nil && err == nil {
		err = uerr
	}
	return res, err
}

// afterClean removes what is gone from the tree and the findings.
func (a *App) afterClean(r *ScanResult, res clean.Result) error {
	var gone []string
	for _, op := range res.Ops {
		if op.Result != "done" {
			continue
		}
		real := a.Links.Real(op.Path)
		if op.Action == "empty-trash" {
			if id, ok := r.Tree.Lookup(real); ok {
				kids := r.Tree.Children(id)
				r.Tree.Write(func(w *tree.Writer) {
					for _, k := range kids {
						w.Remove(k)
					}
				})
			}
			continue
		}
		if _, err := os.Lstat(real); err == nil {
			continue
		}
		if id, ok := r.Tree.Lookup(real); ok {
			r.Tree.Write(func(w *tree.Writer) { w.Remove(id) })
		}
		gone = append(gone, op.Path)
	}
	a.mu.Lock()
	var kept []finding.Finding
	for _, f := range r.Findings {
		had := len(f.Targets) > 0
		f.Targets = slices.DeleteFunc(slices.Clone(f.Targets), func(t finding.Target) bool {
			return slices.ContainsFunc(gone, func(g string) bool { return within(t.Path, g) })
		})
		if had && len(f.Targets) == 0 {
			continue
		}
		f.Total()
		kept = append(kept, f)
	}
	r.Findings = kept
	a.mu.Unlock()
	a.forgetCopies(gone)
	r.Tree.Compact()
	return a.saveLast(r)
}

func (a *App) Ops(limit int) ([]clean.Op, error) { return a.OpLog().List(limit) }

// Undo puts a trashed item back; the empty op ID means the latest one.
func (a *App) Undo(opID string) error {
	if opID == "" {
		ops, err := a.Ops(0)
		if err != nil {
			return err
		}
		for _, op := range ops {
			if op.Action == "trash" && op.Result == "done" {
				opID = op.ID
				break
			}
		}
		if opID == "" {
			return errors.New("nothing to undo")
		}
	}
	return clean.Undo(a.Env, a.OpLog(), opID)
}

// Presets lists the built-in and custom presets with what each would free now.
func (a *App) Presets() []api.PresetView {
	var fs []finding.Finding
	if r, err := a.Last(); err == nil {
		fs = a.View(r).Findings
	}
	out := []api.PresetView{}
	for _, p := range a.Rules.Presets {
		v := api.PresetView{Preset: p}
		for _, f := range fs {
			if clean.PresetMatches(p, f) {
				v.Bytes += f.Bytes
			}
		}
		out = append(out, v)
	}
	return out
}
