package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/history"
	"github.com/doguyilmaz/ebb/internal/rules"
	"github.com/doguyilmaz/ebb/internal/units"
)

func newHistoryCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Past scans",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			metas, err := a.History.List()
			if err != nil {
				return err
			}
			if g.json {
				if metas == nil {
					metas = []history.Meta{}
				}
				return emitJSON(cmd.OutOrStdout(), metas)
			}
			var rows [][]string
			for _, m := range metas {
				rows = append(rows, []string{m.ID, m.Time.Local().Format("2006-01-02 15:04"), units.Bytes(m.Used), units.Bytes(m.Scanned), thousands(m.Files), m.Root})
			}
			printTable(cmd.OutOrStdout(), []string{"ID", "TIME", "DISK USED", "SCANNED", "FILES", "ROOT"}, rows)
			return nil
		},
	}
}

func newDiffCmd(g *globals) *cobra.Command {
	var n int
	var minDelta string
	cmd := &cobra.Command{
		Use:   "diff [from] [to]",
		Short: "What grew or shrank between two scans (default: the last two of the same folder)",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if n < 0 {
				return errors.New("-n must be 0 or more")
			}
			a, err := g.app()
			if err != nil {
				return err
			}
			var minSize rules.Size
			if err := minSize.UnmarshalText([]byte(minDelta)); err != nil {
				return err
			}
			from, to := "", ""
			if len(args) >= 1 {
				from = args[0]
			}
			if len(args) == 2 {
				to = args[1]
			}
			d, err := a.Diff(from, to, uint64(minSize))
			if err != nil {
				return err
			}
			changes := d.Changes[:min(n, len(d.Changes))]
			if g.json {
				d.Changes = changes
				return emitJSON(cmd.OutOrStdout(), d)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s → %s\n", d.From.Time.Local().Format("2006-01-02 15:04"), d.To.Time.Local().Format("2006-01-02 15:04"))
			var rows [][]string
			for _, c := range changes {
				sign := "+"
				d := uint64(c.Delta)
				if c.Delta < 0 {
					sign, d = "-", uint64(-c.Delta)
				}
				rows = append(rows, []string{sign + units.Bytes(d), units.Bytes(c.Before), units.Bytes(c.After), c.Path})
			}
			printTable(cmd.OutOrStdout(), []string{"CHANGE", "BEFORE", "AFTER", "PATH"}, rows)
			return nil
		},
	}
	cmd.Flags().IntVarP(&n, "number", "n", 30, "how many changes to show")
	cmd.Flags().StringVar(&minDelta, "min", "10MB", "ignore changes smaller than this")
	return cmd
}
