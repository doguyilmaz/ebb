package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/app"
)

func (c *client) get(path string, out any) (int, string) {
	c.t.Helper()
	r, _ := http.NewRequest("GET", c.srv.URL+path, nil)
	r.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil {
		_ = json.Unmarshal(raw, out)
	}
	return resp.StatusCode, string(raw) + " " + resp.Header.Get("Content-Type")
}

func scanned(t *testing.T) (*client, string) {
	c := newClient(t)
	root := filepath.Join(c.app.Env.Home(), "data")
	write := func(p string, n int) {
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, make([]byte, n), 0o644)
	}
	write(filepath.Join(root, "a", "big.bin"), 3<<20)
	write(filepath.Join(root, "a", "note.txt"), 100)
	write(filepath.Join(root, "b", "c.bin"), 2<<20)
	if _, err := c.app.Scan(context.Background(), app.ScanOptions{Root: root}, nil); err != nil {
		t.Fatal(err)
	}
	return c, root
}

func TestNeverScanned409(t *testing.T) {
	c := newClient(t)
	var e api.Error
	if status, _ := c.get("/api/v1/volume", &e); status != 409 || e.Code != "never-scanned" {
		t.Fatalf("%d %+v", status, e)
	}
}

func TestStatus(t *testing.T) {
	c, root := scanned(t)
	var st api.Status
	if status, _ := c.get("/api/v1/status", &st); status != 200 || st.Last == nil || st.Last.Root != root || !st.Sandboxed || st.Scan.Running {
		t.Fatalf("%d %+v", status, st)
	}
}

func TestTreeSliceDefaults(t *testing.T) {
	c, root := scanned(t)
	var s api.SliceResponse
	if status, body := c.get("/api/v1/tree", &s); status != 200 || s.Path != root || s.Root == nil || len(s.Root.Children) != 2 {
		t.Fatalf("%d %s", status, body)
	}
	if status, _ := c.get("/api/v1/tree?path="+url.QueryEscape(root+"/a")+"&depth=1&maxNodes=5", &s); status != 200 || s.Root.Name != "a" {
		t.Fatalf("%d %+v", status, s)
	}
}

func TestPathOutsideRootRejected(t *testing.T) {
	c, root := scanned(t)
	for _, p := range []string{"/etc/passwd", root + "/../../etc", root + "/a\x00/b", "relative/x"} {
		var e api.Error
		if status, _ := c.get("/api/v1/children?path="+url.QueryEscape(p), &e); status != 400 || e.Code != "outside-root" {
			t.Fatalf("%q: %d %+v", p, status, e)
		}
	}
}

func TestChildrenListsSmallFiles(t *testing.T) {
	c, root := scanned(t)
	var ch api.ChildrenResponse
	if status, body := c.get("/api/v1/children?small=1&path="+url.QueryEscape(root+"/a"), &ch); status != 200 || len(ch.Items) != 1 || len(ch.Small) != 1 || ch.Small[0].Name != "note.txt" {
		t.Fatalf("%d %s", status, body)
	}
	if ch.Items[0].Name != "big.bin" || ch.Items[0].Kind != "file" {
		t.Fatalf("%+v", ch.Items)
	}
}

func TestSearchAndFindings(t *testing.T) {
	c, _ := scanned(t)
	var sr api.SearchResponse
	if status, _ := c.get("/api/v1/search?q=*.bin", &sr); status != 200 || len(sr.Items) != 2 {
		t.Fatalf("%d %+v", status, sr)
	}
	var fr api.FindingsResponse
	if status, body := c.get("/api/v1/findings", &fr); status != 200 || fr.Findings == nil {
		t.Fatalf("%d %s", status, body)
	}
}

func TestScanConflictAndBackgroundScan(t *testing.T) {
	c, root := scanned(t)
	code, out := c.post("/api/v1/scans", "", true, `{"root":"`+root+`"}`)
	if code != 202 {
		t.Fatalf("start: %d %v", code, out)
	}
	if code, out := c.post("/api/v1/scans", "", true, `{"root":"`+root+`"}`); code != 409 || out["code"] != "scan-running" {
		// The first scan may already be done on a fast machine; only a 409 or a fresh 202 are acceptable.
		if code != 202 {
			t.Fatalf("second start: %d %v", code, out)
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		var st api.ScanState
		c.get("/api/v1/scans/current", &st)
		if !st.Running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background scan never finished")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestReportFormats(t *testing.T) {
	c, _ := scanned(t)
	for f, want := range map[string]string{"json": "application/json", "csv": "text/csv", "html": "text/html", "text": "text/plain"} {
		if status, body := c.get("/api/v1/report?format="+f, nil); status != 200 || !strings.Contains(body, want) {
			t.Fatalf("%s: %d %s", f, status, body[max(0, len(body)-80):])
		}
	}
	if status, _ := c.get("/api/v1/report?format=pdf", nil); status != 400 {
		t.Fatal("unknown format")
	}
}

func TestSettingsRoundTripAndValidation(t *testing.T) {
	c := newClient(t)
	if code, out := c.post("/api/v1/settings", "", true, `{"threads":4,"staleDays":45,"largeFile":"2GiB","protect":["~/Work/**"]}`); code != 200 || out["staleDays"] != float64(45) {
		t.Fatalf("%d %v", code, out)
	}
	var s api.Settings
	c.get("/api/v1/settings", &s)
	if s.Threads != 4 || s.LargeFile != "2GiB" || len(s.Protect) != 1 {
		t.Fatalf("%+v", s)
	}
	for _, bad := range []string{`{"threads":999}`, `{"staleDays":0}`, `{"largeFile":"lots"}`} {
		if code, _ := c.post("/api/v1/settings", "", true, bad); code != 400 {
			t.Fatalf("%s accepted: %d", bad, code)
		}
	}
}
