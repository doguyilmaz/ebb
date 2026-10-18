package app

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/history"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

var (
	ErrOutsideRoot = errors.New("path is outside the scanned folder")
	ErrNotInScan   = errors.New("path is not in the last scan")
)

func within(p, root string) bool {
	return p == root || strings.HasPrefix(p, strings.TrimSuffix(root, "/")+"/")
}

// Resolve checks a path from a client (display or real form) against the last
// scan and returns its node. Anything not absolute, carrying a NUL or lying
// outside the scanned root is refused before the tree is consulted.
func (a *App) Resolve(path string) (*ScanResult, tree.ID, error) {
	last, err := a.Last()
	if err != nil {
		return nil, 0, err
	}
	if path == "" {
		return last, last.Tree.Root(), nil
	}
	if strings.ContainsRune(path, 0) || !filepath.IsAbs(path) {
		return nil, 0, ErrOutsideRoot
	}
	real := a.Links.Real(filepath.Clean(path))
	if !within(real, last.Tree.RootPath()) {
		return nil, 0, ErrOutsideRoot
	}
	id, ok := last.Tree.Lookup(real)
	if !ok {
		return nil, 0, ErrNotInScan
	}
	return last, id, nil
}

func kindName(k tree.Kind) string {
	return [...]string{"dir", "file", "link", "other", "small"}[k]
}

func (a *App) item(r *ScanResult, id tree.ID, notes map[string]string) api.Item {
	t := r.Tree
	n := t.Node(id)
	path := a.Links.Display(t.Path(id))
	it := api.Item{Path: path, Name: n.Name, Kind: kindName(n.Kind), Alloc: n.Alloc, Bytes: t.Reclaimable(id), Files: n.Files, Modified: n.MaxMtime,
		Note: notes["path:"+path]}
	for _, f := range a.findings(r) {
		if slices.ContainsFunc(f.Targets, func(tg finding.Target) bool { return within(tg.Path, path) }) {
			it.Findings = append(it.Findings, f.ID)
		}
	}
	return it
}

func (a *App) Slice(path string, depth int, minShare float64, maxNodes int) (api.SliceResponse, error) {
	r, id, err := a.Resolve(path)
	if err != nil {
		return api.SliceResponse{}, err
	}
	return api.SliceResponse{Path: a.Links.Display(r.Tree.Path(id)), Root: r.Tree.Slice(id, depth, minShare, maxNodes)}, nil
}

var ageBuckets = []api.AgeBucket{{Label: "Past week", Days: 7}, {Label: "Past month", Days: 30}, {Label: "Past 3 months", Days: 91},
	{Label: "Past year", Days: 365}, {Label: "1–2 years", Days: 730}, {Label: "Older"}}

// Ages splits a folder's bytes by how long before the scan they last changed.
func (a *App) Ages(path string) (api.AgesResponse, error) {
	r, id, err := a.Resolve(path)
	if err != nil {
		return api.AgesResponse{}, err
	}
	const day = 86400
	cuts := make([]int64, 0, len(ageBuckets)-1)
	for _, b := range ageBuckets[:len(ageBuckets)-1] {
		cuts = append(cuts, int64(b.Days)*day)
	}
	got := r.Tree.Ages(id, r.Meta.Time.Unix(), cuts)
	out := api.AgesResponse{Path: a.Links.Display(r.Tree.Path(id)), Buckets: slices.Clone(ageBuckets)}
	for i := range out.Buckets {
		out.Buckets[i].Bytes = got[i]
	}
	return out, nil
}

