package assets

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerHeaders(t *testing.T) {
	h := Handler("/api/")
	for _, path := range []string{"/", "/explore", "/api/v1/health"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("Content-Security-Policy"); got != Policy {
			t.Errorf("%s: CSP = %q", path, got)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Errorf("%s: missing nosniff or referrer policy: %v", path, rec.Header())
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("reserved path: status %d, want 404", rec.Code)
	}
}
