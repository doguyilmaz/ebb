// Package clean turns findings and chosen paths into a reviewed plan and
// carries it out, refusing anything that changed, is protected, or lies
// outside what was scanned.
package clean

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

const dataVolume = "/System/Volumes/Data"

// Guard decides whether a path may be removed at all.
type Guard struct {
	Home    string
	Roots   []string // scanned roots; nothing outside them is touched
	Protect []string // the person's own globs (~ allowed)
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

func within(p, root string) bool {
	return p == root || root == "/" || strings.HasPrefix(p, strings.TrimSuffix(root, "/")+"/")
}

// Check returns why path must not be removed, or nil.
func (g Guard) Check(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("not a plain absolute path")
	}
	p := canonical(path)
	if !slices.ContainsFunc(g.Roots, func(r string) bool { return within(p, canonical(filepath.Clean(r))) }) {
		return errors.New("outside the scanned folders")
	}
	home := g.Home
	homeSubtrees := []string{home + "/Library/Keychains", home + "/Library/Mobile Documents", home + "/Library/CloudStorage"}
	for _, s := range append(slices.Clone(subtrees), homeSubtrees...) {
		if within(p, s) && !within(p, "/usr/local") {
			if strings.Contains(s, "Mobile Documents") || strings.Contains(s, "CloudStorage") {
				return errors.New("inside a cloud-synced folder: deleting here deletes it on every device")
			}
			return fmt.Errorf("protected system location (%s)", s)
		}
	}
	exact := []string{"/", "/Applications", "/Library", "/Users", "/Volumes", "/usr/local", "/opt", home, home + "/Library"}
	if slices.Contains(exact, p) {
		return errors.New("protected folder")
	}
	if parent := filepath.Dir(p); parent == home || parent == "/Volumes" || parent == "/Users" {
		return errors.New("top-level folder: remove what is inside it instead")
	}
	for _, glob := range g.Protect {
		pat := glob
		if rest, ok := strings.CutPrefix(glob, "~/"); ok {
			pat = filepath.Join(home, rest)
		}
		if ok, _ := doublestar.Match(pat, p); ok {
			return fmt.Errorf("on your protect list (%s)", glob)
		}
	}
	return nil
}
