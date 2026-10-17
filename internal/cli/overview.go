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
	rep := report.Build(res, a.Links, Version)
	if g.json {
		return emitJSON(cmd.OutOrStdout(), rep)
	}
	if len(rep.Findings) > 10 {
		rep.Findings = rep.Findings[:10]
	}
	rep.Breakdown = rep.Breakdown[:1]
	if err := report.WriteText(cmd.OutOrStdout(), rep, 100); err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "\nMore: ebb serve --open (visual explorer) · ebb findings · ebb report --format html -o ebb.html")
	return err
}
