package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the ebb version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.json {
				return emitJSON(cmd.OutOrStdout(), map[string]string{"version": Version, "os": runtime.GOOS, "arch": runtime.GOARCH})
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "ebb %s %s/%s\n", Version, runtime.GOOS, runtime.GOARCH)
			return err
		},
	}
}
