package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/assistant"
	"github.com/doguyilmaz/ebb/internal/sys"
)

func newAssistantCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{Use: "assistant", Short: "Set ebb up for AI tools (skill + MCP server)"}
	var dryRun bool
	install := &cobra.Command{
		Use:   "install",
		Short: "Give AI tools the ebb skill and register `ebb mcp` (through lockstep when it is installed)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			steps, err := assistant.Install(cmd.Context(), a.Env, sys.Executable(a.Env), dryRun)
			for _, s := range steps {
				fmt.Fprintln(cmd.OutOrStdout(), s)
			}
			return err
		},
	}
	install.Flags().BoolVar(&dryRun, "dry-run", false, "only print what would be done")
	skill := &cobra.Command{
		Use:   "skill",
		Short: "Print the ebb skill (SKILL.md)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := cmd.OutOrStdout().Write(assistant.Skill)
			return err
		},
	}
	cmd.AddCommand(install, skill)
	return cmd
}
