package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/api"
)

func newWebCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{Use: "web", Short: "Which web pages may use this machine's ebb"}
	show := func(cmd *cobra.Command, w api.WebAccess) error {
		if g.json {
			return emitJSON(cmd.OutOrStdout(), w)
		}
		out := cmd.OutOrStdout()
		fmt.Fprintln(out, "allowed:", w.Allowed)
		fmt.Fprintln(out, "paired: ", w.Paired)
		return nil
	}
	cmd.AddCommand(
		&cobra.Command{Use: "ls", Short: "Allowed and paired origins", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			return show(cmd, a.WebAccess())
		}},
		&cobra.Command{Use: "allow <origin>", Short: "Let pages on origin connect through ebb open", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			w, err := a.AllowWebOrigin(args[0])
			if err != nil {
				return err
			}
			return show(cmd, w)
		}},
		&cobra.Command{Use: "deny <origin>", Short: "Revoke an origin and every token it was given", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			w, err := a.DenyWebOrigin(args[0])
			if err != nil {
				return err
			}
			return show(cmd, w)
		}},
	)
	return cmd
}
