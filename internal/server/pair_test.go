package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/sys"
)

func (c *client) post(path, origin string, token bool, body string) (int, map[string]any) {
	c.t.Helper()
	r, _ := http.NewRequest("POST", c.srv.URL+path, strings.NewReader(body))
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if token {
		r.Header.Set("Authorization", "Bearer secret")
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func TestPairNeedsAllowedOriginAndKey(t *testing.T) {
	c := newClient(t)
	key := `{"key":"` + c.linkKey(hosted) + `"}`
	if status, _ := c.post("/api/v1/pair", "https://evil.example", false, key); status != 403 {
		t.Fatalf("unknown origin: %d", status)
	}
	if status, _ := c.post("/api/v1/pair", "", false, key); status != 400 {
		t.Fatalf("no origin: %d", status)
	}
	if status, _ := c.post("/api/v1/pair", hosted, false, `{"nonce":"an-older-page-asking-0123456789"}`); status != 400 {
		t.Fatalf("no key: %d", status)
	}
	// Anything can send an Origin header; without the key it gets nothing.
	if status, body := c.post("/api/v1/pair", hosted, false, `{"key":"`+strings.Repeat("0", 48)+`"}`); status != 403 || body["token"] != nil {
		t.Fatalf("a guessed key: %d %v", status, body)
	}
}

func pairedToken(t *testing.T, c *client) string {
	t.Helper()
	_, got := c.post("/api/v1/pair", hosted, false, `{"key":"`+c.linkKey(hosted)+`"}`)
	return got["token"].(string)
}

func TestDenyRevokesIssuedTokens(t *testing.T) {
	c := newClient(t)
	tok := pairedToken(t, c)
	c.post("/api/v1/web/deny", "", true, `{"origin":"`+hosted+`"}`)
	if resp := c.req("GET", "/api/v1/status", map[string]string{"Authorization": "Bearer " + tok}); resp.StatusCode != 401 {
		t.Fatalf("denied origin's token still works: %d", resp.StatusCode)
	}
}

func TestWebAllowDeny(t *testing.T) {
	c := newClient(t)
	if status, _ := c.post("/api/v1/web/allow", "", true, `{"origin":"http://evil.example"}`); status != 400 {
		t.Fatalf("plain http for a remote host must be refused: %d", status)
	}
	if status, _ := c.post("/api/v1/web/allow", "", true, `{"origin":"https://other.example"}`); status != 200 || !c.app.WebOriginAllowed("https://other.example") {
		t.Fatalf("allow: %d", status)
	}
	if status, _ := c.post("/api/v1/web/deny", "", true, `{"origin":"https://other.example"}`); status != 200 || c.app.WebOriginAllowed("https://other.example") {
		t.Fatalf("deny: %d", status)
	}
}

func TestWebTokenCannotManageAccess(t *testing.T) {
	c := newClient(t)
	tok := pairedToken(t, c)
	for _, p := range []string{"/api/v1/web/allow", "/api/v1/web/deny"} {
		r, _ := http.NewRequest("POST", c.srv.URL+p, strings.NewReader(`{"origin":"https://x.example"}`))
		r.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Fatalf("%s with a page token: %d", p, resp.StatusCode)
		}
	}
}

func TestPageTokenCannotRemoveProtections(t *testing.T) {
	c := newClient(t)
	if code, _ := c.post("/api/v1/settings", "", true, `{"protect":["~/Work/**"]}`); code != 200 {
		t.Fatalf("local add %d", code)
	}
	tok := pairedToken(t, c)
	send := func(body string) int {
		r, _ := http.NewRequest("POST", c.srv.URL+"/api/v1/settings", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := send(`{"protect":[]}`); code != 403 {
		t.Fatalf("a paired page dropped a protection: %d", code)
	}
	if code := send(`{"protect":["~/Work/**","~/Photos/**"]}`); code != 200 {
		t.Fatalf("adding protection from a page must work: %d", code)
	}
}

func (c *client) linkKey(origin string) string {
	c.t.Helper()
	k, err := c.app.IssueLinkKey(origin)
	if err != nil {
		c.t.Fatal(err)
	}
	return k
}

func TestPairWithLinkKey(t *testing.T) {
	c := newClient(t)
	key := `{"key":"` + c.linkKey(hosted) + `"}`
	status, body := c.post("/api/v1/pair", hosted, false, key)
	tok, _ := body["token"].(string)
	if status != 200 || len(tok) < 32 || tok == "secret" {
		t.Fatalf("%d %v", status, body)
	}
	if resp := c.req("GET", "/api/v1/status", map[string]string{"Origin": hosted, "Authorization": "Bearer " + tok}); resp.StatusCode != 200 {
		t.Fatalf("issued token refused: %d", resp.StatusCode)
	}
	if status, body := c.post("/api/v1/pair", hosted, false, key); status != 403 || body["code"] != "link-expired" {
		t.Fatalf("second use: %d %v", status, body)
	}
}

func TestPairLinkKeyExpires(t *testing.T) {
	c := newClient(t)
	key := `{"key":"` + c.linkKey(hosted) + `"}`
	f := c.app.Env.(*sys.Fake)
	f.Time = f.Time.Add(app.LinkWindow + time.Second)
	if status, body := c.post("/api/v1/pair", hosted, false, key); status != 403 || body["code"] != "link-expired" {
		t.Fatalf("%d %v", status, body)
	}
}

func TestPairLinkKeyBoundToOrigin(t *testing.T) {
	c := newClient(t)
	mine := "https://mine.example"
	if _, err := c.app.AllowWebOrigin(mine); err != nil {
		t.Fatal(err)
	}
	key := `{"key":"` + c.linkKey(hosted) + `"}`
	if status, _ := c.post("/api/v1/pair", mine, false, key); status != 403 {
		t.Fatalf("another origin claimed the key: %d", status)
	}
	if status, _ := c.post("/api/v1/pair", hosted, false, key); status != 200 {
		t.Fatalf("a refused claim from another origin used the key up: %d", status)
	}
	if _, err := c.app.IssueLinkKey("https://evil.example"); err == nil {
		t.Fatal("issued a key for an origin that is not allowed")
	}
}
