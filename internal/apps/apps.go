// Package apps finds the apps on the scanned disk, what each keeps in the
// Library besides its bundle, and what apps that are gone left behind.
package apps

import (
	"cmp"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/plist"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

type App struct {
	Name          string           `json:"name"`
	BundleID      string           `json:"bundleId"`
	Version       string           `json:"version,omitempty"`
	Path          string           `json:"path"`
	Bytes         uint64           `json:"bytes"`
	Alloc         uint64           `json:"alloc"`
	Apple         bool             `json:"apple"`     // made by Apple: ebb never uninstalls it
	RootOwned     bool             `json:"rootOwned"` // installed as root (App Store, installers): Finder asks for a password
	Executable    string           `json:"executable"`
	Leftovers     []finding.Target `json:"leftovers"`
	LeftoverBytes uint64           `json:"leftoverBytes"`
}

func (a App) Total() uint64 { return a.Bytes + a.LeftoverBytes }

type Source struct {
	Env   sys.Env
	Tree  *tree.Tree
	Links volume.Firmlinks
}

// place is a Library folder where apps keep things, and how an entry there
// is named after the app.
type place struct {
	dir   string
	match func(entry, id string, names []string) bool
}

var places = []place{
	{"Containers", byID},
	{"Group Containers", byGroup},
	{"Application Support", byIDOrName},
	{"Caches", byIDOrName},
	{"Logs", byIDOrName},
	{"Saved Application State", withSuffix(".savedstate")},
	{"HTTPStorages", func(e, id string, _ []string) bool {
		return byID(e, id, nil) || withSuffix(".binarycookies")(e, id, nil)
	}},
	{"WebKit", byID},
	{"Application Scripts", byID},
	{"Preferences", withSuffix(".plist")},
	{"Cookies", withSuffix(".binarycookies")},
	{"LaunchAgents", withSuffix(".plist")},
}

// byID: the bundle ID itself, or one of its own sub-identifiers (helpers,
// extensions). Two-label IDs only match exactly, so a vendor's other apps stay out.
func byID(e, id string, _ []string) bool {
	return e == id || strings.Count(id, ".") >= 2 && strings.HasPrefix(e, id+".")
}

func byIDOrName(e, id string, names []string) bool {
	return byID(e, id, nil) || slices.Contains(names, e)
}

// byGroup: group.<id>, or <team id>.<id> as App Store apps name shared containers.
func byGroup(e, id string, _ []string) bool {
	if rest, ok := strings.CutPrefix(e, "group."); ok {
		return byID(rest, id, nil)
	}
	return len(e) > 11 && e[10] == '.' && alnum(e[:10]) && byID(e[11:], id, nil)
}

func alnum(s string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') })
}

func withSuffix(sfx string) func(string, string, []string) bool {
	return func(e, id string, _ []string) bool {
		base, ok := strings.CutSuffix(e, sfx)
		return ok && byID(base, id, nil)
	}
}

func (s Source) library(dir string) string { return filepath.Join(s.Env.Home(), "Library", dir) }

// entries lists each place once, for every app to match against.
func (s Source) entries() map[string][]string {
	out := map[string][]string{}
	for _, p := range places {
		des, _ := os.ReadDir(s.library(p.dir))
		for _, de := range des {
			out[p.dir] = append(out[p.dir], de.Name())
		}
	}
	return out
}

// target sizes a path from the tree; a small file the tree only counted in
// its folder is measured on disk.
func (s Source) target(path string) (finding.Target, bool) {
	t := finding.Target{Path: s.Links.Display(path)}
	if id, ok := s.Tree.Lookup(s.Links.Real(path)); ok {
		t.Bytes, t.Alloc = s.Tree.Reclaimable(id), s.Tree.Node(id).Alloc
		return t, true
	}
	// A folder the scan did not see is not vouched for; a small file was only counted in its folder.
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return t, false
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		t.Bytes = uint64(st.Blocks) * 512
		t.Alloc = t.Bytes
	}
	return t, true
}

