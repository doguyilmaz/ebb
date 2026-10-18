package cli

import (
	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/mcpserver"
	"github.com/doguyilmaz/ebb/internal/remote"
)

func newMCPCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Serve ebb to AI agents over the Model Context Protocol (stdio)",
		Long: "Speaks MCP on stdin/stdout. When ebb serve or the background service runs, agents share its scan and plans " +
			"with the window; otherwise the engine runs in this process.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			b := mcpserver.Auto(mcpserver.Local(a, Version), func() (*remote.Client, error) { return remote.Discover(a.Env) })
			return mcpserver.New(b, Version).Serve(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}
