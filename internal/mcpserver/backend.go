package mcpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/clean"
	"github.com/doguyilmaz/ebb/internal/dupes"
	"github.com/doguyilmaz/ebb/internal/remote"
	"github.com/doguyilmaz/ebb/internal/report"
	"github.com/doguyilmaz/ebb/internal/volume"
)

// Backend is what the tools need from ebb. The remote one talks to a running
// server, so an agent and the UI share one scan and one set of plans; the
// local one runs the engine in this process when nothing is serving.
type Backend interface {
	Status(ctx context.Context) (api.Status, error)
	Scan(ctx context.Context, req api.ScanRequest, wait bool) (api.Status, error)
	Volume(ctx context.Context) (volume.Breakdown, error)
	Findings(ctx context.Context, category string, minBytes uint64) (api.FindingsResponse, error)
	Children(ctx context.Context, path string, limit int) (api.ChildrenResponse, error)
	Files(ctx context.Context, path string, minSize uint64, olderThanDays, limit int) (api.FilesResponse, error)
	Search(ctx context.Context, q string) (api.SearchResponse, error)
	Explain(ctx context.Context, path string) (api.Explain, error)
	Presets(ctx context.Context) ([]api.PresetView, error)
	Apps(ctx context.Context) (api.AppsResponse, error)
	Duplicates(ctx context.Context, path string, minSize uint64) (*dupes.Result, error)
	Diff(ctx context.Context, from, to string) (api.DiffResponse, error)
	Plan(ctx context.Context, req clean.Request) (*clean.Plan, error)
	Execute(ctx context.Context, planID string) (clean.Result, error)
	Undo(ctx context.Context, opID string) error
	Note(ctx context.Context, key, text string) error
	Report(ctx context.Context, format string) (string, error)
}

// errNeedsApproval: agents never approve a cleanup; the person does.
func errNeedsApproval(id string) error {
	return fmt.Errorf("needs approval: show the plan to the person, then they approve it with `ebb approve %s` or in ebb's window; call ebb_cleanup_execute again afterwards", id)
}

// errNoOneToApprove: a plan made in this process lives only here, where the
// person cannot see or approve it.
var errNoOneToApprove = errors.New("needs the person's approval, and no ebb is running to collect it: ask them to start ebb " +
	"(`ebb open`), then make the plan again; they approve it in ebb's window " +
	"or with `ebb approve <id>`")

type auto struct {
	local    Backend
	discover func() (*remote.Client, error)
}

// Auto uses the running ebb (ebb serve or the background service) whenever
// one runs, looked up on every call, and the engine in this process
// otherwise. So a plan is made where the person can see and approve it as
// soon as ebb runs, even if it started after the agent.
func Auto(local Backend, discover func() (*remote.Client, error)) Backend {
	return &auto{local, discover}
}

func (a *auto) pick() Backend {
	if c, err := a.discover(); err == nil {
		return Remote(c)
	}
	return a.local
}

func (a *auto) Status(ctx context.Context) (api.Status, error) { return a.pick().Status(ctx) }
func (a *auto) Scan(ctx context.Context, req api.ScanRequest, wait bool) (api.Status, error) {
	return a.pick().Scan(ctx, req, wait)
}
func (a *auto) Volume(ctx context.Context) (volume.Breakdown, error) { return a.pick().Volume(ctx) }
func (a *auto) Findings(ctx context.Context, c string, min uint64) (api.FindingsResponse, error) {
	return a.pick().Findings(ctx, c, min)
}
func (a *auto) Children(ctx context.Context, p string, n int) (api.ChildrenResponse, error) {
	return a.pick().Children(ctx, p, n)
}
func (a *auto) Files(ctx context.Context, p string, min uint64, days, n int) (api.FilesResponse, error) {
	return a.pick().Files(ctx, p, min, days, n)
}
func (a *auto) Search(ctx context.Context, q string) (api.SearchResponse, error) {
	return a.pick().Search(ctx, q)
}
func (a *auto) Explain(ctx context.Context, p string) (api.Explain, error) {
	return a.pick().Explain(ctx, p)
}
func (a *auto) Presets(ctx context.Context) ([]api.PresetView, error) { return a.pick().Presets(ctx) }
func (a *auto) Apps(ctx context.Context) (api.AppsResponse, error)    { return a.pick().Apps(ctx) }
func (a *auto) Duplicates(ctx context.Context, p string, min uint64) (*dupes.Result, error) {
	return a.pick().Duplicates(ctx, p, min)
}
func (a *auto) Diff(ctx context.Context, from, to string) (api.DiffResponse, error) {
	return a.pick().Diff(ctx, from, to)
}
func (a *auto) Plan(ctx context.Context, req clean.Request) (*clean.Plan, error) {
	return a.pick().Plan(ctx, req)
}
func (a *auto) Execute(ctx context.Context, id string) (clean.Result, error) {
	return a.pick().Execute(ctx, id)
}
func (a *auto) Undo(ctx context.Context, id string) error { return a.pick().Undo(ctx, id) }
func (a *auto) Note(ctx context.Context, key, text string) error {
	return a.pick().Note(ctx, key, text)
}
func (a *auto) Report(ctx context.Context, format string) (string, error) {
	return a.pick().Report(ctx, format)
}

