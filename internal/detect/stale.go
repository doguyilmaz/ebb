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

type artifact struct {
	what string
	// markers lists the project files that make this folder build output; a
	// "build" folder next to only a package.json, say, may hold real sources.
	markers []string
}

var (
	js       = []string{"package.json"}
	gradle   = []string{"build.gradle", "build.gradle.kts", "settings.gradle"}
	python   = []string{"pyproject.toml", "requirements.txt", "setup.py"}
	xcodeDir = []string{"xcode"}
)

// artifacts are folders a project's tools rebuild from its sources.
var artifacts = map[string]artifact{
	"node_modules":  {"JavaScript dependencies", js},
	".next":         {"Next.js build output", js},
	".expo":         {"Expo cache", js},
	".turbo":        {"Turborepo cache", js},
	".parcel-cache": {"Parcel cache", js},
	"target":        {"Rust and Maven build output", []string{"Cargo.toml", "pom.xml"}},
	"build":         {"Build output", append(append(slices.Clone(gradle), python...), xcodeDir...)},
	".gradle":       {"Gradle project cache", gradle},
	"Pods":          {"CocoaPods dependencies", []string{"Podfile"}},
	".venv":         {"Python virtual environment", python},
	"venv":          {"Python virtual environment", python},
	"DerivedData":   {"Xcode build output", xcodeDir},
}

func markerMask(names []string) uint16 {
	var m uint16
	for _, n := range names {
		if n == "xcode" {
			m |= tree.MarkerXcode
		} else {
			m |= tree.MarkerBit(n)
		}
	}
	return m
}

type stale struct{}

func (stale) ID() string { return "stale" }

// Pass 2: folders another finding already claims (package caches, say) are
// skipped, so their contents are never reported a second time.
func (stale) Pass() int { return 2 }

func (stale) Detect(_ context.Context, c *Context) ([]finding.Finding, error) {
	type hit struct{ artifact, project tree.ID }
	var hits []hit
	claimed := c.coveredIDs()
	markers := map[tree.ID]uint16{}
	home, ok := c.node(c.Env.Home())
	if !ok {
		return nil, nil
	}
	// Only the person's own projects: apps, editor extensions, global tool
	// installs and everything in ~/Library or dot-folders run from their
	// node_modules and break without them.
	c.Tree.WalkDirs(home, func(n tree.Node) bool {
		if claimed[n.ID] || strings.HasSuffix(n.Name, ".app") ||
			(n.Parent == home && (n.Name == "Library" || strings.HasPrefix(n.Name, "."))) {
			return false
		}
		markers[n.ID] = n.Markers
		if a, ok := artifacts[n.Name]; ok {
			if markers[n.Parent]&markerMask(a.markers) != 0 {
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
				Why:     fmt.Sprintf("%s of projects nobody has changed in %d+ days.", artifacts[name].what, int(c.StaleAfter.Hours()/24)),
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
