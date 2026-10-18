package apps

import (
	"cmp"
	"context"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/tree"
)

// Orphan is what an app that is no longer installed left in the Library.
type Orphan struct {
	BundleID string
	Path     string // display form
	Bytes    uint64
	Alloc    uint64
}

// orphanPlaces hold folders named exactly by bundle ID; names elsewhere
// (Application Support, Logs) cannot be tied to an app that is gone.
var orphanPlaces = []struct{ dir, suffix string }{
	{"Containers", ""}, {"Caches", ""}, {"HTTPStorages", ""}, {"WebKit", ""}, {"Application Scripts", ""},
	{"Saved Application State", ".savedState"},
}

// Orphans finds folders named by a bundle ID whose maker has no app on this
// Mac, untouched for quiet. A maker counts as present when an installed app
// or any bundle Spotlight knows (helpers, system extensions, apps on other
// disks) comes from it. Apple's are never orphans. ok is false when Spotlight
// cannot answer; then nothing counts as an orphan.
func (s Source) Orphans(ctx context.Context, installed []App, now time.Time, quiet time.Duration) (out []Orphan, ok bool) {
	vendors := map[string]bool{}
	for _, a := range installed {
		vendors[vendor(a.BundleID)] = true
	}
	asked := map[string]bool{}
	for _, p := range orphanPlaces {
		dir := s.library(p.dir)
		id, found := s.Tree.Lookup(s.Links.Real(dir))
		if !found {
			continue
		}
		for _, c := range s.Tree.Children(id) {
			n := s.Tree.Node(c)
			bid, cut := strings.CutSuffix(n.Name, p.suffix)
			if n.Kind != tree.KindDir || !cut || !bundleLike(bid) || vendors[vendor(bid)] || apple(bid) {
				continue
			}
			if now.Sub(time.Unix(n.MaxMtime, 0)) < quiet {
				continue
			}
			asked[vendor(bid)] = true
			out = append(out, Orphan{BundleID: bid, Path: s.Links.Display(s.Tree.Path(c)), Bytes: s.Tree.Reclaimable(c), Alloc: n.Alloc})
		}
	}
	if len(out) == 0 {
		return nil, true
	}
	known, ok := s.spotlight(ctx, asked)
	if !ok {
		return nil, false
	}
	out = slices.DeleteFunc(out, func(o Orphan) bool { return known[vendor(o.BundleID)] })
	slices.SortFunc(out, func(a, b Orphan) int { return cmp.Compare(b.Alloc, a.Alloc) })
	return out, true
}

// spotlight returns which of these makers have a bundle somewhere on the Mac,
// outside the Trash. One query asks for all of them.
func (s Source) spotlight(ctx context.Context, vendors map[string]bool) (map[string]bool, bool) {
	var q []string
	for v := range vendors {
		// bundleLike limited the labels to letters, digits, '-' and '_': nothing to escape.
		q = append(q, `kMDItemCFBundleIdentifier == "`+v+`.*"c`)
	}
	slices.Sort(q)
	out, err := s.Env.Run(ctx, "", "mdfind", "-attr", "kMDItemCFBundleIdentifier", strings.Join(q, " || "))
	if err != nil {
		return nil, false
	}
	trash := filepath.Join(s.Env.Home(), ".Trash") + "/"
	known := map[string]bool{}
	for line := range strings.Lines(string(out)) {
		path, id, found := strings.Cut(strings.TrimSpace(line), "   kMDItemCFBundleIdentifier = ")
		if found && !strings.HasPrefix(path, trash) {
			known[vendor(id)] = true
		}
	}
	return known, true
}

// vendor is the first two labels: com.example of com.example.App.
func vendor(id string) string {
	labels := strings.SplitN(strings.ToLower(id), ".", 3)
	if len(labels) < 2 {
		return strings.ToLower(id)
	}
	return labels[0] + "." + labels[1]
}

func apple(id string) bool {
	l := strings.ToLower(id)
	return l == "com.apple" || strings.HasPrefix(l, "com.apple.")
}

// bundleLike: reverse-DNS with at least three labels, a short alphabetic
// top-level label and nothing but letters, digits, '-' and '_'.
func bundleLike(id string) bool {
	labels := strings.Split(id, ".")
	if len(labels) < 3 || len(labels[0]) < 2 || len(labels[0]) > 6 {
		return false
	}
	for i, l := range labels {
		if l == "" || strings.ContainsFunc(l, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_')
		}) {
			return false
		}
		if i == 0 && strings.ContainsFunc(l, func(r rune) bool { return !(r >= 'a' && r <= 'z') }) {
			return false
		}
	}
	return true
}