type local struct {
	a       *app.App
	version string
}

// Local runs the engine in this process.
func Local(a *app.App, version string) Backend { return &local{a, version} }

func (l *local) Status(context.Context) (api.Status, error) { return l.a.Status(l.version), nil }

func (l *local) Scan(ctx context.Context, req api.ScanRequest, wait bool) (api.Status, error) {
	opt := app.ScanOptions{Root: req.Root, Threads: req.Threads, Gentle: req.Gentle, Full: req.Full}
	if wait {
		if _, err := l.a.Scan(ctx, opt, nil); err != nil {
			return api.Status{}, err
		}
	} else if err := l.a.StartScan(opt, nil, nil); err != nil {
		return api.Status{}, err
	}
	return l.a.Status(l.version), nil
}

func (l *local) Volume(context.Context) (volume.Breakdown, error) { return l.a.Volume() }
func (l *local) Findings(_ context.Context, c string, min uint64) (api.FindingsResponse, error) {
	out, err := l.a.Findings(c, min, false)
	out.Notes = report.Notes(l.a, l.version)
	return out, err
}
func (l *local) Children(_ context.Context, p string, n int) (api.ChildrenResponse, error) {
	return l.a.Children(p, n, false)
}
func (l *local) Files(_ context.Context, p string, min uint64, days, n int) (api.FilesResponse, error) {
	return l.a.Files(p, min, days, n)
}
func (l *local) Search(_ context.Context, q string) (api.SearchResponse, error) {
	return l.a.Search(q, 50)
}
func (l *local) Explain(_ context.Context, p string) (api.Explain, error) { return l.a.Explain(p) }
func (l *local) Presets(context.Context) ([]api.PresetView, error)        { return l.a.Presets(), nil }
func (l *local) Apps(ctx context.Context) (api.AppsResponse, error)       { return l.a.Apps(ctx) }
func (l *local) Duplicates(ctx context.Context, p string, min uint64) (*dupes.Result, error) {
	return l.a.FindDuplicates(ctx, p, min, nil)
}
func (l *local) Diff(_ context.Context, from, to string) (api.DiffResponse, error) {
	return l.a.Diff(from, to, 10_000_000)
}
func (l *local) Plan(_ context.Context, req clean.Request) (*clean.Plan, error) { return l.a.Plan(req) }

func (l *local) Execute(ctx context.Context, id string) (clean.Result, error) {
	p, err := l.a.GetPlan(id)
	if err != nil {
		return clean.Result{}, err
	}
	if p.NeedsApproval && !p.Approved {
		return clean.Result{}, errNoOneToApprove
	}
	return l.a.Execute(ctx, id, nil)
}

func (l *local) Undo(_ context.Context, id string) error        { return l.a.Undo(id) }
func (l *local) Note(_ context.Context, key, text string) error { return l.a.SetNote(key, text) }
func (l *local) Report(_ context.Context, format string) (string, error) {
	r, err := l.a.Last()
	if err != nil {
		return "", err
	}
	rep := report.Build(l.a.View(r), l.a.Links, l.version)
	var b bytes.Buffer
	switch format {
	case "", "text":
		err = report.WriteText(&b, rep, report.Width)
	case "json":
		err = report.WriteJSON(&b, rep)
	case "csv":
		err = report.WriteCSV(&b, rep)
	default:
		return "", errors.New("format is text, json or csv")
	}
	return b.String(), err
}

type remoteBackend struct{ c *remote.Client }

// Remote talks to a running ebb server with this machine's token.
func Remote(c *remote.Client) Backend { return &remoteBackend{c} }

func (r *remoteBackend) get(ctx context.Context, path string, out any) error {
	return r.c.Do(ctx, "GET", path, nil, out)
}

func (r *remoteBackend) Status(ctx context.Context) (api.Status, error) {
	var st api.Status
	return st, r.get(ctx, "/status", &st)
}

