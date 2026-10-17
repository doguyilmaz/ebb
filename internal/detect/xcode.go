package detect

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/plist"
	"github.com/doguyilmaz/ebb/internal/tree"
)

type xcode struct{}

func (xcode) ID() string { return "xcode" }
func (xcode) Pass() int  { return 1 }

func (xcode) Detect(ctx context.Context, c *Context) ([]finding.Finding, error) {
	var fs []finding.Finding
	if f, ok := c.oldXcodes(ctx); ok {
		fs = append(fs, f)
	}
	if id, ok := c.node(c.home("Library/Developer/Xcode/Archives")); ok && c.Tree.Node(id).Alloc >= 100<<20 {
		n := 0
		c.Tree.WalkDirs(id, func(d tree.Node) bool {
			if strings.HasSuffix(d.Name, ".xcarchive") {
				n++
				return false
			}
			return true
		})
		plural := "s"
		if n == 1 {
			plural = ""
		}
		fs = append(fs, finish(finding.Finding{RuleID: "xcode.archives", Title: "Xcode archives", Category: "build-artifact", Owner: "Xcode",
			Safety:   finding.Caution,
			Targets:  []finding.Target{c.target(id)},
			Why:      "Every Product → Archive keeps the full app build and its debug symbols.",
			Impact:   "Without an archive's dSYMs, crash reports from that build can no longer be symbolicated. Keep archives of builds that are live in the App Store.",
			Evidence: []string{fmt.Sprintf("%d archive%s", n, plural)},
			Actions:  []finding.Action{{Kind: "trash"}}}))
	}
	return fs, nil
}

// oldXcodes offers every Xcode except the newest and the one xcode-select
// points at.
func (c *Context) oldXcodes(ctx context.Context) (finding.Finding, bool) {
	apps, ok := c.node("/Applications")
	if !ok {
		return finding.Finding{}, false
	}
	selected := ""
	if out, err := c.Env.Run(ctx, "", "xcode-select", "-p"); err == nil {
		selected, _, _ = strings.Cut(strings.TrimSpace(string(out)), "/Contents/Developer")
	}
	type app struct {
		id      tree.ID
		path    string
		version string
	}
	var found []app
	for _, id := range c.Tree.Children(apps) {
		n := c.Tree.Node(id)
		if ok, _ := doublestar.Match("Xcode*.app", n.Name); !ok {
			continue
		}
		path := c.Links.Display(c.Tree.Path(id))
		var info struct{ CFBundleShortVersionString string }
		_ = plist.DecodeFile(ctx, c.Env, filepath.Join(c.Tree.Path(id), "Contents", "Info.plist"), &info)
		found = append(found, app{id, path, info.CFBundleShortVersionString})
	}
	newest := ""
	for _, a := range found {
		if compareVersions(a.version, newest) > 0 {
			newest = a.version
		}
	}
	f := finding.Finding{RuleID: "xcode.old", Title: "Older Xcode versions", Category: "sdk", Owner: "Xcode", Safety: finding.Caution,
		Why:     "Each Xcode is a 10+ GB app; side-by-side versions from betas and upgrades stay until removed.",
		Impact:  "Projects that need that exact Xcode version must install it again.",
		Actions: []finding.Action{{Kind: "trash", Quit: []string{"Xcode"}}}}
	for _, a := range found {
		if a.version == newest || a.path == selected {
			continue
		}
		f.Targets = append(f.Targets, c.target(a.id))
		f.Evidence = append(f.Evidence, fmt.Sprintf("%s is Xcode %s (newest installed: %s)", filepath.Base(a.path), a.version, newest))
	}
	if len(f.Targets) == 0 {
		return f, false
	}
	return finish(f), true
}
