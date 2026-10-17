package server

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/history"
	"github.com/doguyilmaz/ebb/internal/report"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/volume"
)

func (s *Server) dataRoutes() {
	s.api("GET /api/v1/status", s.status)
	s.api("POST /api/v1/scans", s.startScan)
	s.api("GET /api/v1/scans/current", func(http.ResponseWriter, *http.Request) (any, error) { return s.app.ScanState(), nil })
	s.api("DELETE /api/v1/scans/current", func(w http.ResponseWriter, _ *http.Request) (any, error) {
		if !s.app.StopScan() {
			return nil, coded(http.StatusConflict, "no-scan", "no scan is running")
		}
		w.WriteHeader(http.StatusAccepted)
		return s.app.ScanState(), nil
	})
	s.api("GET /api/v1/volume", func(http.ResponseWriter, *http.Request) (any, error) { return s.app.Volume() })
	s.api("GET /api/v1/tree", s.tree)
	s.api("GET /api/v1/ages", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		out, err := s.app.Ages(r.URL.Query().Get("path"))
		return out, pathError(err)
	})
	s.api("GET /api/v1/children", s.children)
	s.api("GET /api/v1/search", s.search)
	s.api("GET /api/v1/findings", s.findings)
	s.mux.HandleFunc("GET /api/v1/report", s.auth(s.report))
	s.api("POST /api/v1/reveal", s.reveal)
	s.api("GET /api/v1/history", s.history)
	s.api("GET /api/v1/history/diff", s.diff)
	s.api("GET /api/v1/settings", func(http.ResponseWriter, *http.Request) (any, error) { return s.app.Settings(), nil })
	s.api("POST /api/v1/settings", s.saveSettings)
}

// pathError maps the facade's path errors to HTTP.
func pathError(err error) error {
	switch {
	case errors.Is(err, app.ErrOutsideRoot):
		return coded(http.StatusBadRequest, "outside-root", err.Error())
	case errors.Is(err, app.ErrNotInScan):
		return coded(http.StatusNotFound, "not-found", err.Error())
	}
	return err
}

func intParam(r *http.Request, name string, def, lo, hi int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return def
	}
	return max(lo, min(hi, v))
}

func (s *Server) status(_ http.ResponseWriter, _ *http.Request) (any, error) {
	st := api.Status{Version: s.version, Scan: s.app.ScanState(), DefaultRoot: s.app.Links.Display(s.app.DefaultRoot()),
		FullDiskAccess: volume.HasFullDiskAccess(s.app.Env.Home()), Sandboxed: sys.Sandboxed(s.app.Env)}
	if last, err := s.app.Last(); err == nil {
		m := last.Meta
		st.Last = &m
	}
	return st, nil
}

func (s *Server) startScan(w http.ResponseWriter, r *http.Request) (any, error) {
	var req api.ScanRequest
	if r.ContentLength != 0 {
		if err := decode(r, &req); err != nil {
			return nil, err
		}
	}
	var mu sync.Mutex
	var last time.Time
	err := s.app.StartScan(app.ScanOptions{Root: req.Root, Threads: req.Threads, Gentle: req.Gentle},
		func(p api.Progress) {
			mu.Lock()
			defer mu.Unlock()
			if time.Since(last) >= 250*time.Millisecond {
				last = time.Now()
				s.Notify(api.EventScanProgress, p)
			}
		},
		func(res *app.ScanResult, err error) {
			if err != nil {
				s.Notify(api.EventScanDone, map[string]any{"error": err.Error(), "stopped": errors.Is(err, app.ErrScanStopped)})
				return
			}
			s.Notify(api.EventScanDone, res.Meta)
			s.Notify(api.EventFindingsChanged, nil)
			s.Notify(api.EventHistoryChanged, nil)
		})
	if err != nil {
		return nil, err
	}
	w.WriteHeader(http.StatusAccepted)
	return s.app.ScanState(), nil
}

func (s *Server) tree(_ http.ResponseWriter, r *http.Request) (any, error) {
	q := r.URL.Query()
	share, err := strconv.ParseFloat(q.Get("minShare"), 64)
	if err != nil || share < 0 || share > 1 {
		share = 0.005
	}
	out, err := s.app.Slice(q.Get("path"), intParam(r, "depth", 3, 1, 8), share, intParam(r, "maxNodes", 2000, 10, 20000))
	return out, pathError(err)
}

