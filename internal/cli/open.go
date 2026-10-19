package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/server"
	"github.com/doguyilmaz/ebb/internal/svc"
	"github.com/doguyilmaz/ebb/internal/sys"
)

const startTimeout = 15 * time.Second

var errNotStarted = errors.New("the background service did not start; see: ebb daemon status")

func newOpenCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "open [origin]",
		Short: "Open ebb's window, starting ebb first when it is not running",
		Long: "Opens the hosted page connected to ebb on this Mac through a one-time link, or ebb's own window when no " +
			"web page is allowed or the default browser is Safari. When ebb is not running it starts the background " +
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
			origin, err := windowOrigin(a.Config.Server.WebOrigins, asked, safari(defaultBrowser(cmd.Context(), a.Env)))
			if err != nil {
				return err
			}
			token, err := server.LoadOrCreateToken(a.Env)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			show := func(port int) error {
				link, err := windowLink(a, origin, port, token)
				if err != nil {
					return err
				}
				if origin == "" {
					fmt.Fprintf(out, "Opening ebb's window: %s\n", link)
				} else {
					fmt.Fprintf(out, "Opening ebb in your browser.\nAnother browser? Open this link within %d minutes: %s\n", int(app.LinkWindow.Minutes()), link)
				}
				return openURL(a.Env, link)
			}

			s := svc.Local(a.Env)
			if st, ok := server.Live(a.Env); ok {
				return show(refreshService(cmd, a.Env, s, st).Port)
			}
			if s.Installed() {
				st, err := startService(cmd.Context(), a.Env, s)
				if err != nil {
					return err
				}
				return show(st.Port)
			}
			if !interactive() {
				return errors.New("ebb is not running. Keep it running in the background with: ebb daemon install, or run it in this terminal with: ebb serve")
			}
			background, answered := ask(cmd, "ebb is not running. Keep it running in the background and start it at login? [Y/n] ")
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
}

// windowOrigin is the page ebb open sends the browser to; "" means ebb's own
// window. Safari keeps https pages away from loopback, so it gets the window.
func windowOrigin(allowed []string, asked string, safari bool) (string, error) {
	if asked != "" {
		o, ok := app.NormalizeOrigin(asked)
		if !ok || !slices.Contains(allowed, o) {
			return "", fmt.Errorf("this machine has not allowed %s; run: ebb web allow %s", asked, asked)
		}
		return o, nil
	}
	if safari || len(allowed) == 0 {
		return "", nil
	}
	return allowed[0], nil
}

// windowLink carries the secret in the fragment, which never reaches a server.
func windowLink(a *app.App, origin string, port int, token string) (string, error) {
	if origin == "" {
		return fmt.Sprintf("http://127.0.0.1:%d/#token=%s", port, token), nil
	}
	key, err := a.IssueLinkKey(origin)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/#key=%s&port=%d", origin, key, port), nil
}

// defaultBrowser is the bundle id of the app that opens https links, read from
// LaunchServices; "" when it cannot tell.
func defaultBrowser(ctx context.Context, env sys.Env) string {
	if env.OS() != "darwin" {
		return ""
	}
	plist := filepath.Join(env.Home(), "Library", "Preferences", "com.apple.LaunchServices", "com.apple.launchservices.secure.plist")
	raw, err := env.Run(ctx, "", "plutil", "-convert", "json", "-o", "-", plist)
	if err != nil {
		return ""
	}
	return httpsHandler(raw)
}

func httpsHandler(raw []byte) string {
	var ls struct {
		LSHandlers []struct {
			Scheme string `json:"LSHandlerURLScheme"`
			App    string `json:"LSHandlerRoleAll"`
		}
	}
	if json.Unmarshal(raw, &ls) != nil {
		return ""
	}
	for _, h := range ls.LSHandlers {
		if h.Scheme == "https" {
			return h.App
		}
	}
	// Nobody picked another browser.
	return "com.apple.Safari"
}

// safari counts an unknown browser too: ebb's own window works in every browser.
func safari(bundleID string) bool {
	return bundleID == "" || strings.HasPrefix(strings.ToLower(bundleID), "com.apple.safari")
}

// refreshService restarts a background ebb older than this one, so an upgrade
// takes over without a separate step.
func refreshService(cmd *cobra.Command, env sys.Env, s svc.Service, st server.Status) server.Status {
	if !st.Daemon || Version == "dev" || st.Version == Version || sys.Sandboxed(env) {
		return st
	}
	if _, err := s.Restart(cmd.Context()); err != nil {
		return st
	}
	if nst, ok := waitLive(env, func(n server.Status) bool { return n.PID != st.PID }); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "Restarted the background ebb (%s) so %s serves the window.\n", st.Version, Version)
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
