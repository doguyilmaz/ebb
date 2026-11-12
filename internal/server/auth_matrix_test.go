package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type protectedRoute struct {
	method string
	path   string
}

// Keep this table next to the route registration audit. Authentication runs
// before every handler, so even routes that would open a chooser or mutate a
// plan are safe to exercise with no token.
var protectedRoutes = []protectedRoute{
	{"GET", "/api/v1/status"},
	{"POST", "/api/v1/scans"},
	{"GET", "/api/v1/scans/current"},
	{"DELETE", "/api/v1/scans/current"},
	{"GET", "/api/v1/volumes"},
	{"GET", "/api/v1/sources"},
	{"GET", "/api/v1/packages"},
	{"GET", "/api/v1/apps"},
	{"GET", "/api/v1/volume"},
	{"GET", "/api/v1/tree"},
	{"GET", "/api/v1/ages"},
	{"GET", "/api/v1/children"},
	{"GET", "/api/v1/explain"},
	{"GET", "/api/v1/files"},
	{"GET", "/api/v1/search"},
	{"GET", "/api/v1/findings"},
	{"GET", "/api/v1/report?format=json"},
	{"POST", "/api/v1/reveal"},
	{"POST", "/api/v1/ask"},
	{"POST", "/api/v1/choose-folder"},
	{"GET", "/api/v1/history"},
	{"GET", "/api/v1/history/diff"},
	{"GET", "/api/v1/settings"},
	{"POST", "/api/v1/settings"},
	{"GET", "/api/v1/daemon"},
	{"GET", "/api/v1/duplicates"},
	{"POST", "/api/v1/duplicates"},
	{"DELETE", "/api/v1/duplicates/current"},
	{"POST", "/api/v1/events/ticket"},
	{"GET", "/api/v1/events"},
	{"POST", "/api/v1/plans"},
	{"GET", "/api/v1/plans"},
	{"GET", "/api/v1/plans/nope"},
	{"POST", "/api/v1/plans/nope/approve"},
	{"POST", "/api/v1/plans/nope/execute"},
	{"GET", "/api/v1/ops"},
	{"POST", "/api/v1/ops/nope/undo"},
	{"GET", "/api/v1/presets"},
	{"GET", "/api/v1/notes"},
	{"POST", "/api/v1/notes"},
	{"GET", "/api/v1/ignores"},
	{"POST", "/api/v1/ignores"},
	{"DELETE", "/api/v1/ignores/nope"},
	{"POST", "/api/v1/daemon/install"},
	{"POST", "/api/v1/daemon/restart"},
	{"POST", "/api/v1/daemon/uninstall"},
	{"GET", "/api/v1/web"},
	{"POST", "/api/v1/web/allow"},
	{"POST", "/api/v1/web/deny"},
}

func TestEveryProtectedRouteRejectsMissingAndWrongToken(t *testing.T) {
	c := newClient(t)
	for _, tc := range protectedRoutes {
		for _, token := range []string{"", "wrong"} {
			r, err := http.NewRequest(tc.method, c.srv.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			resp, err := http.DefaultClient.Do(r)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("%s %s with token %q: got %d", tc.method, tc.path, token, resp.StatusCode)
			}
		}
	}
}

func TestPairedTokenCannotCallMachineOnlyRoutes(t *testing.T) {
	c := newClient(t)
	token := pairedToken(t, c)
	for _, tc := range []protectedRoute{
		{"GET", "/api/v1/web"},
		{"POST", "/api/v1/web/allow"},
		{"POST", "/api/v1/web/deny"},
		{"POST", "/api/v1/daemon/install"},
		{"POST", "/api/v1/daemon/restart"},
		{"POST", "/api/v1/daemon/uninstall"},
	} {
		r, err := http.NewRequest(tc.method, c.srv.URL+tc.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("%s %s with paired token: got %d", tc.method, tc.path, resp.StatusCode)
		}
	}
}

func TestEventTicketExpiresAndStopsAfterRevocation(t *testing.T) {
	c := newClient(t)
	token := pairedToken(t, c)
	expired := c.s.tickets.issue(token)
	c.s.tickets.mu.Lock()
	v := c.s.tickets.m[expired]
	v.expires = time.Now().Add(-time.Second)
	c.s.tickets.m[expired] = v
	c.s.tickets.mu.Unlock()
	if resp := c.req("GET", "/api/v1/events?ticket="+expired, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired ticket: %d", resp.StatusCode)
	}
	revoked := c.s.tickets.issue(token)
	if _, err := c.app.DenyWebOrigin(hosted); err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", c.srv.URL+"/api/v1/events?ticket="+revoked, strings.NewReader(""))
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked ticket: %d", resp.StatusCode)
	}
}
