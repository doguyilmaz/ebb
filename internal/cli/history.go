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
			a, err := g.app()
			if err != nil {
				return err
			}
			var minSize rules.Size
			if err := minSize.UnmarshalText([]byte(minDelta)); err != nil {
				return err
			}
			metas, err := a.History.List()
			if err != nil {
				return err
			}
			if len(metas) == 0 {
				return errors.New("no scans yet — run: ebb scan")
			}
			toID := metas[0].ID
			if len(args) == 2 {
				toID = args[1]
			}
			to, err := a.History.Load(toID)
			if err != nil {
				return err
			}
			var from *history.Snapshot
			if len(args) >= 1 {
				from, err = a.History.Load(args[0])
			} else {
				from, err = a.History.Previous(to.Meta.Root, to.Meta.Time)
			}
			if err != nil {
				return fmt.Errorf("no earlier scan of %s to compare with", to.Meta.Root)
			}
			changes := history.Explaining(history.Diff(from, to, uint64(minSize)))
			changes = changes[:min(n, len(changes))]
			if g.json {
				if changes == nil {
					changes = []history.Change{}
				}
				return emitJSON(cmd.OutOrStdout(), map[string]any{"from": from.Meta, "to": to.Meta, "changes": changes})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s → %s\n", from.Meta.Time.Local().Format("2006-01-02 15:04"), to.Meta.Time.Local().Format("2006-01-02 15:04"))
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
