package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/doguyilmaz/ebb/internal/svc"
)

func (s *Server) daemonRoutes() {
	s.api("GET /api/v1/daemon", func(http.ResponseWriter, *http.Request) (any, error) { return svc.Local(s.app.Env).Read(), nil })
	s.local("POST /api/v1/daemon/install", s.daemonAct(svc.Service.Install))
	s.local("POST /api/v1/daemon/restart", s.daemonAct(svc.Service.Restart))
	s.local("POST /api/v1/daemon/uninstall", s.daemonAct(svc.Service.Uninstall))
}

func (s *Server) daemonAct(act func(svc.Service, context.Context) (string, error)) func(http.ResponseWriter, *http.Request) (any, error) {
	return func(_ http.ResponseWriter, r *http.Request) (any, error) {
		sv := svc.Local(s.app.Env)
		if st := sv.Read(); st.Running && st.PID == os.Getpid() {
			// This server is the service: stopping it would cut the reply, so answer first.
			go func() {
				time.Sleep(300 * time.Millisecond)
				_, _ = act(sv, context.Background())
			}()
			return accepted{st}, nil
		}
		if _, err := act(sv, r.Context()); err != nil {
			if errors.Is(err, svc.ErrUnsupported) {
				return nil, coded(http.StatusConflict, "unsupported", err.Error())
			}
			return nil, badRequest(err.Error())
		}
		return sv.Read(), nil
	}
}
