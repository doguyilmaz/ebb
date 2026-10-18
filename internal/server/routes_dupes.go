package server

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/dupes"
)

func (s *Server) dupesRoutes() {
	s.api("GET /api/v1/duplicates", func(http.ResponseWriter, *http.Request) (any, error) { return s.app.Duplicates(), nil })
	s.api("POST /api/v1/duplicates", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var req api.DuplicatesRequest
		if r.ContentLength != 0 {
			if err := decode(r, &req); err != nil {
				return nil, err
			}
		}
		var mu sync.Mutex
		var last time.Time
		err := s.app.StartDuplicates(req.Path, req.MinSize,
			func(p dupes.Progress) {
				mu.Lock()
				defer mu.Unlock()
				if time.Since(last) >= 250*time.Millisecond || p.Done == p.Total {
					last = time.Now()
					s.Notify(api.EventDupesProgress, p)
				}
			},
			func(_ *dupes.Result, err error) {
				msg := ""
				if err != nil {
					msg = err.Error()
				}
				s.Notify(api.EventDupesDone, map[string]any{"error": msg})
			})
		if errors.Is(err, app.ErrOutsideRoot) || errors.Is(err, app.ErrNotInScan) {
			return nil, coded(http.StatusBadRequest, "outside-root", err.Error())
		}
		if err != nil {
			return nil, err
		}
		return accepted{s.app.Duplicates()}, nil
	})
	s.api("DELETE /api/v1/duplicates/current", func(http.ResponseWriter, *http.Request) (any, error) {
		if !s.app.StopDuplicates() {
			return nil, coded(http.StatusConflict, "no-search", "no duplicate search is running")
		}
		return accepted{s.app.Duplicates()}, nil
	})
}
