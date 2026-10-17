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
		if w.Pending != nil {
			state := "waiting for confirmation"
			if w.Pending.Confirmed {
				state = "confirmed, waiting for the page"
			}
			fmt.Fprintf(out, "pending: %s, code %s (%s)\n", w.Pending.Origin, w.Pending.Code, state)
		}
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
		&cobra.Command{Use: "allow <origin>", Short: "Let pages on origin ask to pair", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
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
		&cobra.Command{Use: "confirm <code>", Short: "Approve the page waiting to pair, by the code it shows", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			p, err := a.ConfirmPair(args[0])
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "confirmed %s; the page connects on its next try\n", p.Origin)
			return err
		}},
		&cobra.Command{Use: "reject", Short: "Discard the page waiting to pair", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			return a.RejectPair()
		}},
	)
	return cmd
}
