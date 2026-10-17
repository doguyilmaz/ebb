package tree

// MarkerNames are the files whose presence identifies a project folder; each
// has a bit in a directory's Markers. MarkerXcode stands for a .xcodeproj or
// .xcworkspace child.
var MarkerNames = []string{"package.json", "Cargo.toml", "go.mod", "build.gradle", "build.gradle.kts", "settings.gradle",
	"Podfile", "pubspec.yaml", "pyproject.toml", "requirements.txt", "setup.py", "Gemfile", "composer.json", "pom.xml"}

const MarkerXcode uint16 = 1 << 15

var markerBits = func() map[string]uint16 {
	m := map[string]uint16{}
	for i, n := range MarkerNames {
		m[n] = 1 << i
	}
	return m
}()

// MarkerBit returns the bit for a marker file name, or 0.
func MarkerBit(name string) uint16 { return markerBits[name] }

// SetMarkers records marker files found in dir and flags it as a project.
func (w *Writer) SetMarkers(dir ID, m uint16) {
	w.t.markers[dir] |= m
	if m != 0 {
		w.t.flags[dir] |= FlagProject
	}
}
