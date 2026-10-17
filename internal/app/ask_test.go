package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/sys"
)

func TestAskSendsMetadataOnly(t *testing.T) {
	a, root := testApp(t)
	secret := filepath.Join(root, "docs", "passwords.txt")
	if err := os.WriteFile(secret, []byte("SECRET-hunter2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Scan(context.Background(), ScanOptions{Root: root, Full: true}, nil); err != nil {
		t.Fatal(err)
	}
	if a.AskAvailable() {
		t.Fatal("no claude on PATH (and a sandbox): Ask stays hidden")
	}
	var prompt, cwd string
	var args []string
	f := a.Env.(*sys.Fake)
	f.Bins = map[string]string{"claude": "/bin/claude"}
	delete(f.Envs, sys.SandboxEnv)
	f.RunFn = func(dir, name string, as ...string) ([]byte, error) {
		cwd, args, prompt = dir, as, as[len(as)-1]
		return []byte(`{"type":"result","result":"It is **your** docs folder."}`), nil
	}
	if !a.AskAvailable() {
		t.Fatal("claude present: Ask is available")
	}
	out, err := a.Ask(context.Background(), api.AskRequest{Path: filepath.Join(root, "docs"), Question: "Is it safe to delete?"})
	if err != nil || out.Answer != "It is **your** docs folder." {
		t.Fatalf("answer %+v %v", out, err)
	}
	if strings.Contains(prompt, "hunter2") {
		t.Fatal("file contents must never reach the model")
	}
	for _, want := range []string{"docs", "Is it safe to delete?"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt misses %q:\n%s", want, prompt)
		}
	}
	tools := slices.Index(args, "--tools")
	if tools < 0 || args[tools+1] != "" || !slices.Contains(args, "--strict-mcp-config") || cwd == "" || cwd == root {
		t.Fatalf("claude must run with no tools and no MCP servers, in an empty folder: %q in %q", args, cwd)
	}
}
