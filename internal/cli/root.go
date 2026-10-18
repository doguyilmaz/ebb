// Package cli holds the thin Cobra layer. Commands parse flags, call into the
// core packages and print; they hold no logic of their own.
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/sys"
)

// Version is overridden at build time via -ldflags "-X ...cli.Version=v1.2.3".
var Version = "dev"

type globals struct {
	json    bool
	home    string
	verbose bool
}

// Main runs the CLI and returns the process exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	root := newRoot(stdout, stderr)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

func newRoot(stdout, stderr io.Writer) *cobra.Command {
	g := &globals{}
	root := &cobra.Command{
		Use:           "ebb",
		Short:         "See where your disk space went and win it back safely",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if g.home != "" {
				return os.Setenv(sys.HomeEnv, g.home)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error { return runOverview(g, cmd) },
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	pf := root.PersistentFlags()
	pf.BoolVar(&g.json, "json", false, "machine-readable JSON output")
	pf.StringVar(&g.home, "home", "", "treat DIR as the home directory (same as EBB_HOME)")
	pf.BoolVarP(&g.verbose, "verbose", "v", false, "verbose diagnostics on stderr")

	root.AddCommand(
		newScanCmd(g),
		newTopCmd(g),
		newFindCmd(g),
		newFindingsCmd(g),
		newReportCmd(g),
		newHistoryCmd(g),
		newMCPCmd(g),
		newAssistantCmd(g),
		newDiffCmd(g),
		newServeCmd(g),
		newDaemonCmd(g),
		newWebCmd(g),
		newCleanCmd(g),
		newAppsCmd(g),
		newDupesCmd(g),
		newUninstallCmd(g),
		newApproveCmd(g),
		newOpsCmd(g),
		newUndoCmd(g),
		newNoteCmd(g),
		newIgnoreCmd(g),
		newProtectCmd(g),
		newVersionCmd(g),
	)
	return root
}
