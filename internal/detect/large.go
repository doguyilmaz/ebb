package detect

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/units"
)

type large struct{}

func (large) ID() string { return "large" }
func (large) Pass() int  { return 2 }

// bundles are folders macOS treats as one document; a large file inside is
// reported as the whole bundle, because removing part of one breaks it.
var bundles = []string{".xcresult", ".xcarchive", ".bundle", ".framework", ".sparsebundle", ".playground", ".docset",
	".vmwarevm", ".pvm", ".utm", ".avd"}

// libraries hold the person's photos, music, films and projects: their space
// is explained by user-data rules, never offered as one large file.
var libraries = []string{".photoslibrary", ".photolibrary", ".musiclibrary", ".tvlibrary", ".imovielibrary", ".fcpbundle",
	".logicx", ".band", ".aplibrary"}

// Detect lists big files in the person's own folders that no other finding
// explains. ~/Library, dot-folders, apps and system locations are left out:
// what lives there belongs to an app or tool and is handled by their rules.
func (large) Detect(_ context.Context, c *Context) ([]finding.Finding, error) {
	home := c.Env.Home()
	f := finding.Finding{RuleID: "large-files", Title: "Large files", Category: "large-file", Safety: finding.Caution,
		Why:     fmt.Sprintf("Files of %s or more in your own folders that no other finding accounts for.", units.Bytes(c.LargeFile)),
		Impact:  "Check each one before removing it; they go to the Trash.",
		Actions: []finding.Action{{Kind: "trash"}}}
	seen := map[string]bool{}
	homeID, ok := c.node(home)
	if !ok {
		return nil, nil
	}
	for _, id := range c.Tree.Top(homeID, 400) {
		n := c.Tree.Node(id)
		if n.Alloc < c.LargeFile {
			break
		}
		path := c.Links.Display(c.Tree.Path(id))
		rel, ok := strings.CutPrefix(path, home+"/")
		if !ok || strings.HasPrefix(rel, "Library/") || strings.HasPrefix(rel, ".") || (c.Covered != nil && c.Covered(path)) ||
			slices.ContainsFunc(strings.Split(rel, "/"), func(p string) bool {
				return slices.ContainsFunc(libraries, func(ext string) bool { return strings.HasSuffix(p, ext) })
			}) {
			continue
		}
		target := id
		if b := outermostBundle(rel); b != "" {
			if strings.HasSuffix(b, ".app") {
				continue
			}
			bid, ok := c.node(home + "/" + b)
			if !ok {
				continue
			}
			target, path = bid, home+"/"+b
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		tn := c.Tree.Node(target)
		f.Targets = append(f.Targets, c.target(target))
		f.Evidence = append(f.Evidence, fmt.Sprintf("%s: %s, modified %s", path, units.Bytes(tn.Alloc), time.Unix(tn.MaxMtime, 0).Format("2006-01-02")))
	}
	if len(f.Targets) == 0 {
		return nil, nil
	}
	return []finding.Finding{finish(f)}, nil
}

// outermostBundle returns the first path prefix of rel that is a bundle or
// an app, or "".
func outermostBundle(rel string) string {
	parts := strings.Split(rel, "/")
	for i, p := range parts[:len(parts)-1] {
		if strings.HasSuffix(p, ".app") || slices.ContainsFunc(bundles, func(ext string) bool { return strings.HasSuffix(p, ext) }) {
			return strings.Join(parts[:i+1], "/")
		}
	}
	return ""
}
