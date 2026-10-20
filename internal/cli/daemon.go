package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/appkit"
	"github.com/doguyilmaz/ebb/internal/daemon"
	"github.com/doguyilmaz/ebb/internal/svc"
	"github.com/doguyilmaz/ebb/internal/sys"
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
			opt := daemon.Options{Version: Version, Out: cmd.OutOrStdout()}
			if !appkit.Bundled() {
				return daemon.Run(ctx, a, opt)
			}
			// From ebb.app the main thread belongs to AppKit, which never returns: the service runs beside it.
			go func() {
				code := 0
				if err := daemon.Run(ctx, a, opt); err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), "error:", err)
					code = 1
				}
				os.Exit(code)
			}()
			appkit.Run(func() {
				if err := daemon.OpenWindow(context.Background(), a); err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), "open the window:", err)
				}
			})
			return nil
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

// healService sets the background service up again when the person had it on
// and an upgrade or a move took it away. Homebrew runs post-install steps in a
// sandbox that cannot reach launchd, so ebb does it the next time it runs.
// open and serve start ebb themselves and the daemon commands manage the service.
func healService(cmd *cobra.Command, verbose bool) {
	for c := cmd; c != nil; c = c.Parent() {
		if n := c.Name(); n == "daemon" || n == "open" || n == "serve" {
			return
		}
	}
	env := sys.Real()
	if sys.Sandboxed(env) {
		return
	}
	if _, err := svc.Local(env).Heal(cmd.Context()); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "ebb's background service stopped and could not start again. Turn it back on with: ebb daemon install")
		if verbose {
			fmt.Fprintln(cmd.ErrOrStderr(), err)
		}
	}
}
