// Package clean turns findings and chosen paths into a reviewed plan and
// carries it out, refusing anything that changed, is protected, or lies
// outside what was scanned.
package clean

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/bmatcuk/doublestar/v4"
)

const dataVolume = "/System/Volumes/Data"

// Guard decides whether a path may be removed at all.
type Guard struct {
	Home    string
	Roots   []string // scanned roots; nothing outside them is touched
	Protect []string // the person's own globs (~ allowed)
	Keep    []string // ebb's own config and state: removing them would lift every protection
}

// subtrees are never touched, nor anything inside them.
var subtrees = []string{"/System", "/bin", "/sbin", "/usr", "/etc", "/private/etc", "/var/db", "/private/var/db",
	"/var/root", "/private/var/root", "/Library/Apple", "/Library/Keychains", "/Library/Security", "/cores"}

// canonical maps the data volume's real paths to the paths people see, so one
// list of protected locations covers both spellings.
func canonical(p string) string {
	if rest, ok := strings.CutPrefix(p, dataVolume+"/"); ok {
		return "/" + rest
	}
	if p == dataVolume {
		return "/"
	}
	return p
}

// isFile: the top-level rule guards folders like ~/Documents, not a download
// lying directly in the home folder. Anything unreadable counts as a folder.
func isFile(path string) bool {
	st, err := os.Lstat(path)
	return err == nil && !st.IsDir()
}

func within(p, root string) bool {
	return p == root || root == "/" || strings.HasPrefix(p, strings.TrimSuffix(root, "/")+"/")
}

// Check returns why path must not be removed, or nil. APFS ignores case and
// Unicode normalization, and a symlinked parent sends a path elsewhere, so
// the check runs on the name as given (case-insensitively), on the path the
// OS will actually reach, and on the identity (device and inode) of the
// protected folders that exist.
func (g Guard) Check(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("not a plain absolute path")
	}
	if err := g.checkName(path); err != nil {
		return err
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return nil
	}
	resolved := filepath.Join(dir, filepath.Base(path))
	rg := g.resolved()
	if err := rg.checkName(resolved); err != nil {
		return err
	}
	return rg.checkIdentity(resolved)
}

// resolved returns the guard with home and roots spelled as the OS resolves
// them, for comparing with resolved paths.
func (g Guard) resolved() Guard {
	r := Guard{Home: g.Home, Protect: g.Protect, Keep: g.Keep}
	if h, err := filepath.EvalSymlinks(g.Home); err == nil {
		r.Home = h
	}
	for _, root := range g.Roots {
		if x, err := filepath.EvalSymlinks(root); err == nil {
			root = x
		}
		r.Roots = append(r.Roots, root)
	}
	return r
}

var syncedHomes = []string{"Dropbox", "Google Drive", "OneDrive", "Box", "Box Sync"}

func cloudy(s string) bool {
	return strings.Contains(s, "Mobile Documents") || strings.Contains(s, "CloudStorage") || slices.Contains(syncedHomes, filepath.Base(s))
}

func (g Guard) protectedLists() (subs, exact, tops []string) {
	home := g.Home
	subs = append(slices.Clone(subtrees), home+"/Library/Keychains", home+"/Library/Mobile Documents", home+"/Library/CloudStorage")
	for _, d := range syncedHomes {
		subs = append(subs, home+"/"+d)
	}
	exact = []string{"/", "/Applications", "/Library", "/Users", "/Volumes", "/usr/local", "/opt", "/private", "/private/var",
		"/private/tmp", "/var", "/tmp", home, home + "/Library"}
	// Children of these are folders the system or the person relies on as a
	// whole (the data volume's hidden databases, mounted disks, homes).
	tops = []string{"/", dataVolume, home, "/Volumes", "/Users"}
	return
}

