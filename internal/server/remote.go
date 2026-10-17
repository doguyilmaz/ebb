package server

import (
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
	if origin := r.Header.Get("Origin"); origin != "" && !s.app.WebOriginAllowed(origin) {
		h.Allowed = false
		return h, nil
	}
	h.Device = s.app.Env.Hostname()
	return h, nil
}