// Explain gathers what ebb knows about one path for people and agents alike.
func (a *App) Explain(path string) (api.Explain, error) {
	r, id, err := a.Resolve(path)
	if err != nil {
		return api.Explain{}, err
	}
	notes := a.Notes()
	out := api.Explain{Item: a.item(r, id, notes), Findings: []api.FindingView{}, Biggest: []api.Item{}}
	display := out.Item.Path
	ig := a.ignores()
	for _, f := range a.findings(r) {
		if slices.ContainsFunc(f.Targets, func(tg finding.Target) bool { return within(tg.Path, display) || within(display, tg.Path) }) {
			f.UserNote = notes["finding:"+f.ID]
			out.Findings = append(out.Findings, api.FindingView{Finding: f, Ignored: ignored(ig, f)})
		}
	}
	kids := r.Tree.Children(id)
	for _, c := range kids[:min(10, len(kids))] {
		out.Biggest = append(out.Biggest, a.item(r, c, notes))
	}
	return out, nil
}

// Files lists the largest stored files under path, optionally only those
// untouched for olderThanDays before the scan.
func (a *App) Files(path string, minSize uint64, olderThanDays, limit int) (api.FilesResponse, error) {
	r, id, err := a.Resolve(path)
	if err != nil {
		return api.FilesResponse{}, err
	}
	cutoff := r.Meta.Time.Add(-time.Duration(olderThanDays) * 24 * time.Hour).Unix()
	keep := func(alloc uint64, mtime int64) bool {
		return alloc >= minSize && (olderThanDays <= 0 || mtime < cutoff)
	}
	notes := a.Notes()
	out := api.FilesResponse{Path: a.Links.Display(r.Tree.Path(id)), Items: []api.Item{}}
	for _, f := range r.Tree.TopFunc(id, limit, keep) {
		out.Items = append(out.Items, a.item(r, f, notes))
	}
	return out, nil
}

// Status is what the UI and agents see first: version, scan state, the last scan and what changed since.
func (a *App) Status(version string) api.Status {
	st := api.Status{Version: version, Scan: a.ScanState(), DefaultRoot: a.Links.Display(a.DefaultRoot()),
		FullDiskAccess: volume.HasFullDiskAccess(a.Env.Home()), Sandboxed: sys.Sandboxed(a.Env), Changes: a.Changes(), AskAvailable: a.AskAvailable()}
	if last, err := a.Last(); err == nil {
		m := last.Meta
		st.Last = &m
	}
	return st
}

// Children lists a folder's stored children and, when asked, the small files
// folded into it, read live from disk because the scan does not keep them.
func (a *App) Children(path string, limit int, small bool) (api.ChildrenResponse, error) {
	r, id, err := a.Resolve(path)
	if err != nil {
		return api.ChildrenResponse{}, err
	}
	t := r.Tree
	out := api.ChildrenResponse{Path: a.Links.Display(t.Path(id)), Items: []api.Item{}}
	stored := map[string]bool{}
	notes := a.Notes()
	for _, c := range t.Children(id) {
		stored[t.Node(c).Name] = true
		if len(out.Items) < limit {
			out.Items = append(out.Items, a.item(r, c, notes))
		}
	}
	if small && t.Node(id).Kind == tree.KindDir {
		des, _ := os.ReadDir(t.Path(id))
		for _, de := range des {
			if de.IsDir() || stored[de.Name()] {
				continue
			}
			info, err := de.Info()
			if err != nil {
				continue
			}
			out.Small = append(out.Small, api.SmallFile{Name: de.Name(), Size: uint64(info.Size()), Modified: info.ModTime().Unix()})
		}
		slices.SortFunc(out.Small, func(x, y api.SmallFile) int { return cmp.Compare(y.Size, x.Size) })
		out.Small = out.Small[:min(len(out.Small), limit)]
	}
	return out, nil
}

func (a *App) Search(q string, limit int) (api.SearchResponse, error) {
	r, err := a.Last()
	if err != nil {
		return api.SearchResponse{}, err
	}
	out := api.SearchResponse{Items: []api.Item{}}
	ids := r.Tree.Find(q, 100000)
	slices.SortFunc(ids, func(x, y tree.ID) int { return cmp.Compare(r.Tree.Node(y).Alloc, r.Tree.Node(x).Alloc) })
	notes := a.Notes()
	for _, id := range ids[:min(limit, len(ids))] {
		out.Items = append(out.Items, a.item(r, id, notes))
	}
	return out, nil
}

