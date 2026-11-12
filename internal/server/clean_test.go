package server

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/sys"
)

func cleanClient(t *testing.T) (*client, string) {
	c := newClient(t)
	home := c.app.Env.Home()
	os.MkdirAll(filepath.Join(home, ".config", "ebb", "rules"), 0o755)
	os.WriteFile(filepath.Join(home, ".config", "ebb", "rules", "t.toml"), []byte(`
[[rule]]
id = "t.cache"
title = "T"
category = "package-cache"
paths = ["~/w/cache"]
safety = "regenerable"
why = "w"
impact = "i"
min_size = "1MB"
min_target = "1MB"
action = { kind = "delete" }
`), 0o600)
	a, err := app.New(c.app.Env)
	if err != nil {
		t.Fatal(err)
	}
	c.app, c.s.app = a, a
	os.MkdirAll(filepath.Join(home, "w", "cache"), 0o755)
	os.WriteFile(filepath.Join(home, "w", "cache", "b"), make([]byte, 2<<20), 0o644)
	if _, err := a.Scan(context.Background(), app.ScanOptions{Root: home}, nil); err != nil {
		t.Fatal(err)
	}
	return c, home
}

func TestPlanApproveExecuteOverAPI(t *testing.T) {
	c, home := cleanClient(t)
	code, plan := c.post("/api/v1/plans", "", true, `{"findingIds":["t.cache"],"source":"ui"}`)
	if code != 200 || plan["id"] == nil {
		t.Fatalf("%d %v", code, plan)
	}
	id := plan["id"].(string)
	if code, _ := c.post("/api/v1/plans/"+id+"/approve", "", true, ""); code != 200 {
		t.Fatalf("approve %d", code)
	}
	code, res := c.post("/api/v1/plans/"+id+"/execute", "", true, "")
	if code != 200 || res["done"] != float64(1) {
		t.Fatalf("execute %d %v", code, res)
	}
	if _, err := os.Stat(filepath.Join(home, "w", "cache")); !os.IsNotExist(err) {
		t.Fatal("not deleted")
	}
	var ops api.OpsResponse
	if c.get("/api/v1/ops", &ops); len(ops.Ops) != 1 {
		t.Fatalf("%+v", ops)
	}
	if code, out := c.post("/api/v1/plans/nope/execute", "", true, ""); code != 404 {
		t.Fatalf("%d %v", code, out)
	}
}

// clock is a test Env whose time moves while the server answers.
type clock struct {
	*sys.Fake
	now atomic.Int64
}

func (c *clock) Now() time.Time { return time.Unix(0, c.now.Load()) }

// The server refuses a plan nobody approved and one that expired, whatever a
// client (an agent's MCP server) checked before asking.
func TestUnapprovedAndExpiredPlansRefusedOverAPI(t *testing.T) {
	c, home := cleanClient(t)
	env := &clock{Fake: c.app.Env.(*sys.Fake)}
	env.now.Store(env.Fake.Now().UnixNano())
	a, err := app.New(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Scan(context.Background(), app.ScanOptions{Root: home}, nil); err != nil {
		t.Fatal(err)
	}
	c.app, c.s.app = a, a
	file := filepath.Join(home, "w", "cache", "b")
	code, plan := c.post("/api/v1/plans", "", true, `{"paths":["`+file+`"],"source":"mcp"}`)
	if code != 200 || plan["needsApproval"] != true {
		t.Fatalf("%d %v", code, plan)
	}
	if code, out := c.post("/api/v1/plans/"+plan["id"].(string)+"/execute", "", true, ""); code != 409 || out["code"] != "needs-approval" {
		t.Fatalf("unapproved: %d %v", code, out)
	}
	code, plan = c.post("/api/v1/plans", "", true, `{"findingIds":["t.cache"],"source":"mcp"}`)
	if code != 200 || plan["needsApproval"] != false {
		t.Fatalf("%d %v", code, plan)
	}
	env.now.Add(int64(11 * time.Minute))
	if code, out := c.post("/api/v1/plans/"+plan["id"].(string)+"/execute", "", true, ""); code != 410 || out["code"] != "plan-expired" {
		t.Fatalf("expired: %d %v", code, out)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("nothing may be touched: %v", err)
	}
}

func TestPresetsNotesIgnoresOverAPI(t *testing.T) {
	c, _ := cleanClient(t)
	var presets []api.PresetView
	if code, _ := c.get("/api/v1/presets", &presets); code != 200 || len(presets) < 6 {
		t.Fatalf("%d %d", code, len(presets))
	}
	if code, _ := c.post("/api/v1/notes", "", true, `{"key":"finding:t.cache","text":"later"}`); code != 200 {
		t.Fatalf("note %d", code)
	}
	var fr api.FindingsResponse
	c.get("/api/v1/findings", &fr)
	if len(fr.Findings) == 0 || fr.Findings[0].UserNote != "later" {
		t.Fatalf("%+v", fr)
	}
	if code, _ := c.post("/api/v1/ignores", "", true, `{"findingId":"t.cache"}`); code != 200 {
		t.Fatalf("ignore %d", code)
	}
	c.get("/api/v1/findings", &fr)
	if len(fr.Findings) != 0 {
		t.Fatal("ignored finding still listed")
	}
	c.get("/api/v1/findings?ignored=1", &fr)
	if len(fr.Findings) != 1 || !fr.Findings[0].Ignored {
		t.Fatalf("%+v", fr)
	}
}
