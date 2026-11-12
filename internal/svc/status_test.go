package svc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrimLogKeepsNewestCompleteLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.log")
	content := "old-1\nold-2\nnew-1\nnew-2\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	TrimLog(dir, 13)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-1\nnew-2\n" {
		t.Fatalf("trimmed log = %q, want newest complete lines", got)
	}
	if int64(len(got)) > 13 {
		t.Fatalf("trimmed log is %d bytes, want at most 13", len(got))
	}
}
