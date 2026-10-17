package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
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

const nonce = `{"nonce":"n0nce-for-this-tab-1234567890abcd"}`

func TestPairFirstPending(t *testing.T) {
	c := newClient(t)
	status, body := c.post("/api/v1/pair", hosted, false, nonce)
	if status != 200 || body["pending"] != true || body["token"] != nil || len(body["code"].(string)) != 6 {
		t.Fatalf("%d %v", status, body)
	}
	if status, _ := c.post("/api/v1/pair", "https://evil.example", false, nonce); status != 403 {
		t.Fatalf("unknown origin: %d", status)
	}
	if status, _ := c.post("/api/v1/pair", "", false, nonce); status != 400 {
		t.Fatalf("no origin: %d", status)
	}
	if status, _ := c.post("/api/v1/pair", hosted, false, `{"nonce":"short"}`); status != 400 {
		t.Fatalf("a weak nonce must be refused: %d", status)
	}
}

func TestPairSelfConfirmRejected(t *testing.T) {
	c := newClient(t)
	_, body := c.post("/api/v1/pair", hosted, false, nonce)
	code := body["code"].(string)
	if status, _ := c.post("/api/v1/pair/confirm", hosted, false, `{"code":"`+code+`"}`); status != 401 {
		t.Fatalf("confirm without token: %d", status)
	}
	if status, _ := c.post("/api/v1/pair/confirm", hosted, true, `{"code":"`+code+`"}`); status != 403 {
		t.Fatalf("a page must never confirm itself, even holding a token: %d", status)
	}
	if _, body := c.post("/api/v1/pair", hosted, false, nonce); body["token"] != nil {
		t.Fatal("token handed out without a confirmation")
	}
}

func TestPairConfirmedGetsToken(t *testing.T) {
	c := newClient(t)
	_, body := c.post("/api/v1/pair", hosted, false, nonce)
	if status, out := c.post("/api/v1/pair/confirm", "", true, `{"code":"`+body["code"].(string)+`"}`); status != 200 {
		t.Fatalf("local confirm: %d %v", status, out)
	}
	status, got := c.post("/api/v1/pair", hosted, false, nonce)
	tok, _ := got["token"].(string)
	if status != 200 || len(tok) < 32 || tok == "secret" {
		t.Fatalf("the page gets its own token, never the master one: %d %v", status, got)
	}
	r, _ := http.NewRequest("GET", c.srv.URL+"/api/v1/web", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	if resp, err := http.DefaultClient.Do(r); err != nil || resp.StatusCode != 200 {
		t.Fatalf("issued token must work: %v %v", resp, err)
	}
	if _, again := c.post("/api/v1/pair", hosted, false, nonce); again["token"] != nil {
		t.Fatal("a confirmation hands out exactly one token")
	}
}

func TestPairSpoofedOriginGetsNothing(t *testing.T) {
	c := newClient(t)
	_, body := c.post("/api/v1/pair", hosted, false, nonce)
	c.post("/api/v1/pair/confirm", "", true, `{"code":"`+body["code"].(string)+`"}`)
	// Anything can send an Origin header; without the page's nonce it is just a new request.
	_, got := c.post("/api/v1/pair", hosted, false, `{"nonce":"another-process-guessing-0123456789"}`)
	if got["token"] != nil {
		t.Fatal("a confirmed origin header alone must not release a token")
	}
}

func TestDenyRevokesIssuedTokens(t *testing.T) {
	c := newClient(t)
	_, body := c.post("/api/v1/pair", hosted, false, nonce)
	c.post("/api/v1/pair/confirm", "", true, `{"code":"`+body["code"].(string)+`"}`)
	_, got := c.post("/api/v1/pair", hosted, false, nonce)
	tok := got["token"].(string)
	c.post("/api/v1/web/deny", "", true, `{"origin":"`+hosted+`"}`)
	r, _ := http.NewRequest("GET", c.srv.URL+"/api/v1/web", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	if resp, _ := http.DefaultClient.Do(r); resp.StatusCode != 401 {
		t.Fatalf("denied origin's token still works: %d", resp.StatusCode)
	}
}

func TestPairWrongCodeLockout(t *testing.T) {
	c := newClient(t)
	c.post("/api/v1/pair", hosted, false, nonce)
	for i := 0; i < 5; i++ {
		if status, _ := c.post("/api/v1/pair/confirm", "", true, `{"code":"000000x"}`); status != 400 {
			t.Fatalf("wrong code %d: %d", i, status)
		}
	}
	if p, _ := c.app.PendingPair(); p != nil {
		t.Fatal("request must be discarded after five wrong codes")
	}
}

func TestPairBusy(t *testing.T) {
	c := newClient(t)
	if _, err := c.app.AllowWebOrigin("https://second.example"); err != nil {
		t.Fatal(err)
	}
	c.post("/api/v1/pair", hosted, false, nonce)
	if status, _ := c.post("/api/v1/pair", "https://second.example", false, nonce); status != 409 {
		t.Fatalf("second origin while one is pending: %d", status)
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

func pairedToken(t *testing.T, c *client) string {
	_, body := c.post("/api/v1/pair", hosted, false, nonce)
	c.post("/api/v1/pair/confirm", "", true, `{"code":"`+body["code"].(string)+`"}`)
	_, got := c.post("/api/v1/pair", hosted, false, nonce)
	return got["token"].(string)
}

func TestWebTokenCannotManageAccess(t *testing.T) {
	c := newClient(t)
	tok := pairedToken(t, c)
	for _, p := range []string{"/api/v1/web/allow", "/api/v1/web/deny", "/api/v1/pair/confirm", "/api/v1/pair/reject"} {
		r, _ := http.NewRequest("POST", c.srv.URL+p, strings.NewReader(`{"origin":"https://x.example","code":"123456"}`))
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

func TestConfirmNeedsTheCode(t *testing.T) {
	c := newClient(t)
	c.post("/api/v1/pair", hosted, false, nonce)
	if status, _ := c.post("/api/v1/pair/confirm", "", true, `{}`); status != 400 {
		t.Fatalf("confirming without the code the page shows: %d", status)
	}
	if p, _ := c.app.PendingPair(); p == nil || p.Confirmed {
		t.Fatal("request must stay unconfirmed")
	}
}
