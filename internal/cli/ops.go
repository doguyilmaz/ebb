package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/remote"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/units"
)

func cleanPath(p string) string { return filepath.Clean(p) }

func newApproveCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "approve <plan>",
		Short: "Approve a plan an AI agent made through the running ebb",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := remote.Discover(sys.Real())
			if err != nil {
				return err
			}
			if err := c.Do(context.Background(), "POST", "/plans/"+args[0]+"/approve", nil, nil); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "approved %s\n", args[0])
			return err
		},
	}
}

func newOpsCmd(g *globals) *cobra.Command {
	var n int
	cmd := &cobra.Command{
		Use:   "ops",
		Short: "What ebb deleted, trashed or ran",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			ops, err := a.Ops(n)
			if err != nil {
				return err
			}
			if g.json {
				return emitJSON(cmd.OutOrStdout(), ops)
			}
			var rows [][]string
			for _, op := range ops {
				rows = append(rows, []string{op.Time.Local().Format("2006-01-02 15:04"), op.Action, op.Result, units.Bytes(op.Bytes), op.Path, op.ID})
			}
			printTable(cmd.OutOrStdout(), []string{"TIME", "ACTION", "RESULT", "SIZE", "PATH", "ID"}, rows)
			return nil
		},
	}
	cmd.Flags().IntVarP(&n, "number", "n", 50, "how many")
	return cmd
}

func newUndoCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "undo [op]",
		Short: "Put an item ebb moved to the Trash back (default: the latest)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			id := ""
			if len(args) == 1 {
				id = args[0]
			}
			if err := a.Undo(id); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "restored; run ebb scan to refresh the numbers")
			return err
		},
	}
}

func newNoteCmd(g *globals) *cobra.Command {
	var rm bool
	cmd := &cobra.Command{
		Use:   "note <finding|path> [text]",
		Short: "Keep a note on a finding or a path; it shows in the UI and every report",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			key := ""
			if last, err := a.Last(); err == nil && slices.ContainsFunc(last.Findings, func(f finding.Finding) bool { return f.ID == args[0] }) {
				key = "finding:" + args[0]
			} else {
				abs, err := absPath(args[0])
				if err != nil {
					return err
				}
				key = "path:" + a.Links.Display(abs)
			}
			text := ""
			if len(args) == 2 && !rm {
				text = args[1]
			}
			return a.SetNote(key, text)
		},
	}
	cmd.Flags().BoolVar(&rm, "rm", false, "remove the note")
	return cmd
}

func newIgnoreCmd(g *globals) *cobra.Command {
	var undo bool
	cmd := &cobra.Command{
		Use:   "ignore <finding>",
		Short: "Hide a finding until it grows by half",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			if undo {
				return a.Unignore(args[0])
			}
			return a.Ignore(args[0])
		},
	}
	cmd.Flags().BoolVar(&undo, "undo", false, "show it again")
	return cmd
}

func newProtectCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{Use: "protect", Short: "Paths ebb must never remove (globs, ~ allowed)"}
	change := func(add bool) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			s := a.Settings()
			if add {
				if !slices.Contains(s.Protect, args[0]) {
					s.Protect = append(s.Protect, args[0])
				}
			} else {
				s.Protect = slices.DeleteFunc(s.Protect, func(p string) bool { return p == args[0] })
			}
			_, err = a.SaveSettings(s)
			return err
		}
	}
	cmd.AddCommand(
		&cobra.Command{Use: "add <glob>", Args: cobra.ExactArgs(1), RunE: change(true)},
		&cobra.Command{Use: "rm <glob>", Args: cobra.ExactArgs(1), RunE: change(false)},
		&cobra.Command{Use: "ls", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			for _, p := range a.Settings().Protect {
				fmt.Fprintln(cmd.OutOrStdout(), p)
			}
			return nil
		}},
	)
	return cmd
}
