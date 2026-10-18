package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
)

// tickets let a browser open the event stream (EventSource sends no headers)
// without putting its token in a URL: a ticket is asked for with the token in
// the header, works once, within ticketLife, and stands for that token.
type tickets struct {
	mu sync.Mutex
	m  map[string]ticket
}

type ticket struct {
	token   string
	expires time.Time
}

const ticketLife = 30 * time.Second

func (t *tickets) issue(token string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.m == nil {
		t.m = map[string]ticket{}
	}
	now := time.Now()
	for k, v := range t.m {
		if now.After(v.expires) {
			delete(t.m, k)
		}
	}
	t.m[id] = ticket{token: token, expires: now.Add(ticketLife)}
	return id
}

// redeem returns the token a ticket stands for and spends the ticket.
func (t *tickets) redeem(id string) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	v, ok := t.m[id]
	delete(t.m, id)
	if !ok || time.Now().After(v.expires) {
		return "", false
	}
	return v.token, true
}

func (s *Server) eventTicket(_ http.ResponseWriter, r *http.Request) (any, error) {
	return map[string]string{"ticket": s.tickets.issue(bearer(r))}, nil
}

// streamAuth admits the event stream with a header token or a ticket whose
// token is still valid (a page whose access was removed meanwhile is refused).
func (s *Server) streamAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			tok, ok := s.tickets.redeem(r.URL.Query().Get("ticket"))
			if !ok || !s.valid(tok) {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}
		next(w, r)
	}
}

type event struct {
	name string
	data []byte
}

// hub fans out server-sent events to every open browser tab; a slow tab
// misses events instead of stalling the others.
type hub struct {
	mu   sync.Mutex
	subs map[chan event]struct{}
}

func newHub() *hub { return &hub{subs: map[chan event]struct{}{}} }

func (h *hub) subscribe() chan event {
	ch := make(chan event, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan event) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

func (h *hub) publish(e event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Notify sends an event (one of the api.Event* names) to every open page.
func (s *Server) Notify(name string, data any) {
	b, err := json.Marshal(data)
	if err != nil || data == nil {
		b = []byte("{}")
	}
	s.hub.publish(event{name, b})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "event: %s\ndata: {}\n\n", api.EventHello)
	fl.Flush()
	ch := s.hub.subscribe()
	defer s.hub.unsubscribe(ch)
	tick := time.NewTicker(25 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.name, e.data)
			fl.Flush()
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		}
	}
}