func (s *Server) children(_ http.ResponseWriter, r *http.Request) (any, error) {
	out, err := s.app.Children(r.URL.Query().Get("path"), intParam(r, "limit", 500, 1, 5000), r.URL.Query().Get("small") == "1")
	return out, pathError(err)
}

func (s *Server) search(_ http.ResponseWriter, r *http.Request) (any, error) {
	q := r.URL.Query().Get("q")
	if q == "" {
		return api.SearchResponse{Items: []api.Item{}}, nil
	}
	return s.app.Search(q, intParam(r, "limit", 50, 1, 1000))
}

func (s *Server) findings(_ http.ResponseWriter, r *http.Request) (any, error) {
	min, _ := strconv.ParseUint(r.URL.Query().Get("minBytes"), 10, 64)
	out, err := s.app.Findings(r.URL.Query().Get("category"), min, r.URL.Query().Get("ignored") == "1")
	if err != nil {
		return nil, err
	}
	if last, err := s.app.Last(); err == nil {
		out.Notes = report.Build(s.app.View(last), s.app.Links, s.version).Notes
	}
	if out.Notes == nil {
		out.Notes = []string{}
	}
	return out, nil
}

func (s *Server) report(w http.ResponseWriter, r *http.Request) {
	last, err := s.app.Last()
	if err != nil {
		s.json(func(http.ResponseWriter, *http.Request) (any, error) { return nil, err })(w, r)
		return
	}
	rep := report.Build(s.app.View(last), s.app.Links, s.version)
	format := r.URL.Query().Get("format")
	types := map[string]string{"json": "application/json", "csv": "text/csv; charset=utf-8", "html": "text/html; charset=utf-8", "text": "text/plain; charset=utf-8"}
	ct, ok := types[format]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "format must be json, csv, html or text", "code": "bad-format"})
		return
	}
	w.Header().Set("Content-Type", ct)
	if format != "html" {
		w.Header().Set("Content-Disposition", `attachment; filename="ebb-report.`+map[string]string{"json": "json", "csv": "csv", "text": "txt"}[format]+`"`)
	}
	switch format {
	case "json":
		_ = report.WriteJSON(w, rep)
	case "csv":
		_ = report.WriteCSV(w, rep)
	case "html":
		_ = report.WriteHTML(w, rep, last.Tree)
	case "text":
		_ = report.WriteText(w, rep, 120)
	}
}

func (s *Server) reveal(_ http.ResponseWriter, r *http.Request) (any, error) {
	var req api.PathRequest
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	return nil, pathError(s.app.Reveal(req.Path))
}

func (s *Server) history(_ http.ResponseWriter, _ *http.Request) (any, error) {
	metas, err := s.app.History.List()
	if metas == nil {
		metas = []history.Meta{}
	}
	return api.HistoryResponse{Snapshots: metas}, err
}

func (s *Server) diff(_ http.ResponseWriter, r *http.Request) (any, error) {
	min, err := strconv.ParseUint(r.URL.Query().Get("minBytes"), 10, 64)
	if err != nil {
		min = 10_000_000
	}
	out, err := s.app.Diff(r.URL.Query().Get("from"), r.URL.Query().Get("to"), min)
	return out, pathError(err)
}

func (s *Server) saveSettings(_ http.ResponseWriter, r *http.Request) (any, error) {
	before := s.app.Settings()
	st := before
	st.Protect = nil
	if err := decode(r, &st); err != nil {
		return nil, err
	}
	if st.Protect == nil {
		st.Protect = before.Protect
	}
	// A paired page may add protections but only this machine may lift one,
	// so a compromised page cannot unprotect a folder and then clean it.
	if !s.master(r) && slices.ContainsFunc(before.Protect, func(p string) bool { return !slices.Contains(st.Protect, p) }) {
		return nil, coded(http.StatusForbidden, "local-only", "only ebb on this machine can remove a protection")
	}
	out, err := s.app.SaveSettings(st)
	if err != nil {
		return nil, badRequest(err.Error())
	}
	return out, nil
}
