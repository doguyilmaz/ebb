package detect

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/tree"
)

// artifacts are folders a project's tools rebuild from its sources.
var artifacts = map[string]string{
	"node_modules": "JavaScript dependencies", "target": "Rust and Maven build output", "build": "build output",
	".gradle": "Gradle project cache", "Pods": "CocoaPods dependencies", ".next": "Next.js build output",
	".expo": "Expo cache", ".turbo": "Turborepo cache", ".parcel-cache": "Parcel cache", ".venv": "Python virtual environment",
	"venv": "Python virtual environment", "DerivedData": "Xcode build output",
}

type stale struct{}

func (stale) ID() string { return "stale" }
func (stale) Pass() int  { return 1 }

func (stale) Detect(_ context.Context, c *Context) ([]finding.Finding, error) {
	type hit struct{ artifact, project tree.ID }
	var hits []hit
	project := map[tree.ID]bool{}
	c.Tree.WalkDirs(c.Tree.Root(), func(n tree.Node) bool {
		project[n.ID] = n.Flags&tree.FlagProject != 0
		if _, ok := artifacts[n.Name]; ok {
			if project[n.Parent] {
				hits = append(hits, hit{n.ID, n.Parent})
			}
			return false
		}
		return true
	})
	activity := map[tree.ID]time.Time{}
	lastChange := func(p tree.ID) time.Time {
		if t, ok := activity[p]; ok {
			return t
		}
		newest := c.Tree.Node(p).OwnMtime
		for _, ch := range c.Tree.Children(p) {
			n := c.Tree.Node(ch)
			if _, art := artifacts[n.Name]; !art {
				newest = max(newest, n.MaxMtime)
			}
		}
		activity[p] = time.Unix(newest, 0)
		return activity[p]
	}
	groups := map[string]*finding.Finding{}
	var order []string
	for _, h := range hits {
		last := lastChange(h.project)
		if c.Now.Sub(last) < c.StaleAfter {
			continue
		}
		name := c.Tree.Node(h.artifact).Name
		f, ok := groups[name]
		if !ok {
			f = &finding.Finding{RuleID: "stale." + strings.TrimPrefix(name, "."), Category: "stale-project", Safety: finding.Regenerable,
				Why:     fmt.Sprintf("%s of projects nobody has changed in %d+ days.", capitalize(artifacts[name]), int(c.StaleAfter.Hours()/24)),
				Impact:  "The project's install or build step recreates it the next time you work on it.",
				Actions: []finding.Action{{Kind: "delete"}}}
			groups[name] = f
			order = append(order, name)
		}
		f.Targets = append(f.Targets, c.target(h.artifact))
		f.Evidence = append(f.Evidence, fmt.Sprintf("%s: last change %s", c.Links.Display(c.Tree.Path(h.project)), last.Format("2006-01-02")))
	}
	var fs []finding.Finding
	for _, name := range order {
		f := groups[name]
		plural := "s"
		if len(f.Targets) == 1 {
			plural = ""
		}
		f.Title = fmt.Sprintf("%s in %d project%s untouched for %d+ days", name, len(f.Targets), plural, int(c.StaleAfter.Hours()/24))
		slices.Sort(f.Evidence)
		if out := finish(*f); out.Alloc >= 50<<20 {
			fs = append(fs, out)
		}
	}
	return fs, nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
