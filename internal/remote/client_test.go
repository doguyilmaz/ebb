package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoSendsTokenAndDecodesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		if r.URL.Path == "/api/v1/fail" {
			w.WriteHeader(409)
			w.Write([]byte(`{"error":"needs approval"}`))
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := &Client{Base: srv.URL, Token: "tok", HTTP: srv.Client()}
	var out struct{ OK bool }
	if err := c.Do(context.Background(), "GET", "/x", nil, &out); err != nil || !out.OK {
		t.Fatal(err)
	}
	if err := c.Do(context.Background(), "POST", "/fail", map[string]int{"a": 1}, nil); err == nil || err.Error() != "needs approval" {
		t.Fatalf("%v", err)
	}
}
