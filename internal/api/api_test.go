package api

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestTSGenUpToDate(t *testing.T) {
	var buf bytes.Buffer
	if err := GenerateTS(&buf); err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile("../../ui/src/api/types.gen.ts")
	if err != nil || !bytes.Equal(committed, buf.Bytes()) {
		t.Fatal("ui/src/api/types.gen.ts is stale: run go generate ./internal/api")
	}
}

func TestTSGenShapes(t *testing.T) {
	type inner struct {
		N int `json:"n"`
	}
	type sample struct {
		Name    string            `json:"name"`
		Opt     string            `json:"opt,omitempty"`
		Skip    string            `json:"-"`
		When    time.Time         `json:"when"`
		List    []inner           `json:"list"`
		Ptr     *inner            `json:"ptr"`
		Map     map[string]uint64 `json:"map"`
		Any     any               `json:"any"`
		private int
		inner
	}
	var buf bytes.Buffer
	if err := generate(&buf, []any{sample{}}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"export interface sample {", "  name: string;", "  opt?: string;", "  when: string;", "  list: inner[] | null;",
		"  ptr: inner | null;", "  map: Record<string, number> | null;", "  any: unknown;", "  n: number;", "export interface inner {",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Skip") || strings.Contains(out, "private") {
		t.Errorf("ignored fields leaked:\n%s", out)
	}
}
