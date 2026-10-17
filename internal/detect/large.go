package detect

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/units"
)

type large struct{}

func (large) ID() string { return "large" }
func (large) Pass() int  { return 2 }

// Detect lists big single files no other finding explains. Files inside app
// bundles and system folders are left out: deleting one would break its owner.
func (large) Detect(_ context.Context, c *Context) ([]finding.Finding, error) {
	f := finding.Finding{RuleID: "large-files", Title: "Large files", Category: "large-file", Safety: finding.Caution,
		Why:     fmt.Sprintf("Single files of %s or more that no other finding accounts for.", units.Bytes(c.LargeFile)),
		Impact:  "Check each one before removing it; they go to the Trash.",
		Actions: []finding.Action{{Kind: "trash"}}}
	for _, id := range c.Tree.Top(c.Tree.Root(), 200) {
		n := c.Tree.Node(id)
		if n.Alloc < c.LargeFile {
			break
		}
		path := c.Links.Display(c.Tree.Path(id))
		if (c.Covered != nil && c.Covered(path)) || strings.Contains(path, ".app/") || strings.HasPrefix(path, "/System/") ||
			strings.HasPrefix(path, "/private/") || strings.HasPrefix(path, "/Library/Apple/") {
			continue
		}
		f.Targets = append(f.Targets, c.target(id))
		f.Evidence = append(f.Evidence, fmt.Sprintf("%s: %s, modified %s", path, units.Bytes(n.Alloc), time.Unix(n.MaxMtime, 0).Format("2006-01-02")))
	}
	if len(f.Targets) == 0 {
		return nil, nil
	}
	return []finding.Finding{finish(f)}, nil
}
