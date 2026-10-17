package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/history"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

func result() *app.ScanResult {
	tr := tree.New("/h")
	tr.Write(func(w *tree.Writer) {
		lib := w.AddDir(tr.Root(), []byte("Library"), 1, 0)
		w.AddFile(lib, []byte("big.bin"), tree.KindFile, 3<<30, 2<<30, 3<<30, 1700000000, tree.FlagClone)
		docs := w.AddDir(tr.Root(), []byte("Documents"), 1, 0)
		w.AddFile(docs, []byte("thesis <draft>.pdf"), tree.KindFile, 1<<30, 1<<30, 1<<30, 1700000000, 0)
	})
	tr.Finish()
	return &app.ScanResult{
		Meta: history.Meta{ID: "20261017T120000Z", Time: time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC), Root: "/h", Files: 2, Dirs: 3,
			Elapsed: 2 * time.Second},
		Tree:   tr,
		Volume: volume.Breakdown{Capacity: 500e9, Used: 460e9, Free: 40e9, DataUsed: 420e9, Scanned: 4 << 30, NotVisible: 30e9, Purgeable: 1e9},
		Findings: []finding.Finding{
			{ID: "bun.cache:1", RuleID: "bun.cache", Title: "Bun install cache", Category: "package-cache", Safety: finding.Regenerable,
				Bytes: 2 << 30, Alloc: 3 << 30, Targets: []finding.Target{{Path: "/h/Library/big.bin", Bytes: 2 << 30, Alloc: 3 << 30}},
				Why: "Bun keeps every version.", Impact: "Re-downloads.", Evidence: []string{"2 entries"}, Actions: []finding.Action{{Kind: "command"}}},
			{ID: "docker.vm:1", RuleID: "docker.vm", Title: "Docker", Category: "sdk", Safety: finding.Caution, Bytes: 5 << 30,
				Targets: []finding.Target{{Path: "/h/Docker.raw"}}, Why: "w", Impact: "i", Actions: []finding.Action{{Kind: "manual"}}},
			{ID: "userdata.mail:1", RuleID: "userdata.mail", Title: "Mail", Category: "user-data", Safety: finding.UserData, Bytes: 1 << 30,
				Targets: []finding.Target{{Path: "/h/Library/Mail"}}, Why: "w", Impact: "i", Notes: "Runtime images are mounted elsewhere."},
		},
		ProblemCount: 3,
	}
}

func TestBuildReclaimableAndNotes(t *testing.T) {
	r := Build(result(), volume.Firmlinks{}, "test")
	if r.Reclaimable != 2<<30 {
		t.Fatalf("only findings ebb can act on count as reclaimable: %d", r.Reclaimable)
	}
	notes := strings.Join(r.Notes, "\n")
	for _, want := range []string{"Full Disk Access", "not visible", "shared", "3 folders could not be read", "purgeable", "Runtime images are mounted elsewhere."} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes missing %q:\n%s", want, notes)
		}
	}
	if len(r.Breakdown) == 0 || r.Breakdown[0].Depth != 0 || len(r.TopFiles) != 2 || r.TopFiles[0].Name != "big.bin" {
		t.Fatalf("breakdown %+v top %+v", r.Breakdown, r.TopFiles)
	}
}

func TestJSONRoundTrips(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, Build(result(), volume.Firmlinks{}, "test")); err != nil {
		t.Fatal(err)
	}
	var back Report
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil || len(back.Findings) != 3 {
		t.Fatalf("%v %d", err, len(back.Findings))
	}
}

func TestCSVHasFindingsAndBreakdown(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, Build(result(), volume.Firmlinks{}, "test")); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, r := range rows[1:] {
		kinds[r[0]]++
	}
	if rows[0][0] != "kind" || kinds["finding"] != 3 || kinds["breakdown"] == 0 || kinds["file"] != 2 {
		t.Fatalf("%v", kinds)
	}
}

func TestHTMLSelfContained(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteHTML(&buf, Build(result(), volume.Firmlinks{}, "test"), result().Tree); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if regexp.MustCompile(`(src|href)=["']?https?:`).MatchString(out) {
		t.Fatal("report must not load anything from the network")
	}
	if !strings.Contains(out, `id="ebb-data"`) || !strings.Contains(out, "Bun install cache") {
		t.Fatal("data or findings missing")
	}
	if strings.Contains(out, "thesis <draft>") {
		t.Fatal("file names must be escaped")
	}
}

func TestTextFitsWidth(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteText(&buf, Build(result(), volume.Firmlinks{}, "test"), 100); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(buf.String(), "\n") {
		if n := len([]rune(line)); n > 100 {
			t.Fatalf("line of %d runes: %q", n, line)
		}
	}
	if !strings.Contains(buf.String(), "Bun install cache") {
		t.Fatal(buf.String())
	}
}
