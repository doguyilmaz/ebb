package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/app"
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
