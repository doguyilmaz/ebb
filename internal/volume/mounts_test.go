package volume

import (
	"runtime"
	"testing"
)

func TestDisksStartWithTheStartupDisk(t *testing.T) {
	ds := Disks()
	if runtime.GOOS != "darwin" {
		t.Skip("mount table is macOS's")
	}
	if len(ds) == 0 || !ds[0].Boot || ds[0].Path != "/System/Volumes/Data" || ds[0].Capacity == 0 {
		t.Fatalf("%+v", ds)
	}
	for _, d := range ds {
		if d.Path == "/" || d.Path == "/System/Volumes/VM" || d.Path == "/System/Volumes/Preboot" {
			t.Fatalf("hidden system volume listed: %+v", d)
		}
	}
}