func (r *remoteBackend) Scan(ctx context.Context, req api.ScanRequest, wait bool) (api.Status, error) {
	if err := r.c.Do(ctx, "POST", "/scans", req, nil); err != nil {
		return api.Status{}, err
	}
	for wait {
		st, err := r.Status(ctx)
		if err != nil || !st.Scan.Running {
			return st, err
		}
		select {
		case <-ctx.Done():
			return api.Status{}, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return r.Status(ctx)
}

func (r *remoteBackend) Volume(ctx context.Context) (volume.Breakdown, error) {
	var v volume.Breakdown
	return v, r.get(ctx, "/volume", &v)
}

func (r *remoteBackend) Findings(ctx context.Context, c string, min uint64) (api.FindingsResponse, error) {
	var f api.FindingsResponse
	return f, r.get(ctx, "/findings?category="+url.QueryEscape(c)+"&minBytes="+strconv.FormatUint(min, 10), &f)
}

func (r *remoteBackend) Children(ctx context.Context, p string, n int) (api.ChildrenResponse, error) {
	var out api.ChildrenResponse
	return out, r.get(ctx, "/children?limit="+strconv.Itoa(n)+"&path="+url.QueryEscape(p), &out)
}

func (r *remoteBackend) Files(ctx context.Context, p string, min uint64, days, n int) (api.FilesResponse, error) {
	var out api.FilesResponse
	q := fmt.Sprintf("/files?minSize=%d&olderThanDays=%d&limit=%d&path=%s", min, days, n, url.QueryEscape(p))
	return out, r.get(ctx, q, &out)
}

func (r *remoteBackend) Search(ctx context.Context, q string) (api.SearchResponse, error) {
	var out api.SearchResponse
	return out, r.get(ctx, "/search?limit=50&q="+url.QueryEscape(q), &out)
}

func (r *remoteBackend) Explain(ctx context.Context, p string) (api.Explain, error) {
	var out api.Explain
	return out, r.get(ctx, "/explain?path="+url.QueryEscape(p), &out)
}

func (r *remoteBackend) Presets(ctx context.Context) ([]api.PresetView, error) {
	var out []api.PresetView
	return out, r.get(ctx, "/presets", &out)
}

func (r *remoteBackend) Apps(ctx context.Context) (api.AppsResponse, error) {
	var out api.AppsResponse
	return out, r.get(ctx, "/apps", &out)
}

// Duplicates starts the search in the running ebb (so its window shows it too) and waits for it.
func (r *remoteBackend) Duplicates(ctx context.Context, p string, min uint64) (*dupes.Result, error) {
	if err := r.c.Do(ctx, "POST", "/duplicates", api.DuplicatesRequest{Path: p, MinSize: min}, nil); err != nil {
		return nil, err
	}
	for {
		var st api.DuplicatesState
		if err := r.get(ctx, "/duplicates", &st); err != nil {
			return nil, err
		}
		switch {
		case st.Running:
		case st.Error != "":
			return nil, errors.New(st.Error)
		case st.Stopped || st.Result == nil:
			return nil, errors.New("the search was stopped in ebb's window before it finished")
		default:
			return st.Result, nil
		}
		select {
		case <-ctx.Done():
			_ = r.c.Do(context.Background(), "DELETE", "/duplicates/current", nil, nil)
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (r *remoteBackend) Diff(ctx context.Context, from, to string) (api.DiffResponse, error) {
	var out api.DiffResponse
	return out, r.get(ctx, "/history/diff?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), &out)
}

func (r *remoteBackend) Plan(ctx context.Context, req clean.Request) (*clean.Plan, error) {
	var p clean.Plan
	return &p, r.c.Do(ctx, "POST", "/plans", req, &p)
}

func (r *remoteBackend) Execute(ctx context.Context, id string) (clean.Result, error) {
	var p clean.Plan
	if err := r.get(ctx, "/plans/"+url.PathEscape(id), &p); err != nil {
		return clean.Result{}, err
	}
	if p.NeedsApproval && !p.Approved {
		return clean.Result{}, errNeedsApproval(id)
	}
	var res clean.Result
	return res, r.c.Do(ctx, "POST", "/plans/"+url.PathEscape(id)+"/execute", nil, &res)
}

func (r *remoteBackend) Undo(ctx context.Context, id string) error {
	return r.c.Do(ctx, "POST", "/ops/"+url.PathEscape(id)+"/undo", nil, nil)
}

func (r *remoteBackend) Note(ctx context.Context, key, text string) error {
	return r.c.Do(ctx, "POST", "/notes", api.NoteRequest{Key: key, Text: text}, nil)
}

func (r *remoteBackend) Report(ctx context.Context, format string) (string, error) {
	if format == "" {
		format = "text"
	}
	if format != "text" && format != "json" && format != "csv" {
		return "", errors.New("format is text, json or csv")
	}
	b, err := r.c.Raw(ctx, "/report?format="+format)
	return string(b), err
}
