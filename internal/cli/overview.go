package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/report"
)

// runOverview reuses a scan younger than an hour, or scans first.
func runOverview(g *globals, cmd *cobra.Command) error {
	a, err := g.app()
	if err != nil {
		return err
	}
	res, err := a.Current(cmd.Context(), time.Hour, app.ScanOptions{}, progress(os.Stderr))
	if err != nil {
		return err
	}
	rep := report.Build(a.View(res), a.Links, Version)
	if g.json {
		return emitJSON(cmd.OutOrStdout(), rep)
	}
	if len(rep.Findings) > 10 {
		rep.Findings = rep.Findings[:10]
	}
	rep.Breakdown = topFolders(rep.Breakdown, 8)
	if err := report.WriteText(cmd.OutOrStdout(), rep, report.Width); err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "\nMore: ebb open (visual explorer) · ebb findings · ebb report --format html -o ebb.html")
	return err
}

// topFolders keeps the root and its n largest folders. Small files and the
// rest of a level carry their parent's path, so they would read as the root.
func topFolders(rows []report.Row, n int) []report.Row {
	out := rows[:1:1]
	for _, r := range rows[1:] {
		if r.Depth == 1 && r.Path != rows[0].Path && len(out) <= n {
			out = append(out, r)
		}
	}
	return out
}
