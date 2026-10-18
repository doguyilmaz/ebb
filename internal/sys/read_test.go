package sys

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReadSmall(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small")
	os.WriteFile(small, []byte("dist\n"), 0o644)
	if b, err := ReadSmall(small, 16); err != nil || string(b) != "dist\n" {
		t.Fatalf("%q %v", b, err)
	}
	if _, err := ReadSmall(small, 2); err == nil {
		t.Error("a file over the limit was read")
	}
	link := filepath.Join(dir, "link")
	os.Symlink("/dev/zero", link)
	if _, err := ReadSmall(link, 16); err == nil {
		t.Error("followed a link")
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skip(err)
	}
	if _, err := ReadSmall(fifo, 16); err == nil {
		t.Error("read a pipe")
	}
}
