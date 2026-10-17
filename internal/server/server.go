// Package server serves the local web app: a token-protected JSON API on the
// loopback interface plus the embedded single-page frontend.
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/server/assets"
	"github.com/doguyilmaz/ebb/internal/state"
	"github.com/doguyilmaz/ebb/internal/sys"
)

type Server struct {
	app     *app.App
	token   string
	version string
	hub     *hub
	mux     *http.ServeMux
	spa     http.Handler
}

func New(a *app.App, version, token string) *Server {
	s := &Server{app: a, token: token, version: version, hub: newHub(), mux: http.NewServeMux(), spa: assets.Handler("/api/")}
	s.routes()
	return s
}

func (s *Server) Token() string { return s.token }

// loopbackHost refuses requests whose Host is not this machine: a page on a
// rebound DNS name reaches 127.0.0.1 but still sends its own name.
func loopbackHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !loopbackHost(r.Host) {
		writeJSON(w, http.StatusMisdirectedRequest, map[string]string{"error": "host not allowed"})
		return
	}
	if s.cors(w, r) {
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) api(pattern string, h func(http.ResponseWriter, *http.Request) (any, error)) {
	s.mux.HandleFunc(pattern, s.auth(s.json(h)))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET "+healthPath, s.json(s.health))
	s.mux.HandleFunc("POST /api/v1/pair", s.json(s.pair))
	s.api("GET /api/v1/pair/pending", s.pairPending)
	s.api("POST /api/v1/pair/confirm", s.pairConfirm)
	s.api("POST /api/v1/pair/reject", s.pairReject)
	s.api("GET /api/v1/web", s.webAccess)
	s.api("POST /api/v1/web/allow", s.webAllow)
	s.api("POST /api/v1/web/deny", s.webDeny)
	s.mux.HandleFunc("GET /api/v1/events", s.auth(s.handleEvents))
	s.mux.HandleFunc("/api/", s.auth(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such endpoint"})
	}))
	s.mux.Handle("/", s.spa)
}

func (s *Server) authorized(r *http.Request) bool {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || tok == "" {
		tok = r.URL.Query().Get("token")
	}
	return subtle.ConstantTimeCompare([]byte(tok), []byte(s.token)) == 1 || s.app.WebTokenValid(tok)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

type httpError struct {
	status int
	code   string
	msg    string
}

func (e *httpError) Error() string { return e.msg }

func badRequest(msg string) error              { return &httpError{http.StatusBadRequest, "", msg} }
func coded(status int, code, msg string) error { return &httpError{status, code, msg} }

func (s *Server) json(h func(http.ResponseWriter, *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := h(w, r)
		if err != nil {
			status, code := http.StatusInternalServerError, ""
			var he *httpError
			switch {
			case errors.As(err, &he):
				status, code = he.status, he.code
			case errors.Is(err, app.ErrNeverScanned):
				status, code = http.StatusConflict, "never-scanned"
			case errors.Is(err, app.ErrScanRunning):
				status, code = http.StatusConflict, "scan-running"
			}
			body := map[string]string{"error": err.Error()}
			if code != "" {
				body["code"] = code
			}
			writeJSON(w, status, body)
			return
		}
		if v == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v); err != nil {
		return badRequest("invalid JSON body: " + err.Error())
	}
	return nil
}

// LoadOrCreateToken keeps one API token per machine in the state folder, so
// a restarted server keeps working for an open page.
func LoadOrCreateToken(env sys.Env) (string, error) {
	p := filepath.Join(sys.StateDir(env), "ui-token")
	if b, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(b))) >= 32 {
		return strings.TrimSpace(string(b)), nil
	}
	tok := state.NewToken()
	return tok, state.WriteAtomic(p, []byte(tok+"\n"), 0o600)
}

// Serve listens on addr (loopback) until ctx ends.
func Serve(ctx context.Context, s *Server, addr string, ready func(url string)) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	if ready != nil {
		ready("http://" + ln.Addr().String() + "/#token=" + s.token)
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
