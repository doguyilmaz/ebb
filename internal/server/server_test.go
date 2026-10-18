package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/sys"
)

const hosted = "https://ebb.apps.doguyilmaz.com"

type client struct {
	t   *testing.T
	srv *httptest.Server
	s   *Server
	app *app.App
}

func newClient(t *testing.T) *client {
	t.Helper()
	home := t.TempDir()
	env := &sys.Fake{HomeDir: home, GOOS: "darwin", Time: time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC),
		Envs:  map[string]string{sys.HomeEnv: home, sys.SandboxEnv: "1"},
		RunFn: func(string, string, ...string) ([]byte, error) { return nil, sys.ErrSandbox }}
	a, err := app.New(env)
	if err != nil {
		t.Fatal(err)
	}
	s := New(a, "test", "secret")
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return &client{t: t, srv: srv, s: s, app: a}
}

func (c *client) req(method, path string, hdr map[string]string) *http.Response {
	c.t.Helper()
	r, _ := http.NewRequest(method, c.srv.URL+path, nil)
	for k, v := range hdr {
		if k == "Host" {
			r.Host = v
		} else {
			r.Header.Set(k, v)
		}
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestHealthOpen(t *testing.T) {
	c := newClient(t)
	resp := c.req("GET", "/api/v1/health", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("health %d", resp.StatusCode)
	}
	var h api.Health
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		t.Fatal(err)
	}
	if want := c.srv.Listener.Addr().(*net.TCPAddr).Port; h.Port != want {
		t.Fatalf("health port %d, listening on %d", h.Port, want)
	}
}

func TestAuthRequired(t *testing.T) {
	c := newClient(t)
	if resp := c.req("GET", "/api/v1/events", nil); resp.StatusCode != 401 {
		t.Fatalf("events without token: %d", resp.StatusCode)
	}
	if resp := c.req("GET", "/api/v1/events", map[string]string{"Authorization": "Bearer wrong"}); resp.StatusCode != 401 {
		t.Fatalf("wrong token: %d", resp.StatusCode)
	}
}

func TestHostHeaderRejected(t *testing.T) {
	c := newClient(t)
	for _, p := range []string{"/api/v1/health", "/", "/api/v1/events"} {
		if resp := c.req("GET", p, map[string]string{"Host": "evil.example:47340"}); resp.StatusCode != http.StatusMisdirectedRequest {
			t.Fatalf("%s with foreign Host: %d", p, resp.StatusCode)
		}
	}
	for _, h := range []string{"localhost:3000", "127.0.0.1:47340", "[::1]:47340"} {
		if resp := c.req("GET", "/api/v1/health", map[string]string{"Host": h}); resp.StatusCode != 200 {
			t.Fatalf("loopback host %s refused: %d", h, resp.StatusCode)
		}
	}
}

func TestCORSAllowedOriginOnly(t *testing.T) {
	c := newClient(t)
	pre := func(origin string) *http.Response {
		return c.req("OPTIONS", "/api/v1/events", map[string]string{"Origin": origin, "Access-Control-Request-Method": "GET"})
	}
	ok := pre(hosted)
	if ok.StatusCode != 204 || ok.Header.Get("Access-Control-Allow-Origin") != hosted {
		t.Fatalf("allowed origin: %d %q", ok.StatusCode, ok.Header.Get("Access-Control-Allow-Origin"))
	}
	bad := pre("https://evil.example")
	if bad.StatusCode != 403 || bad.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("foreign origin: %d %q", bad.StatusCode, bad.Header.Get("Access-Control-Allow-Origin"))
	}
	if h := c.req("GET", "/api/v1/health", map[string]string{"Origin": "https://evil.example"}).Header.Get("Access-Control-Allow-Origin"); h == "*" {
		t.Fatal("never a wildcard")
	}
}

func TestPrivateNetworkHeader(t *testing.T) {
	c := newClient(t)
	resp := c.req("OPTIONS", "/api/v1/events", map[string]string{"Origin": hosted, "Access-Control-Request-Method": "GET",
		"Access-Control-Request-Private-Network": "true"})
	if resp.Header.Get("Access-Control-Allow-Private-Network") != "true" {
		t.Fatal("private network opt-in missing")
	}
}

func TestEventStreamTickets(t *testing.T) {
	c := newClient(t)
	if resp := c.req("GET", "/api/v1/events?token=secret", nil); resp.StatusCode != 401 {
		t.Fatalf("a token in the URL is not accepted: %d", resp.StatusCode)
	}
	_, got := c.post("/api/v1/events/ticket", "", true, "")
	tk, _ := got["ticket"].(string)
	if len(tk) < 32 {
		t.Fatalf("ticket %v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r, _ := http.NewRequestWithContext(ctx, "GET", c.srv.URL+"/api/v1/events?ticket="+tk, nil)
	resp, err := http.DefaultClient.Do(r)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("a fresh ticket opens the stream: %v %v", resp, err)
	}
	cancel()
	resp.Body.Close()
	if resp := c.req("GET", "/api/v1/events?ticket="+tk, nil); resp.StatusCode != 401 {
		t.Fatalf("a ticket works once: %d", resp.StatusCode)
	}
	if resp := c.req("POST", "/api/v1/events/ticket", nil); resp.StatusCode != 401 {
		t.Fatalf("tickets need the token: %d", resp.StatusCode)
	}
}

func TestSSEHelloAndEvent(t *testing.T) {
	c := newClient(t)
	resp := c.req("GET", "/api/v1/events", map[string]string{"Authorization": "Bearer secret"})
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("%d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	rd := bufio.NewReader(resp.Body)
	readEvent := func() string {
		var lines []string
		for {
			l, err := rd.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if l == "\n" {
				return strings.Join(lines, "")
			}
			lines = append(lines, l)
		}
	}
	if ev := readEvent(); !strings.Contains(ev, "event: hello") {
		t.Fatalf("first frame %q", ev)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		c.s.Notify(api.EventFindingsChanged, map[string]int{"n": 3})
	}()
	if ev := readEvent(); !strings.Contains(ev, "event: findings.changed") || !strings.Contains(ev, `"n":3`) {
		t.Fatalf("event frame %q", ev)
	}
}

func TestSPAFallback(t *testing.T) {
	c := newClient(t)
	resp := c.req("GET", "/explore", nil)
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), "<html") {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	if resp := c.req("GET", "/api/v1/nope", map[string]string{"Authorization": "Bearer secret"}); resp.StatusCode != 404 {
		t.Fatalf("unknown API route must 404, not serve the app: %d", resp.StatusCode)
	}
}
