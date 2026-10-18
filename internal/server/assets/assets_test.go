package assets

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHandlerHeaders(t *testing.T) {
	src, err := os.ReadFile("../../../ui/public/_headers")
	if err != nil {
		t.Fatal(err)
	}
	headerRules = parseHeaders(string(src))
	h := Handler("/api/")
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	for _, path := range []string{"/", "/explore", "/assets/x.js", "/api/v1/health"} {
		got := get(path).Header()
		csp := got.Get("Content-Security-Policy")
		if !strings.Contains(csp, "frame-ancestors 'none'") || !strings.Contains(csp, "connect-src 'self' http://127.0.0.1:*") {
			t.Errorf("%s: CSP %q", path, csp)
		}
		if got.Get("X-Content-Type-Options") != "nosniff" || got.Get("Referrer-Policy") != "no-referrer" {
			t.Errorf("%s: missing nosniff or referrer policy: %v", path, got)
		}
	}
	if rec := get("/api/v1/health"); rec.Code != http.StatusNotFound {
		t.Errorf("reserved path: status %d, want 404", rec.Code)
	}
}

func TestParseHeaders(t *testing.T) {
	rules := parseHeaders("# note\n/*\n  A: 1\n\n/assets/*\n\tB: x: y\n/exact\n  C: 3\n")
	if len(rules) != 3 || rules[0].headers[0] != [2]string{"A", "1"} || rules[1].headers[0] != [2]string{"B", "x: y"} {
		t.Fatalf("%+v", rules)
	}
	if !rules[1].matches("/assets/a.js") || rules[1].matches("/fonts/a.woff2") || !rules[2].matches("/exact") || rules[2].matches("/exact/x") {
		t.Fatal("path matching")
	}
}
