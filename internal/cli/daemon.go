package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/daemon"
	"github.com/doguyilmaz/ebb/internal/svc"
)

func newDaemonCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Keep ebb running in the background: the window, fresh scans and low space warnings",
		Long: "The background service starts at login and serves ebb's window and API. While the Mac is on power and " +
			"nobody uses it, it updates the last scan (at most every half hour, a full scan weekly), and it warns once " +
			"when free space drops below the limit in settings.",
	}
	act := func(use, short, done string, fn func(svc.Service, context.Context) (string, error)) *cobra.Command {
		return &cobra.Command{
			Use:   use,
			Short: short,
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				a, err := g.app()
				if err != nil {
					return err
				}
				note, err := fn(svc.Local(a.Env), cmd.Context())
				if err != nil {
					return err
				}
				if g.verbose && note != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), note)
				}
				fmt.Fprintln(cmd.OutOrStdout(), done)
				return nil
			},
		}
	}
	status := &cobra.Command{
		Use:   "status",
		Short: "Whether the service is set up and running, and what it did last",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			st := svc.Local(a.Env).Read()
			out := cmd.OutOrStdout()
			if g.json {
				return emitJSON(out, st)
			}
			now := a.Env.Now()
			state := "not running"
			switch {
			case !st.Supported:
				state = "only on macOS"
			case st.Waiting:
				state = "waiting for the ebb serving the window to close"
			case st.Running:
				state = fmt.Sprintf("running (pid %d, since %s)", st.PID, ago(st.Started, now))
			}
			installed := "no (ebb daemon install)"
			if st.Installed {
				installed = "yes, starts at login"
			}
			work := "-"
			if st.LastWork != "" {
				work = st.LastWork + " scan, " + ago(st.LastWorkAt, now)
			}
			printTable(out, []string{"", ""}, [][]string{
				{"Installed", installed}, {"State", state}, {"Last check", ago(st.LastCheck, now)},
				{"Last scan", work}, {"Last warning", ago(st.LastAlert, now)}, {"Log", st.Log},
			})
			if st.Error != "" {
				fmt.Fprintln(out, "Problem:", st.Error)
			}
			return nil
		},
	}
	run := &cobra.Command{
		Use:    "run",
		Short:  "Run the service in this terminal (launchd runs this)",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return daemon.Run(ctx, a, daemon.Options{Version: Version, Out: cmd.OutOrStdout()})
		},
	}
	cmd.AddCommand(
		act("install", "Start ebb at login and keep it running", "ebb now runs in the background. Open it with: ebb open", svc.Service.Install),
		act("restart", "Restart the service, for example after an upgrade", "Restarted.", svc.Service.Restart),
		act("uninstall", "Stop the service and remove it from login", "The background service is gone.", svc.Service.Uninstall),
		status, run,
	)
	return cmd
}
