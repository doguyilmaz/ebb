package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/units"
)

func newFindingsCmd(g *globals) *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:   "findings",
		Short: "What takes space, why, and how safe it is to remove",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			last, err := a.Last()
			if err != nil {
				return err
			}
			var fs []finding.Finding
			for _, f := range a.View(last).Findings {
				if category == "" || f.Category == category {
					fs = append(fs, f)
				}
			}
			if g.json {
				if fs == nil {
					fs = []finding.Finding{}
				}
				return emitJSON(cmd.OutOrStdout(), fs)
			}
			var rows [][]string
			for _, f := range fs {
				act := "-"
				if f.Actionable() {
					act = f.Actions[0].Kind
				} else if len(f.Actions) > 0 {
					act = "manual"
				}
				rows = append(rows, []string{units.Bytes(f.Bytes), string(f.Safety), act, f.Title, f.ID})
			}
			printTable(cmd.OutOrStdout(), []string{"SIZE", "SAFETY", "ACTION", "FINDING", "ID"}, rows)
			if len(fs) == 0 {
				fmt.Fprintln(os.Stderr, "no findings")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "only this category (package-cache, simulator, stale-project, …)")
	return cmd
}
