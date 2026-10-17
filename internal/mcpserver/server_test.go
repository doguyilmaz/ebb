package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/sys"
)

const rules = `
[[rule]]
id = "test.cache"
title = "Test cache"
category = "package-cache"
paths = ["~/.testcache/pkgs"]
safety = "regenerable"
why = "Rebuilt on demand."
impact = "Rebuilt next time."
min_size = "1MB"
min_target = "1MB"
action = { kind = "delete" }

[[rule]]
id = "installers"
title = "Installers"
category = "installer"
paths = ["~/Downloads/**/*.dmg"]
safety = "caution"
why = "Kept after installing."
impact = "Download again to reinstall."
min_size = "1MB"
min_target = "1MB"
action = { kind = "trash" }
`

func fill(t *testing.T, p string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sandbox(t *testing.T) (*app.App, string) {
	t.Helper()
	home := t.TempDir()
	fill(t, filepath.Join(home, ".testcache", "pkgs", "blob"), 3<<20)
	fill(t, filepath.Join(home, "Downloads", "Setup.dmg"), 2<<20)
	if err := os.MkdirAll(filepath.Join(home, ".config", "ebb", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "ebb", "rules", "t.toml"), []byte(rules), 0o644); err != nil {
		t.Fatal(err)
	}
	env := &sys.Fake{HomeDir: home, GOOS: "darwin", Time: time.Now(), Envs: map[string]string{sys.HomeEnv: home, sys.SandboxEnv: "1"},
		RunFn: func(string, string, ...string) ([]byte, error) { return nil, sys.ErrSandbox }}
	a, err := app.New(env)
	if err != nil {
		t.Fatal(err)
	}
	return a, home
}

type reply struct {
	ID     int `json:"id"`
	Result struct {
		Tools   []struct{ Name string } `json:"tools"`
		Content []struct{ Text string } `json:"content"`
		IsError bool                    `json:"isError"`
		Version string                  `json:"protocolVersion"`
	} `json:"result"`
	Error *struct{ Message string } `json:"error"`
}

func session(t *testing.T, s *Server, lines ...string) map[int]reply {
	t.Helper()
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	got := map[int]reply{}
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var r reply
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("bad reply %q: %v", l, err)
		}
		got[r.ID] = r
	}
	return got
}

func call(id int, name, args string) string {
	return `{"jsonrpc":"2.0","id":` + itoa(id) + `,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}`
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestServeToolsOverStdio(t *testing.T) {
	a, home := sandbox(t)
	s := New(Local(a, "test"), "test")
	got := session(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		call(3, "ebb_scan", `{"root":"`+home+`"}`),
		call(4, "ebb_overview", `{}`),
		call(5, "ebb_explain", `{"path":"`+filepath.Join(home, ".testcache", "pkgs")+`"}`),
		call(6, "nope", `{}`),
		`{"jsonrpc":"2.0","id":7,"method":"bogus/method"}`,
		call(8, "ebb_cleanup_plan", `{}`),
	)
	if len(got) != 8 {
		t.Fatalf("replies %d: a notification gets none, every request one", len(got))
	}
	if got[1].Result.Version != protocolVersion {
		t.Fatalf("initialize %+v", got[1])
	}
	names := map[string]bool{}
	for _, tl := range got[2].Result.Tools {
		names[tl.Name] = true
	}
	for _, want := range []string{"ebb_status", "ebb_scan", "ebb_overview", "ebb_top", "ebb_findings", "ebb_explain", "ebb_cleanup_plan", "ebb_cleanup_execute", "ebb_undo"} {
		if !names[want] {
			t.Fatalf("tools/list misses %s", want)
		}
	}
	for _, id := range []int{3, 4, 5} {
		if got[id].Result.IsError || len(got[id].Result.Content) == 0 {
			t.Fatalf("call %d: %+v", id, got[id])
		}
	}
	if !strings.Contains(got[4].Result.Content[0].Text, "reclaimable") || !strings.Contains(got[5].Result.Content[0].Text, "test.cache") {
		t.Fatalf("overview/explain content:\n%s\n%s", got[4].Result.Content[0].Text, got[5].Result.Content[0].Text)
	}
	if got[6].Error == nil || got[7].Error == nil || !got[8].Result.IsError {
		t.Fatalf("unknown tool %+v, unknown method %+v, empty plan %+v", got[6], got[7], got[8])
	}
}

func planID(t *testing.T, s *Server, findings string) string {
	t.Helper()
	got := session(t, s, call(1, "ebb_cleanup_plan", `{"findingIds":["`+findings+`"]}`))
	var p struct{ ID string }
	if got[1].Result.IsError || json.Unmarshal([]byte(got[1].Result.Content[0].Text), &p) != nil || p.ID == "" {
		t.Fatalf("plan: %+v", got[1])
	}
	return p.ID
}

func TestExecuteNeedsApproval(t *testing.T) {
	a, home := sandbox(t)
	if _, err := a.Scan(context.Background(), app.ScanOptions{Root: home}, nil); err != nil {
		t.Fatal(err)
	}
	s := New(Local(a, "test"), "test")

	id := planID(t, s, "installers")
	got := session(t, s, call(1, "ebb_cleanup_execute", `{"planId":"`+id+`"}`))
	if !got[1].Result.IsError || !strings.Contains(got[1].Result.Content[0].Text, "needs approval") {
		t.Fatalf("an installer plan must wait for the person: %+v", got[1])
	}
	if _, err := os.Stat(filepath.Join(home, "Downloads", "Setup.dmg")); err != nil {
		t.Fatal("nothing may be touched before approval")
	}
	if _, err := a.Approve(id); err != nil {
		t.Fatal(err)
	}
	if got := session(t, s, call(1, "ebb_cleanup_execute", `{"planId":"`+id+`"}`)); got[1].Result.IsError {
		t.Fatalf("approved by the person, it runs: %+v", got[1])
	}

	cache := planID(t, s, "test.cache")
	if got := session(t, s, call(1, "ebb_cleanup_execute", `{"planId":"`+cache+`"}`)); got[1].Result.IsError {
		t.Fatalf("a plan of rebuildable caches runs without approval: %+v", got[1])
	}
	if _, err := os.Stat(filepath.Join(home, ".testcache", "pkgs")); !os.IsNotExist(err) {
		t.Fatalf("cache still there: %v", err)
	}
}
