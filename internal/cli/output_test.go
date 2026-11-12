package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/sys"
)

func TestPrintTableStripsControlCharacters(t *testing.T) {
	var buf bytes.Buffer
	printTable(&buf, []string{"PATH"}, [][]string{{"/x/\x1b[31mred\x1b[0m\nnext"}})
	if strings.ContainsAny(buf.String(), "\x1b") || strings.Count(buf.String(), "\n") != 2 {
		t.Fatalf("%q", buf.String())
	}
}

func TestJSONMutatingCommandsEmitJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv(sys.HomeEnv, home)
	t.Setenv(sys.SandboxEnv, "1")
	run := func(args ...string) (int, string, string) {
		var out, errOut bytes.Buffer
		code := Main(append([]string{"--home", home, "--json"}, args...), &out, &errOut)
		return code, out.String(), errOut.String()
	}
	check := func(name string, args ...string) string {
		t.Helper()
		code, out, errOut := run(args...)
		if code != 0 {
			t.Fatalf("%s failed: code=%d stdout=%q stderr=%q", name, code, out, errOut)
		}
		if !json.Valid([]byte(out)) {
			t.Fatalf("%s did not emit JSON: %q", name, out)
		}
		return out
	}
	var notes map[string]string
	if err := json.Unmarshal([]byte(check("note", "note", "~/memo.txt", "remember")), &notes); err != nil {
		t.Fatal(err)
	}
	if notes["path:"+filepath.Join(home, "memo.txt")] != "remember" {
		t.Fatalf("note output: %#v", notes)
	}
	var protect []string
	if err := json.Unmarshal([]byte(check("protect add", "protect", "add", "~/Documents/**")), &protect); err != nil {
		t.Fatal(err)
	}
	if len(protect) != 1 || protect[0] != "~/Documents/**" {
		t.Fatalf("protect add output: %#v", protect)
	}
	if err := json.Unmarshal([]byte(check("protect ls", "protect", "ls")), &protect); err != nil {
		t.Fatal(err)
	}
	if len(protect) != 1 || protect[0] != "~/Documents/**" {
		t.Fatalf("protect ls output: %#v", protect)
	}
	if err := os.WriteFile(filepath.Join(home, "payload"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := app.New(sys.Real())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Scan(context.Background(), app.ScanOptions{Root: home}, nil); err != nil {
		t.Fatal(err)
	}
	var ignored map[string]any
	if err := json.Unmarshal([]byte(check("ignore", "ignore", "unknown")), &ignored); err != nil {
		t.Fatal(err)
	}
	if _, ok := ignored["unknown"]; !ok {
		t.Fatalf("ignore output: %#v", ignored)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(check("report", "report")), &report); err != nil {
		t.Fatal(err)
	}
	if len(report) == 0 {
		t.Fatal("report output is empty")
	}
}
