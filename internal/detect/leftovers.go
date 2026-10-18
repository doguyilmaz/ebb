package detect

import (
	"context"
	"fmt"
	"time"

	"github.com/doguyilmaz/ebb/internal/apps"
	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/units"
)

type leftovers struct{}

func (leftovers) ID() string { return "leftovers" }
func (leftovers) Pass() int  { return 1 }

// Detect reports what apps that are no longer installed left in the Library.
// It stays silent unless every installed app could be identified, since
// otherwise an app's own data could pass for a removed one's.
func (leftovers) Detect(ctx context.Context, c *Context) ([]finding.Finding, error) {
	src := apps.Source{Env: c.Env, Tree: c.Tree, Links: c.Links}
	installed, complete := src.Installed(ctx)
	if !complete {
		return nil, nil
	}
	quiet := c.StaleAfter
	if quiet == 0 {
		quiet = 30 * 24 * time.Hour
	}
	orphans, ok := src.Orphans(ctx, installed, c.Now, quiet)
	if !ok || len(orphans) == 0 {
		return nil, nil
	}
	f := finding.Finding{RuleID: "leftovers", Title: "Leftovers of removed apps", Category: "leftover", Safety: finding.Caution,
		Why: "Apps keep containers, caches and saved windows in your Library, and removing an app leaves them behind. No installed app " +
			"comes from the makers of these, and none was touched in " + fmt.Sprint(int(quiet.Hours()/24)) + " days.",
		Impact:  "If you install the app again, it starts fresh: its settings, sign-ins and local data are gone. They stay in the Trash until you empty it.",
		Actions: []finding.Action{{Kind: "trash"}}}
	byID := map[string]uint64{}
	var order []string
	for _, o := range orphans {
		f.Targets = append(f.Targets, finding.Target{Path: o.Path, Bytes: o.Bytes, Alloc: o.Alloc})
		if _, ok := byID[o.BundleID]; !ok {
			order = append(order, o.BundleID)
		}
		byID[o.BundleID] += o.Alloc
	}
	for _, id := range order[:min(len(order), 12)] {
		f.Evidence = append(f.Evidence, fmt.Sprintf("%s: %s, no app from this maker on this Mac", id, units.Bytes(byID[id])))
	}
	return []finding.Finding{finish(f)}, nil
}
