package app

import (
	"context"
	"path/filepath"
	"slices"
	"time"

	"github.com/doguyilmaz/ebb/internal/detect"
	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/fsevents"
	"github.com/doguyilmaz/ebb/internal/history"
	"github.com/doguyilmaz/ebb/internal/rules"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
	"github.com/doguyilmaz/ebb/internal/walk"
)

type ScanOptions struct {
	Root    string
	Threads int
	Gentle  bool
	// Full reads every folder even when the folders changed since the last scan are known.
	Full bool
}

type ScanResult struct {
	Meta         history.Meta      `json:"meta"`
	Tree         *tree.Tree        `json:"-"`
	Volume       volume.Breakdown  `json:"volume"`
	Findings     []finding.Finding `json:"findings"`
	Problems     []walk.Problem    `json:"problems,omitempty"`
	ProblemCount int               `json:"problemCount"`
	Errors       []string          `json:"errors,omitempty"` // checks that could not run
}

const keepProblems = 1000

// Scan walks root, accounts for the volume, runs every detector and saves
// the result as the last scan and as a history snapshot.
func (a *App) Scan(ctx context.Context, opt ScanOptions, progress func(walk.Progress)) (*ScanResult, error) {
	root := opt.Root
	if root == "" {
		root = a.DefaultRoot()
	}
	if err := a.begin(a.Links.Display(root)); err != nil {
		return nil, err
	}
	res, err := a.scan(ctx, opt, progress)
	a.end(err)
	return res, err
}

func (a *App) scan(ctx context.Context, opt ScanOptions, progress func(walk.Progress)) (*ScanResult, error) {
	root := opt.Root
	if root == "" {
		root = a.DefaultRoot()
	}
	root, err := filepath.Abs(a.Links.Real(root))
	if err != nil {
		return nil, err
	}
	threads := opt.Threads
	if threads == 0 {
		threads = a.Config.Scan.Threads
	}
	wopt := walk.Options{Root: root, Threads: threads, Gentle: opt.Gentle}
	if !opt.Full {
		if out, ok, err := a.update(ctx, root, wopt, progress); ok {
			return out, err
		}
	}
	// Taken before the walk, so whatever changes while it runs is replayed by the next update.
	eventID, uuid := fsevents.Current(), fsevents.UUID(root)
	started := a.Env.Now()
	res, err := walk.Scan(ctx, wopt, 100*time.Millisecond, progress)
	if err != nil {
		return nil, err
	}
	meta := history.Meta{Elapsed: res.Elapsed, Dirs: res.Dirs, Files: res.Files, EventID: eventID, FSEvents: uuid}
	out, err := a.conclude(ctx, root, res.Tree, res.Problems, started, meta)
	if err != nil {
		return nil, err
	}
	a.rewatch(out)
	return out, nil
}

// update lists again only the folders changed since the last scan of root.
// ok is false when that is not known or not worth it; then the caller scans
// everything.
func (a *App) update(ctx context.Context, root string, opt walk.Options, progress func(walk.Progress)) (*ScanResult, bool, error) {
	last, err := a.Last()
	if err != nil || last.Tree.RootPath() != root {
		return nil, false, nil
	}
	set, from, ok := a.changesSince(ctx, last)
	if !ok {
		return nil, false, nil
	}
	started := a.Env.Now()
	res, err := walk.UpdateWith(ctx, last.Tree, set.dirs, set.subtrees, opt, 100*time.Millisecond, progress)
	if err != nil {
		from.putBack(set)
		return nil, true, err
	}
	problems := append(slices.Clone(last.Problems), res.Problems...)
	meta := history.Meta{Elapsed: res.Elapsed, Dirs: last.Meta.Dirs, Files: uint64(last.Tree.Node(last.Tree.Root()).Files),
		EventID: max(set.last, last.Meta.EventID), FSEvents: last.Meta.FSEvents, Incremental: true, Changed: res.Dirs}
	out, err := a.conclude(ctx, root, last.Tree, problems, started, meta)
	return out, true, err
}

// conclude turns a finished tree into a scan result: what the volume holds,
// the findings, a history snapshot and the saved last scan.
func (a *App) conclude(ctx context.Context, root string, t *tree.Tree, problems []walk.Problem, started time.Time, meta history.Meta) (*ScanResult, error) {
	vol, _ := volume.Inspect(ctx, a.Env, root)
	rootNode := t.Node(t.Root())
	scanned := rootNode.Alloc
	// Attribution uses unique bytes: allocated sizes count every clone copy,
	// which would hide part of what the scan could not see.
	if root == dataVolume || root == "/" {
		vol.Attribute(rootNode.Unique)
	} else {
		vol.Scanned = rootNode.Unique
	}
	var large rules.Size
	_ = large.UnmarshalText([]byte(a.Config.Scan.LargeFile))
	dc := &detect.Context{Tree: t, Volume: vol, Env: a.Env, Rules: a.Rules, History: a.History, Links: a.Links,
		Now: started, StaleAfter: time.Duration(a.Config.Scan.StaleDays) * 24 * time.Hour, LargeFile: uint64(large)}
	findings, errs := detect.Run(ctx, dc)
	var errText []string
	for _, e := range errs {
		errText = append(errText, e.Error())
	}
	meta.Time, meta.Root, meta.Used, meta.Free, meta.Scanned = started, a.Links.Display(root), vol.Used, vol.Free, scanned
	out := &ScanResult{Meta: meta, Tree: t, Volume: vol, Findings: findings, Problems: problems, ProblemCount: len(problems), Errors: errText}
	if len(out.Problems) > keepProblems {
		out.Problems = out.Problems[:keepProblems]
	}
	snap := &history.Snapshot{Meta: out.Meta, Volume: vol, Dirs: history.FromTree(t, 10<<20, a.Links.Display)}
	if err := a.History.Save(snap); err != nil {
		return nil, err
	}
	out.Meta.ID = snap.Meta.ID
	if err := a.saveLast(out); err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.cur = out
	a.mu.Unlock()
	return out, nil
}
