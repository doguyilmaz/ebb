package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/report"
)

func newReportCmd(g *globals) *cobra.Command {
	var format, out string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Full report of the last scan as text, JSON, CSV or HTML",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			last, err := a.Last()
			if err != nil {
				return err
			}
			rep := report.Build(last, a.Links, Version)
			var w io.Writer = cmd.OutOrStdout()
			if out != "" {
				f, err := os.Create(out)
				if err != nil {
					return err
				}
				defer f.Close()
				w = f
			}
			switch format {
			case "text":
				return report.WriteText(w, rep, 100)
			case "json":
				return report.WriteJSON(w, rep)
			case "csv":
				return report.WriteCSV(w, rep)
			case "html":
				return report.WriteHTML(w, rep, last.Tree)
			}
			return fmt.Errorf("unknown format %q (text, json, csv, html)", format)
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "text, json, csv or html")
	cmd.Flags().StringVarP(&out, "output", "o", "", "write to FILE instead of stdout")
	return cmd
}
