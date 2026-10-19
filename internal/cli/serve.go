package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/server"
	"github.com/doguyilmaz/ebb/internal/window"
)

func newServeCmd(g *globals) *cobra.Command {
	var port int
	var open bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the local web UI and API on 127.0.0.1",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			token, err := server.LoadOrCreateToken(a.Env)
			if err != nil {
				return err
			}
			if st, ok := server.Live(a.Env); ok {
				url := st.URL() + "/#token=" + token
				fmt.Fprintf(cmd.OutOrStdout(), "ebb is already running: %s\n", url)
				if open {
					return window.Open(a.Env, url)
				}
				return nil
			}
			if port == 0 {
				port = a.Config.Server.Port
			}
			return serveHere(cmd, a, token, port, func(url string) {
				fmt.Fprintf(cmd.OutOrStdout(), "ebb is running: %s\nPress Ctrl-C or close this terminal to stop it. To keep it running in the background: ebb daemon install\n", url)
				if open {
					_ = window.Open(a.Env, url)
				}
			})
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "port on 127.0.0.1 (default from config, 47340)")
	cmd.Flags().BoolVar(&open, "open", false, "open the UI in the browser")
	return cmd
}

// serveHere runs the window and the API in this process until it is stopped.
func serveHere(cmd *cobra.Command, a *app.App, token string, port int, ready func(url string)) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	s := server.New(a, Version, token)
	defer server.RemoveStatus(a.Env)
	go a.Watch(ctx)
	return server.Serve(ctx, s, fmt.Sprintf("127.0.0.1:%d", port), func(url string) {
		_ = server.WriteStatus(a.Env, server.Status{PID: os.Getpid(), Port: port, Version: Version, Started: a.Env.Now()})
		ready(url)
	})
}
