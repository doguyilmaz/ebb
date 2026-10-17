package server

import (
	"errors"
	"net/http"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/clean"
)

func (s *Server) cleanRoutes() {
	s.api("POST /api/v1/plans", s.makePlan)
	s.api("GET /api/v1/plans/{id}", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		p, err := s.app.GetPlan(r.PathValue("id"))
		return p, planError(err)
	})
	s.api("POST /api/v1/plans/{id}/approve", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		p, err := s.app.Approve(r.PathValue("id"))
		return p, planError(err)
	})
	s.api("POST /api/v1/plans/{id}/execute", s.execute)
	s.api("GET /api/v1/ops", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		ops, err := s.app.Ops(intParam(r, "limit", 200, 1, 5000))
		return api.OpsResponse{Ops: ops}, err
	})
	s.api("POST /api/v1/ops/{id}/undo", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		if err := s.app.Undo(r.PathValue("id")); err != nil {
			return nil, badRequest(err.Error())
		}
		s.Notify(api.EventFindingsChanged, nil)
		return nil, nil
	})
	s.api("GET /api/v1/presets", func(http.ResponseWriter, *http.Request) (any, error) { return s.app.Presets(), nil })
	s.api("GET /api/v1/notes", func(http.ResponseWriter, *http.Request) (any, error) { return s.app.Notes(), nil })
	s.api("POST /api/v1/notes", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var req api.NoteRequest
		if err := decode(r, &req); err != nil {
			return nil, err
		}
		if req.Key == "" || len(req.Text) > 4000 {
			return nil, badRequest("a note needs a key and at most 4000 characters")
		}
		return s.app.Notes(), s.app.SetNote(req.Key, req.Text)
	})
	s.api("GET /api/v1/ignores", func(http.ResponseWriter, *http.Request) (any, error) { return s.app.Ignores(), nil })
	s.api("POST /api/v1/ignores", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var req api.IgnoreRequest
		if err := decode(r, &req); err != nil {
			return nil, err
		}
		if err := s.app.Ignore(req.FindingID); err != nil {
			return nil, err
		}
		s.Notify(api.EventFindingsChanged, nil)
		return s.app.Ignores(), nil
	})
	s.api("DELETE /api/v1/ignores/{id}", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		if err := s.app.Unignore(r.PathValue("id")); err != nil {
			return nil, err
		}
		s.Notify(api.EventFindingsChanged, nil)
		return s.app.Ignores(), nil
	})
}

func planError(err error) error {
	switch {
	case errors.Is(err, app.ErrNoPlan):
		return coded(http.StatusNotFound, "no-plan", err.Error())
	case errors.Is(err, clean.ErrNeedsApproval):
		return coded(http.StatusConflict, "needs-approval", err.Error())
	case errors.Is(err, clean.ErrExpired):
		return coded(http.StatusGone, "plan-expired", err.Error())
	case errors.Is(err, clean.ErrEmptyPlan):
		return badRequest(err.Error())
	}
	return err
}

func (s *Server) makePlan(_ http.ResponseWriter, r *http.Request) (any, error) {
	var req api.PlanRequest
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	if req.Source == "" {
		req.Source = "ui"
	}
	p, err := s.app.Plan(req)
	if err != nil && !errors.Is(err, app.ErrNeverScanned) {
		return nil, badRequest(err.Error())
	}
	return p, err
}

// execute runs a plan and streams each item over the event stream; the
// response carries the result once everything ran.
func (s *Server) execute(_ http.ResponseWriter, r *http.Request) (any, error) {
	res, err := s.app.Execute(r.Context(), r.PathValue("id"), func(e clean.Event) { s.Notify(api.EventPlanProgress, e) })
	if err != nil && res.PlanID == "" {
		return nil, planError(err)
	}
	s.Notify(api.EventPlanDone, res)
	s.Notify(api.EventFindingsChanged, nil)
	return res, planError(err)
}
