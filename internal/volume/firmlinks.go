package volume

import (
	"os"
	"strings"
)

// Firmlinks maps paths on the data volume to the paths people see: macOS
// presents /System/Volumes/Data/Users as /Users, and so on.
type Firmlinks struct {
	dataRoot string
	targets  []string // data-volume relative, e.g. "Users", "System/Library/Caches"
}

// LoadFirmlinks reads /usr/share/firmlinks; a missing table maps nothing.
func LoadFirmlinks(path, dataRoot string) Firmlinks {
	f := Firmlinks{dataRoot: strings.TrimSuffix(dataRoot, "/")}
	b, err := os.ReadFile(path)
	if err != nil {
		return f
	}
	for line := range strings.Lines(string(b)) {
		if _, target, ok := strings.Cut(strings.TrimRight(line, "\n"), "\t"); ok && target != "" {
			f.targets = append(f.targets, target)
		}
	}
	return f
}

func (f Firmlinks) match(rel string) bool {
	for _, t := range f.targets {
		if rel == t || strings.HasPrefix(rel, t+"/") {
			return true
		}
	}
	return false
}

func (f Firmlinks) Display(p string) string {
	if f.dataRoot == "" {
		return p
	}
	rel, ok := strings.CutPrefix(p, f.dataRoot+"/")
	if !ok || !f.match(rel) {
		return p
	}
	return "/" + rel
}

func (f Firmlinks) Real(display string) string {
	if f.dataRoot == "" || strings.HasPrefix(display, f.dataRoot+"/") {
		return display
	}
	if rel, ok := strings.CutPrefix(display, "/"); ok && f.match(rel) {
		return f.dataRoot + "/" + rel
	}
	return display
}