func (g Guard) checkName(path string) error {
	lower := strings.ToLower
	p := lower(canonical(path))
	if !slices.ContainsFunc(g.Roots, func(r string) bool { return within(p, lower(canonical(filepath.Clean(r)))) }) {
		return errors.New("outside the scanned folders")
	}
	subs, exact, tops := g.protectedLists()
	for _, s := range subs {
		if within(p, lower(s)) && !within(p, "/usr/local") {
			if cloudy(s) {
				return errors.New("inside a cloud-synced folder: deleting here deletes it on every device")
			}
			return fmt.Errorf("protected system location (%s)", s)
		}
	}
	if slices.ContainsFunc(exact, func(e string) bool { return lower(e) == p }) {
		return errors.New("protected folder")
	}
	parent := filepath.Dir(p)
	if slices.ContainsFunc(tops, func(t string) bool { return lower(t) == parent }) && !isFile(path) {
		return errors.New("top-level folder: remove what is inside it instead")
	}
	for _, k := range g.Keep {
		k = lower(canonical(filepath.Clean(k)))
		if within(p, k) || within(k, p) {
			return errors.New("ebb's own settings and history")
		}
	}
	for _, glob := range g.Protect {
		pat := glob
		if rest, ok := strings.CutPrefix(glob, "~/"); ok {
			pat = filepath.Join(g.Home, rest)
		}
		pat = lower(canonical(pat))
		if ok, _ := doublestar.Match(pat, p); ok {
			return fmt.Errorf("on your protect list (%s)", glob)
		}
		// Removing a folder removes what is inside it, protected or not.
		if base := staticBase(pat); base != "/" && within(base, p) {
			return fmt.Errorf("holds something on your protect list (%s)", glob)
		}
	}
	return nil
}

// staticBase is the part of a glob before its first wildcard segment.
func staticBase(pat string) string {
	var keep []string
	for _, seg := range strings.Split(pat, "/") {
		if strings.ContainsAny(seg, "*?[{") {
			break
		}
		keep = append(keep, seg)
	}
	if b := strings.Join(keep, "/"); b != "" {
		return b
	}
	return "/"
}

type fileID struct {
	dev int64
	ino uint64
}

func idOf(path string, follow bool) (fileID, bool) {
	var st syscall.Stat_t
	stat := syscall.Lstat
	if follow {
		stat = syscall.Stat
	}
	if stat(path, &st) != nil {
		return fileID{}, false
	}
	return fileID{int64(st.Dev), st.Ino}, true
}

func ids(paths []string) map[fileID]string {
	out := map[fileID]string{}
	for _, p := range paths {
		if id, ok := idOf(p, true); ok {
			out[id] = p
		}
	}
	return out
}

// checkIdentity matches the resolved path and its ancestors against the
// protected folders by device and inode, which no spelling can evade.
func (g Guard) checkIdentity(resolved string) error {
	subs, exact, tops := g.protectedLists()
	subIDs, exactIDs, topIDs, rootIDs := ids(subs), ids(exact), ids(tops), ids(g.Roots)
	usrLocal, _ := idOf("/usr/local", true)
	if id, ok := idOf(resolved, false); ok {
		if _, hit := exactIDs[id]; hit {
			return errors.New("protected folder")
		}
	}
	// The data volume is reached through firmlinks (/Users, /private) whose
	// parent chain runs over the system volume and never passes the root, and
	// both volumes report one device. For it the name check on the resolved
	// path, already done, is the whole answer; system locations have their
	// own protections and the system volume is read-only.
	inRoot := slices.ContainsFunc(g.Roots, func(r string) bool { return canonical(filepath.Clean(r)) == "/" })
	if id, ok := idOf(resolved, false); ok {
		// A scan never crosses into another disk, so neither does a cleanup:
		// a mounted volume reports its own device (the system and data
		// volumes of one Mac share theirs).
		if !slices.ContainsFunc(g.Roots, func(r string) bool { rid, ok := idOf(r, true); return ok && rid.dev == id.dev }) {
			return errors.New("on another disk than the one scanned")
		}
		if !inRoot {
			_, inRoot = rootIDs[id]
		}
	}
	first := true
	for dir := filepath.Dir(resolved); ; dir = filepath.Dir(dir) {
		id, ok := idOf(dir, true)
		if ok {
			if _, hit := topIDs[id]; hit && first && !isFile(resolved) {
				return errors.New("top-level folder: remove what is inside it instead")
			}
			if id == usrLocal {
				subIDs = nil
			}
			if s, hit := subIDs[id]; hit {
				if cloudy(s) {
					return errors.New("inside a cloud-synced folder: deleting here deletes it on every device")
				}
				return fmt.Errorf("protected system location (%s)", s)
			}
			if _, hit := rootIDs[id]; hit {
				inRoot = true
			}
		}
		first = false
		if dir == "/" {
			break
		}
	}
	if !inRoot && len(rootIDs) > 0 {
		return errors.New("outside the scanned folders")
	}
	return nil
}