// Findings lists findings with notes attached; ignored ones only when asked,
// flagged, and never counted as reclaimable.
func (a *App) Findings(category string, minBytes uint64, withIgnored bool) (api.FindingsResponse, error) {
	r, err := a.Last()
	if err != nil {
		return api.FindingsResponse{}, err
	}
	out := api.FindingsResponse{Findings: []api.FindingView{}}
	notes, ig := a.Notes(), a.ignores()
	for _, f := range a.findings(r) {
		if (category != "" && f.Category != category) || f.Bytes < minBytes {
			continue
		}
		f.UserNote = notes["finding:"+f.ID]
		v := api.FindingView{Finding: f, Ignored: ignored(ig, f)}
		if v.Ignored && !withIgnored {
			continue
		}
		out.Findings = append(out.Findings, v)
		if f.Actionable() && !v.Ignored {
			out.Reclaimable += f.Bytes
		}
	}
	return out, nil
}

func (a *App) Volume() (volume.Breakdown, error) {
	r, err := a.Last()
	if err != nil {
		return volume.Breakdown{}, err
	}
	return r.Volume, nil
}

// Reveal shows a scanned path in Finder.
func (a *App) Reveal(path string) error {
	r, id, err := a.Resolve(path)
	if err != nil {
		return err
	}
	if sys.Sandboxed(a.Env) {
		return nil
	}
	return exec.Command("open", "-R", r.Tree.Path(id)).Run()
}

var choosePrompts = map[string]string{"scan": "Choose a folder for ebb to scan", "protect": "Choose a folder ebb must never touch"}

// ChooseFolder shows the macOS folder chooser on this Mac and returns the
// picked folder, or "" when the person cancels. A browser cannot hand a page
// a real path, so the page asks ebb. The prompt is fixed per purpose; nothing
// from the request reaches the script.
func (a *App) ChooseFolder(ctx context.Context, purpose string) (string, error) {
	prompt, ok := choosePrompts[purpose]
	if !ok {
		return "", fmt.Errorf("unknown purpose %q", purpose)
	}
	out, err := a.Env.Run(ctx, "", "osascript", "-e", "activate", "-e", `POSIX path of (choose folder with prompt "`+prompt+`")`)
	var exit *exec.ExitError
	if errors.As(err, &exit) && bytes.Contains(exit.Stderr, []byte("(-128)")) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(string(out))
	if len(p) > 1 {
		p = strings.TrimSuffix(p, "/")
	}
	return a.Links.Display(p), nil
}

// Diff compares two snapshots; empty IDs mean the newest one and the scan of
// the same folder before it.
func (a *App) Diff(fromID, toID string, minDelta uint64) (api.DiffResponse, error) {
	metas, err := a.History.List()
	if err != nil {
		return api.DiffResponse{}, err
	}
	if len(metas) == 0 {
		return api.DiffResponse{}, ErrNeverScanned
	}
	if toID == "" {
		toID = metas[0].ID
	}
	to, err := a.History.Load(toID)
	if err != nil {
		return api.DiffResponse{}, ErrNotInScan
	}
	var from *history.Snapshot
	if fromID != "" {
		if from, err = a.History.Load(fromID); err != nil {
			return api.DiffResponse{}, ErrNotInScan
		}
	} else if from, err = a.History.Previous(to.Meta.Root, to.Meta.Time); err != nil {
		return api.DiffResponse{To: to.Meta, Changes: []history.Change{}}, nil
	}
	changes := history.Explaining(history.Diff(from, to, minDelta))
	if changes == nil {
		changes = []history.Change{}
	}
	return api.DiffResponse{From: &from.Meta, To: to.Meta, Changes: changes}, nil
}
