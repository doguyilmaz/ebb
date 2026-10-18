package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/clean"
	"github.com/doguyilmaz/ebb/internal/units"
)

func newAppsCmd(g *globals) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Installed apps with what each keeps in your Library, biggest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			res, err := a.Apps(cmd.Context())
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if g.json {
				return emitJSON(out, res)
			}
			if res.Note != "" {
				fmt.Fprintln(out, res.Note)
			}
			var rows [][]string
			for _, ap := range res.Apps[:min(limit, len(res.Apps))] {
				who := ""
				switch {
				case ap.Apple:
					who = "Apple"
				case ap.RootOwned:
					who = "all users"
				}
				rows = append(rows, []string{units.Bytes(ap.Total()), units.Bytes(ap.Bytes), units.Bytes(ap.LeftoverBytes), who, ap.Name, ap.BundleID})
			}
			printTable(out, []string{"TOTAL", "APP", "LIBRARY", "", "NAME", "BUNDLE ID"}, rows)
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 30, "how many")
	return cmd
}

func newUninstallCmd(g *globals) *cobra.Command {
	var dry bool
	cmd := &cobra.Command{
		Use:   "uninstall <name|bundle-id>",
		Short: "Move an app and everything it keeps in your Library to the Trash",
		Long: "Plans the app bundle and its Library data (containers, caches, preferences, saved state, launch agents)\n" +
			"for the Trash, shows the plan and asks first. The app must not be running; launch agents are stopped\n" +
			"before they go. Apple's apps are never uninstalled.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			return runPlan(cmd, a, clean.Request{App: args[0], Source: "cli"}, dry, false)
		},
	}
	cmd.Flags().BoolVar(&dry, "dry-run", false, "only show the plan")
	return cmd
}
