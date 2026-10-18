package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/rules"
	"github.com/doguyilmaz/ebb/internal/units"
)

func newDupesCmd(g *globals) *cobra.Command {
	var minSize string
	var limit int
	cmd := &cobra.Command{
		Use:   "dupes [folder]",
		Short: "Files with the same content, in your own folders (default: home)",
		Long: "Reads files of the same size from the last scan to find identical ones. Bundles, libraries (Photos, Music),\n" +
			"~/Library, node_modules and dot-folders are left out. Remove extra copies with ebb clean --path.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			var size rules.Size
			if err := size.UnmarshalText([]byte(minSize)); err != nil {
				return fmt.Errorf("--min: %w", err)
			}
			path := ""
			if len(args) == 1 {
				abs, err := absPath(args[0])
				if err != nil {
					return err
				}
				path = a.Links.Display(abs)
			}
			res, err := a.FindDuplicates(cmd.Context(), path, uint64(size), nil)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if g.json {
				return emitJSON(out, res)
			}
			var rows [][]string
			for _, grp := range res.Groups[:min(limit, len(res.Groups))] {
				rows = append(rows, []string{units.Bytes(grp.Wasted), fmt.Sprintf("%d × %s", len(grp.Copies), units.Bytes(grp.Size)), filepath.Base(grp.Copies[0].Path)})
			}
			printTable(out, []string{"EXTRA", "COPIES", "NAME"}, rows)
			fmt.Fprintf(out, "%d groups, %s in extra copies (%d files read in %s).\n", len(res.Groups), units.Bytes(res.Wasted), res.Hashed, res.Elapsed.Round(1e7))
			for _, grp := range res.Groups[:min(3, len(res.Groups))] {
				fmt.Fprintf(out, "\n%s\n", filepath.Base(grp.Copies[0].Path))
				for i, c := range grp.Copies {
					mark := "  extra"
					if i == 0 {
						mark = "  keep "
					}
					fmt.Fprintf(out, "%s %s\n", mark, units.Printable(c.Path))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&minSize, "min", "1MiB", "smallest file to compare")
	cmd.Flags().IntVarP(&limit, "limit", "n", 30, "how many groups")
	return cmd
}
