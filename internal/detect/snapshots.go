package detect

import (
	"context"
	"fmt"

	"github.com/doguyilmaz/ebb/internal/finding"
)

type snapshots struct{}

func (snapshots) ID() string { return "snapshots" }
func (snapshots) Pass() int  { return 1 }

// Detect explains local Time Machine snapshots. Their size is not reported
// by macOS without root, so the finding carries no byte count.
func (snapshots) Detect(_ context.Context, c *Context) ([]finding.Finding, error) {
	snaps := c.Volume.Snapshots
	if len(snaps) == 0 {
		return nil, nil
	}
	f := finding.Finding{ID: "snapshots", RuleID: "snapshots", Title: "Local Time Machine snapshots",
		Category: "snapshot", Owner: "Time Machine", Safety: finding.System,
		Why:     "Time Machine keeps hourly snapshots on the internal disk while the backup disk is away. macOS deletes them when space runs low, but they show up as used or purgeable space until then.",
		Impact:  "Deleting them loses the ability to restore from those points in time; your Time Machine backups are not affected.",
		Actions: []finding.Action{{Kind: "manual", Hint: "tmutil deletelocalsnapshots /   (may ask for an administrator password)"}}}
	for _, s := range snaps {
		f.Evidence = append(f.Evidence, fmt.Sprintf("snapshot from %s", s.Date.Format("2006-01-02 15:04")))
	}
	return []finding.Finding{f}, nil
}
