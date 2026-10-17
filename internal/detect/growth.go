package detect

import (
	"context"
	"fmt"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/history"
	"github.com/doguyilmaz/ebb/internal/units"
)

type growth struct{}

func (growth) ID() string { return "growth" }
func (growth) Pass() int  { return 2 }

// Detect compares the tree with the previous snapshot of the same root and
// names the deepest folders that grew by 1 GiB and 20 % or more.
func (growth) Detect(_ context.Context, c *Context) ([]finding.Finding, error) {
	if c.History == nil {
		return nil, nil
	}
	root := c.Links.Display(c.Tree.RootPath())
	prev, err := c.History.Previous(root, c.Now)
	if err != nil {
		return nil, nil
	}
	cur := &history.Snapshot{Dirs: history.FromTree(c.Tree, 10<<20, c.Links.Display)}
	since := prev.Meta.Time.Format("2006-01-02")
	f := finding.Finding{RuleID: "growth", Title: "Grew since " + since, Category: "growth", Safety: finding.Caution,
		Why:    "These folders grew the most since the previous scan; they explain where recent space went.",
		Impact: "Informational: look at what is inside before removing anything."}
	for _, ch := range history.Explaining(history.Diff(prev, cur, 1<<30)) {
		if ch.Delta <= 0 || (ch.Before > 0 && uint64(ch.Delta)*5 < ch.Before) {
			continue
		}
		f.Targets = append(f.Targets, finding.Target{Path: ch.Path, Bytes: uint64(ch.Delta), Alloc: ch.After})
		f.Evidence = append(f.Evidence, fmt.Sprintf("%s: %s → %s (+%s)", ch.Path, units.Bytes(ch.Before), units.Bytes(ch.After), units.Bytes(uint64(ch.Delta))))
	}
	if len(f.Targets) == 0 {
		return nil, nil
	}
	f = finish(f)
	return []finding.Finding{f}, nil
}
