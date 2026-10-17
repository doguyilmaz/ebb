package server

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/app"
)

// healthPath answers without a token so a page served from elsewhere can tell
// "no ebb here" from "ebb is here but this page may not use it".
const healthPath = "/api/v1/health"

// cors lets the web app on an allowed origin reach this server from a
// browser, and reports whether it already answered the request. Chrome also
// demands the private-network opt-in, since the page is public and this API
// is on loopback.
func (s *Server) cors(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	health := r.URL.Path == healthPath
	allowed := s.app.WebOriginAllowed(origin)
	if allowed || health {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
	}
	if r.Method != http.MethodOptions || r.Header.Get("Access-Control-Request-Method") == "" {
		return false
	}
	if !allowed && !health {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "origin not allowed"})
		return true
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	if r.Header.Get("Access-Control-Request-Private-Network") == "true" {
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
	}
	w.Header().Set("Access-Control-Max-Age", "600")
	w.WriteHeader(http.StatusNoContent)
	return true
}

func (s *Server) health(_ http.ResponseWriter, r *http.Request) (any, error) {
	h := api.Health{OK: true, Version: s.version, Allowed: true}
	if origin := r.Header.Get("Origin"); origin != "" && !s.app.WebOriginAllowed(origin) {
		h.Allowed = false
		return h, nil
	}
	h.Device = s.app.Env.Hostname()
	return h, nil
}

// pair gives a page on an allowed origin a token of its own, after someone
// at this machine confirmed its request. The Origin header only says who may
// ask; the token goes to the request that presents the nonce the confirmed
// request was made with, once, because any process can set an Origin header.
func (s *Server) pair(_ http.ResponseWriter, r *http.Request) (any, error) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil, badRequest("pairing is for a browser page; the local UI already has the token")
	}
	if !s.app.WebOriginAllowed(origin) {
		return nil, coded(http.StatusForbidden, "origin-not-allowed", "this machine has not allowed "+origin+"; run: ebb web allow "+origin)
	}
	var req struct{ Nonce string }
	_ = decode(r, &req)
	if tok, ok, err := s.app.ClaimPair(origin, req.Nonce); err != nil {
		return nil, err
	} else if ok {
		return api.PairResponse{Token: tok, Device: s.app.Env.Hostname(), Version: s.version}, nil
	}
	p, err := s.app.RequestPair(origin, req.Nonce)
	switch {
	case errors.Is(err, app.ErrPairBusy):
		return nil, coded(http.StatusConflict, "pair-busy", err.Error())
	case errors.Is(err, app.ErrWeakNonce):
		return nil, badRequest(err.Error())
	case err != nil:
		return nil, err
	}
	return api.PairResponse{Pending: true, Code: p.Code, Expires: p.Requested.Add(app.PairWindow).Format(time.RFC3339),
		Confirm: "ebb web confirm " + p.Code,
		Message: "Confirm on the machine you are pairing with: run `ebb web confirm " + p.Code + "`, or approve it in ebb's local UI."}, nil
}

func (s *Server) pairPending(_ http.ResponseWriter, _ *http.Request) (any, error) {
	return s.app.WebAccess(), nil
}

// pairConfirm is the yes, reachable only with the local token and never from
// another origin: the page asking cannot approve itself.
func (s *Server) pairConfirm(_ http.ResponseWriter, r *http.Request) (any, error) {
	if origin := r.Header.Get("Origin"); origin != "" && !isLoopbackOrigin(origin) {
		return nil, coded(http.StatusForbidden, "not-local", "a confirmation has to come from this machine, not from the page asking to be let in")
	}
	var req struct{ Code string }
	_ = decode(r, &req)
	p, err := s.app.ConfirmPair(req.Code)
	if err != nil {
		return nil, badRequest(err.Error())
	}
	return map[string]string{"confirmed": p.Origin}, nil
}

func (s *Server) pairReject(_ http.ResponseWriter, _ *http.Request) (any, error) {
	return nil, s.app.RejectPair()
}

func isLoopbackOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	h := u.Hostname()
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) webAccess(_ http.ResponseWriter, _ *http.Request) (any, error) {
	return s.app.WebAccess(), nil
}

func (s *Server) webAllow(_ http.ResponseWriter, r *http.Request) (any, error) {
	var req struct{ Origin string }
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	out, err := s.app.AllowWebOrigin(req.Origin)
	if err != nil {
		return nil, badRequest(err.Error())
	}
	return out, nil
}

func (s *Server) webDeny(_ http.ResponseWriter, r *http.Request) (any, error) {
	var req struct{ Origin string }
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	out, err := s.app.DenyWebOrigin(req.Origin)
	if err != nil {
		return nil, badRequest(err.Error())
	}
	return out, nil
}
