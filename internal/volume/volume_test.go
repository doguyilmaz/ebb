package volume

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/sys"
)

func TestParseAPFS(t *testing.T) {
	raw, err := os.ReadFile("testdata/apfs-list.json")
	if err != nil {
		t.Fatal(err)
	}
	var b Breakdown
	if err := applyAPFS(&b, raw, "disk3s5"); err != nil {
		t.Fatal(err)
	}
	if b.Capacity != 494384795648 || b.Free != 34654007296 || b.Used != 494384795648-34654007296 {
		t.Fatalf("container %+v", b)
	}
	if b.DataUsed != 425555922944 || b.SystemUsed != 12643491840 {
		t.Fatalf("volumes %+v", b)
	}
	var vm uint64
	for _, o := range b.Others {
		if o.Role == "VM" {
			vm = o.Used
		}
		if o.Role == "Data" || o.Role == "System" {
			t.Fatalf("data/system must not be listed as others: %+v", o)
		}
	}
	if vm != 10741170176 {
		t.Fatalf("vm %d in %+v", vm, b.Others)
	}
	if err := applyAPFS(&b, raw, "disk99"); err == nil {
		t.Fatal("unknown device must be an error")
	}
}

func TestParseSnapshots(t *testing.T) {
	raw, _ := os.ReadFile("testdata/tmutil.txt")
	snaps := parseSnapshots(raw)
	if len(snaps) != 2 || !snaps[1].Date.Equal(time.Date(2026, 10, 11, 22, 10, 5, 0, time.Local)) {
		t.Fatalf("snapshots %+v", snaps)
	}
	if snaps[0].Name != "com.apple.TimeMachine.2026-10-10-101500.local" {
		t.Fatal(snaps[0].Name)
	}
}

func TestAttributeNotVisible(t *testing.T) {
	b := Breakdown{DataUsed: 100}
	b.Attribute(70)
	if b.Scanned != 70 || b.NotVisible != 30 {
		t.Fatalf("%+v", b)
	}
	b.Attribute(120)
	if b.NotVisible != 0 {
		t.Fatalf("over-attribution must floor at 0: %+v", b)
	}
}

func TestFirmlinksDisplay(t *testing.T) {
	f := filepath.Join(t.TempDir(), "firmlinks")
	os.WriteFile(f, []byte("/Users\tUsers\n/Applications\tApplications\n/System/Library/Caches\tSystem/Library/Caches\n"), 0o644)
	fl := LoadFirmlinks(f, "/System/Volumes/Data")
	cases := map[string]string{
		"/System/Volumes/Data/Users/a":                 "/Users/a",
		"/System/Volumes/Data/Users":                   "/Users",
		"/System/Volumes/Data/System/Library/Caches/x": "/System/Library/Caches/x",
		"/System/Volumes/Data/.Spotlight-V100":         "/System/Volumes/Data/.Spotlight-V100",
		"/Users2/x":                                    "/Users2/x",
	}
	for in, want := range cases {
		if got := fl.Display(in); got != want {
			t.Errorf("Display(%q) = %q, want %q", in, got, want)
		}
		if got := fl.Real(fl.Display(in)); got != in {
			t.Errorf("Real(Display(%q)) = %q", in, got)
		}
	}
	if got := LoadFirmlinks(filepath.Join(t.TempDir(), "missing"), "/x").Display("/x/Users"); got != "/x/Users" {
		t.Fatalf("missing table must be identity: %q", got)
	}
}

func TestInspectSandboxed(t *testing.T) {
	env := &sys.Fake{HomeDir: t.TempDir(), GOOS: "darwin", RunFn: func(dir, name string, args ...string) ([]byte, error) {
		return nil, sys.ErrSandbox
	}}
	b, err := Inspect(context.Background(), env, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if b.Capacity == 0 || b.Used == 0 || b.DataUsed == 0 {
		t.Fatalf("statfs fallback missing: %+v", b)
	}
}

func TestFullDiskAccessProbe(t *testing.T) {
	home := t.TempDir()
	if HasFullDiskAccess(home) {
		t.Fatal("no TCC.db must mean no access")
	}
	db := filepath.Join(home, "Library", "Application Support", "com.apple.TCC", "TCC.db")
	os.MkdirAll(filepath.Dir(db), 0o755)
	os.WriteFile(db, []byte("x"), 0o644)
	if !HasFullDiskAccess(home) {
		t.Fatal("readable TCC.db means access")
	}
}
