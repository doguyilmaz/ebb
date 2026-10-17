package app

import (
	"context"
	"path/filepath"
	"time"

	"github.com/doguyilmaz/ebb/internal/detect"
	"github.com/doguyilmaz/ebb/internal/finding"
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
}

type ScanResult struct {
	Meta         history.Meta      `json:"meta"`
	Tree         *tree.Tree        `json:"-"`
	Volume       volume.Breakdown  `json:"volume"`
	Findings     []finding.Finding `json:"findings"`
	Problems     []walk.Problem    `json:"problems,omitempty"`
	ProblemCount int               `json:"problemCount"`
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
	started := a.Env.Now()
	res, err := walk.Scan(ctx, walk.Options{Root: root, Threads: threads, Gentle: opt.Gentle}, 100*time.Millisecond, progress)
	if err != nil {
		return nil, err
	}
	vol, _ := volume.Inspect(ctx, a.Env, root)
	scanned := res.Tree.Node(res.Tree.Root()).Alloc
	if root == dataVolume || root == "/" {
		vol.Attribute(scanned)
	} else {
		vol.Scanned = scanned
	}
	var large rules.Size
	_ = large.UnmarshalText([]byte(a.Config.Scan.LargeFile))
	dc := &detect.Context{Tree: res.Tree, Volume: vol, Env: a.Env, Rules: a.Rules, History: a.History, Links: a.Links,
		Now: started, StaleAfter: time.Duration(a.Config.Scan.StaleDays) * 24 * time.Hour, LargeFile: uint64(large)}
	findings, _ := detect.Run(ctx, dc)

	out := &ScanResult{
		Meta: history.Meta{Time: started, Root: a.Links.Display(root), Used: vol.Used, Free: vol.Free, Scanned: scanned,
			Elapsed: res.Elapsed, Dirs: res.Dirs, Files: res.Files},
		Tree: res.Tree, Volume: vol, Findings: findings, Problems: res.Problems, ProblemCount: len(res.Problems),
	}
	if len(out.Problems) > keepProblems {
		out.Problems = out.Problems[:keepProblems]
	}
	snap := &history.Snapshot{Meta: out.Meta, Volume: vol, Dirs: history.FromTree(res.Tree, 10<<20, a.Links.Display)}
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
