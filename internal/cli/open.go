package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/server"
	"github.com/doguyilmaz/ebb/internal/svc"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/window"
)

const startTimeout = 15 * time.Second

var errNotStarted = errors.New("the background service did not start; see: ebb daemon status")

func newOpenCmd(g *globals) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "open [origin]",
		Short: "Open ebb's window, starting ebb first when it is not running",
		Long: "Opens the hosted page connected to ebb on this Mac through a one-time link. ebb's own window opens instead " +
			"when no web page is allowed or the default browser is Safari. When ebb is not running it starts the background " +
			"service, offering to set it up the first time. Give an origin to open your own copy of the page.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			asked := ""
			if len(args) == 1 {
				asked = args[0]
			}
			origin, err := window.Target(cmd.Context(), a.Env, a.Config.Server.WebOrigins, asked)
			if err != nil {
				return err
			}
			token, err := server.LoadOrCreateToken(a.Env)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			show := func(port int) error {
				link, err := window.Link(a, origin, port, token)
				if err != nil {
					return err
				}
				if origin == "" {
					fmt.Fprintf(out, "Opening ebb's window: %s\n", link)
				} else {
					fmt.Fprintf(out, "Opening ebb in your browser.\nAnother browser? Open this link within %d minutes: %s\n", int(app.LinkWindow.Minutes()), link)
				}
				return window.Open(a.Env, link)
			}

			s := svc.Local(a.Env)
			if st, ok := server.Live(a.Env); ok {
				return show(refreshService(cmd, a.Env, s, st).Port)
			}
			if s.Installed() || s.Wanted() {
				st, err := startService(cmd.Context(), a.Env, s)
				if err != nil {
					return err
				}
				return show(st.Port)
			}
			if !yes && !interactive() {
				return errors.New("ebb is not running. Keep it running in the background with: ebb daemon install. Run it in this terminal with: ebb serve")
			}
			background, answered := true, true
			if !yes {
				background, answered = ask(cmd, "ebb is not running. Keep it running in the background and start it at login? [Y/n] ")
			}
			switch {
			case !answered:
				return errors.New("nothing started")
			case background:
				st, err := startService(cmd.Context(), a.Env, s)
				if err != nil {
					return err
				}
				fmt.Fprintln(out, "ebb now runs in the background and starts at login. Turn it off with: ebb daemon uninstall")
				return show(st.Port)
			default:
				port := a.Config.Server.Port
				return serveHere(cmd, a, token, port, func(string) {
					fmt.Fprintln(out, "ebb is running in this terminal. Press Ctrl-C or close the terminal to stop it.")
					if err := show(port); err != nil {
						fmt.Fprintln(cmd.ErrOrStderr(), "error:", err)
					}
				})
			}
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "keep ebb running in the background without asking")
	return cmd
}

// refreshService restarts a background ebb older than this one, or one an
// older definition started, so an upgrade takes over without a separate step.
func refreshService(cmd *cobra.Command, env sys.Env, s svc.Service, st server.Status) server.Status {
	older := Version != "dev" && st.Version != Version
	if !st.Daemon || sys.Sandboxed(env) || (!older && s.Current()) {
		return st
	}
	if _, err := s.Restart(cmd.Context()); err != nil {
		return st
	}
	if nst, ok := waitLive(env, func(n server.Status) bool { return n.PID != st.PID }); ok {
		fmt.Fprintln(cmd.OutOrStdout(), "Restarted the background ebb so this version serves the window.")
		return nst
	}
	return st
}

func startService(ctx context.Context, env sys.Env, s svc.Service) (server.Status, error) {
	// kickstart fails when the service is not loaded, for example after launchctl bootout.
	if _, err := s.Restart(ctx); err != nil {
		if _, err := s.Install(ctx); err != nil {
			return server.Status{}, err
		}
	}
	st, ok := waitLive(env, func(server.Status) bool { return true })
	if !ok {
		return st, errNotStarted
	}
	return st, nil
}

func waitLive(env sys.Env, done func(server.Status) bool) (server.Status, bool) {
	for end := time.Now().Add(startTimeout); time.Now().Before(end); time.Sleep(200 * time.Millisecond) {
		if st, ok := server.Live(env); ok && done(st) {
			return st, true
		}
	}
	return server.Status{}, false
}

// ask is a yes/no question where Enter means yes; Ctrl-D answers nothing.
func ask(cmd *cobra.Command, question string) (yes, answered bool) {
	fmt.Fprint(cmd.OutOrStdout(), question)
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(cmd.OutOrStdout())
		return false, false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "" || answer == "y" || answer == "yes", true
}