// Installed lists the apps in /Applications and ~/Applications (and one
// folder deep, for Setapp and vendor folders) as the scan saw them, biggest
// first. complete is false when an app could not be identified: then what
// other apps left behind cannot be told apart from what it uses.
func (s Source) Installed(ctx context.Context) (out []App, complete bool) {
	var bundles []string
	for _, d := range []string{"/Applications", filepath.Join(s.Env.Home(), "Applications")} {
		id, ok := s.Tree.Lookup(s.Links.Real(d))
		if !ok {
			continue
		}
		for _, c := range s.Tree.Children(id) {
			n := s.Tree.Node(c)
			switch {
			case n.Kind != tree.KindDir:
			case strings.HasSuffix(n.Name, ".app"):
				bundles = append(bundles, s.Tree.Path(c))
			default:
				for _, cc := range s.Tree.Children(c) {
					if nn := s.Tree.Node(cc); nn.Kind == tree.KindDir && strings.HasSuffix(nn.Name, ".app") {
						bundles = append(bundles, s.Tree.Path(cc))
					}
				}
			}
		}
	}
	infos := make([]info, len(bundles))
	errs := make([]error, len(bundles))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, b := range bundles {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			infos[i], errs[i] = readInfo(ctx, s.Env, b)
		})
	}
	wg.Wait()
	complete = len(bundles) > 0
	entries := s.entries()
	for i, b := range bundles {
		in := infos[i]
		if errs[i] != nil || in.id == "" {
			complete = false
			continue
		}
		a := App{Name: in.name, BundleID: in.id, Version: in.version, Executable: in.exec, Path: s.Links.Display(b),
			Apple: strings.HasPrefix(strings.ToLower(in.id), "com.apple."), RootOwned: rootOwned(b)}
		if t, ok := s.target(b); ok {
			a.Bytes, a.Alloc = t.Bytes, t.Alloc
		}
		a.Leftovers = s.leftovers(entries, in.id, []string{in.name, strings.TrimSuffix(filepath.Base(b), ".app")})
		for _, t := range a.Leftovers {
			a.LeftoverBytes += t.Bytes
		}
		out = append(out, a)
	}
	slices.SortFunc(out, func(x, y App) int { return cmp.Or(cmp.Compare(y.Total(), x.Total()), cmp.Compare(x.Name, y.Name)) })
	return out, complete
}

func (s Source) leftovers(entries map[string][]string, id string, names []string) []finding.Target {
	id = strings.ToLower(id)
	var ln []string
	for _, n := range names {
		// Very short names would match folders that are not the app's.
		if n = strings.ToLower(n); len(n) >= 3 && !slices.Contains(ln, n) {
			ln = append(ln, n)
		}
	}
	var out []finding.Target
	for _, p := range places {
		for _, e := range entries[p.dir] {
			if !p.match(strings.ToLower(e), id, ln) {
				continue
			}
			if t, ok := s.target(filepath.Join(s.library(p.dir), e)); ok {
				out = append(out, t)
			}
		}
	}
	slices.SortFunc(out, func(a, b finding.Target) int { return cmp.Compare(b.Bytes, a.Bytes) })
	return out
}

func rootOwned(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && st.Uid == 0
}

type info struct{ id, name, version, exec string }

type cacheKey struct {
	path  string
	mtime time.Time
	size  int64
}

var infoCache sync.Map

// readInfo reads a bundle's Info.plist (iOS apps keep theirs in WrappedBundle),
// remembered while the file stays the same.
func readInfo(ctx context.Context, env sys.Env, bundle string) (info, error) {
	var p string
	var fi os.FileInfo
	var err error
	for _, c := range []string{filepath.Join(bundle, "Contents", "Info.plist"), filepath.Join(bundle, "WrappedBundle", "Info.plist")} {
		if fi, err = os.Stat(c); err == nil {
			p = c
			break
		}
	}
	if p == "" {
		return info{}, err
	}
	key := cacheKey{p, fi.ModTime(), fi.Size()}
	if v, ok := infoCache.Load(key); ok {
		return v.(info), nil
	}
	m, err := plist.Strings(ctx, env, p, "CFBundleIdentifier", "CFBundleName", "CFBundleDisplayName", "CFBundleShortVersionString", "CFBundleExecutable")
	if err != nil {
		return info{}, err
	}
	in := info{id: m["CFBundleIdentifier"], name: cmp.Or(m["CFBundleDisplayName"], m["CFBundleName"], strings.TrimSuffix(filepath.Base(bundle), ".app")),
		version: m["CFBundleShortVersionString"], exec: m["CFBundleExecutable"]}
	infoCache.Store(key, in)
	return in, nil
}
