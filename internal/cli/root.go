// Package cli holds the thin Cobra layer. Commands parse flags, call into the
// core packages and print; they hold no logic of their own.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/appkit"
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
	if launchedAsApp(args) {
		args = []string{"open", "--yes"}
	}
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
				if err := os.Setenv(sys.HomeEnv, g.home); err != nil {
					return err
				}
			}
			healService(cmd, g.verbose)
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error { return runOverview(g, cmd) },
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	pf := root.PersistentFlags()
	pf.BoolVar(&g.json, "json", false, "machine-readable JSON output")
	pf.StringVar(&g.home, "home", "", "treat DIR as the home directory (same as EBB_HOME)")
	pf.BoolVar(&g.verbose, "verbose", false, "verbose diagnostics on stderr")
	root.Version = Version
	root.SetVersionTemplate(versionLine() + "\n")

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
		newOpenCmd(g),
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

// launchedAsApp: Finder, Spotlight or a notification click start ebb.app with
// no arguments (older macOS adds -psn_…) and no terminal, which means "open ebb".
func launchedAsApp(args []string) bool {
	noArgs := len(args) == 0 || (len(args) == 1 && strings.HasPrefix(args[0], "-psn_"))
	return noArgs && os.Getppid() == 1 && !interactive() && appkit.Bundled()
}
