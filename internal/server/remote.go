package server

import (
	"net"
	"net/http"

	"github.com/doguyilmaz/ebb/internal/api"
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
	if a, ok := r.Context().Value(http.LocalAddrContextKey).(*net.TCPAddr); ok {
		h.Port = a.Port
	}
	if origin := r.Header.Get("Origin"); origin != "" && !s.app.WebOriginAllowed(origin) {
		h.Allowed = false
		return h, nil
	}
	h.Device = s.app.Env.Hostname()
	return h, nil
}

// pair gives a page on an allowed origin a token of its own for the key in
// the link ebb open opened. The Origin header only says who may ask: any
// process can set one, so the key is what lets a page in.
func (s *Server) pair(_ http.ResponseWriter, r *http.Request) (any, error) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil, badRequest("pairing is for a browser page; the local UI already has the token")
	}
	if !s.app.WebOriginAllowed(origin) {
		return nil, coded(http.StatusForbidden, "origin-not-allowed", "this machine has not allowed "+origin+"; run: ebb web allow "+origin)
	}
	var req api.PairRequest
	_ = decode(r, &req)
	if req.Key == "" {
		return nil, badRequest("a page connects through the link ebb open opens")
	}
	tok, ok, err := s.app.ClaimLinkKey(origin, req.Key)
	switch {
	case err != nil:
		return nil, err
	case !ok:
		return nil, coded(http.StatusForbidden, "link-expired", "this link has expired or was used already; run ebb open again")
	}
	return api.PairResponse{Token: tok, Device: s.app.Env.Hostname(), Version: s.version}, nil
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
