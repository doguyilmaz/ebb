package volume

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
)

// Disk is a volume someone can scan: the startup disk (through its data
// volume) or any disk the Finder shows.
type Disk struct {
	Name     string `json:"name"`
	Path     string `json:"path"` // what to scan
	Capacity uint64 `json:"capacity"`
	Free     uint64 `json:"free"`
	Boot     bool   `json:"boot"`
	ReadOnly bool   `json:"readOnly"`
	Network  bool   `json:"network"`
	FS       string `json:"fs"`
}

type mount struct {
	on, fs              string
	capacity, free      uint64
	rdonly, local, hide bool
}

// Disks lists the startup disk first, then the others by name.
func Disks() []Disk {
	var out []Disk
	for _, m := range mounts() {
		d := Disk{Name: filepath.Base(m.on), Path: m.on, Capacity: m.capacity, Free: m.free, ReadOnly: m.rdonly, Network: !m.local, FS: m.fs}
		switch {
		case m.on == "/System/Volumes/Data":
			d.Name, d.Boot = bootName(), true
		case m.hide, m.on == "/", !filepath.IsAbs(m.on):
			continue
		}
		out = append(out, d)
	}
	slices.SortFunc(out, func(a, b Disk) int {
		if a.Boot != b.Boot {
			if a.Boot {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return out
}

// bootName is the startup disk's name as the Finder shows it: the entry in
// /Volumes that points at /.
func bootName() string {
	des, _ := os.ReadDir("/Volumes")
	for _, de := range des {
		if t, err := os.Readlink(filepath.Join("/Volumes", de.Name())); err == nil && t == "/" {
			return de.Name()
		}
	}
	return "This Mac"
}
