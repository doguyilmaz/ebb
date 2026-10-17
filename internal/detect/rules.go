package detect

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/rules"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/units"
)

// rulesDetector applies the data rules in load order. A location matched by
// one rule is claimed and never reported by a later one, so specific rules
// win over the generic catch-alls at the end.
type rulesDetector struct{}

func (rulesDetector) ID() string { return "rules" }
func (rulesDetector) Pass() int  { return 1 }

func (rulesDetector) Detect(_ context.Context, c *Context) ([]finding.Finding, error) {
	var claimed []string
	var out []finding.Finding
	for _, r := range c.Rules.Rules {
		var ids []tree.ID
		for _, p := range r.Paths {
			ids = append(ids, rules.Resolve(c.Tree, c.Links.Real(rules.Expand(c.Env, p)))...)
		}
		if r.Match != nil {
			ids = append(ids, matchDirs(c, r.Match)...)
		}
		var targets []finding.Target
		for _, id := range ids {
			path := c.Tree.Path(id)
			if slices.ContainsFunc(claimed, func(p string) bool { return overlaps(p, path) }) {
				continue
			}
			claimed = append(claimed, path)
			n := c.Tree.Node(id)
			if n.Alloc < uint64(r.MinTarget) {
				continue
			}
			targets = append(targets, finding.Target{Path: c.Links.Display(path), Bytes: c.Tree.Reclaimable(id), Alloc: n.Alloc})
		}
		if len(targets) == 0 {
			continue
		}
		slices.SortFunc(targets, func(a, b finding.Target) int { return cmp.Compare(b.Alloc, a.Alloc) })
		f := finding.Finding{
			ID: finding.ID(r.ID, targets[0].Path), RuleID: r.ID, Title: r.Title, Category: r.Category, Owner: r.Owner,
			Safety: r.Safety, Targets: targets, Why: r.Why, Impact: r.Impact, Evidence: evidence(c, targets),
		}
		if r.Action.Kind != "" {
			f.Actions = []finding.Action{r.Action}
		}
		f.Total()
		if f.Alloc < uint64(r.MinSize) {
			continue
		}
		out = append(out, f)
	}
	return out, nil
}

func evidence(c *Context, targets []finding.Target) []string {
	if len(targets) == 1 {
		id, ok := c.Tree.Lookup(c.Links.Real(targets[0].Path))
		if ok && c.Tree.Node(id).Kind == tree.KindDir {
			n := c.Tree.Node(id)
			return []string{fmt.Sprintf("%d entries, %d files", len(c.Tree.Children(id))+int(n.SmallCount), n.Files)}
		}
		return nil
	}
	var parts []string
	for _, t := range targets[:min(3, len(targets))] {
		parts = append(parts, fmt.Sprintf("%s (%s)", lastElem(t.Path), units.Bytes(t.Alloc)))
	}
	return []string{fmt.Sprintf("%d locations; largest: %s", len(targets), strings.Join(parts, ", "))}
}

func lastElem(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func matchDirs(c *Context, m *rules.Match) []tree.ID {
	roots := []tree.ID{c.Tree.Root()}
	if len(m.Under) > 0 {
		roots = roots[:0]
		for _, u := range m.Under {
			if id, ok := c.Tree.Lookup(c.Links.Real(rules.Expand(c.Env, u))); ok {
				roots = append(roots, id)
			}
		}
	}
	var out []tree.ID
	for _, root := range roots {
		parentProject := map[tree.ID]bool{}
		c.Tree.WalkDirs(root, func(n tree.Node) bool {
			parentProject[n.ID] = n.Flags&tree.FlagProject != 0
			if ok, _ := doublestar.Match(m.Name, n.Name); ok && n.ID != root && (!m.Project || parentProject[n.Parent]) {
				out = append(out, n.ID)
				return false
			}
			return true
		})
	}
	return out
}
