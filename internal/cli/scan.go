package cli

import (
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/report"
	"github.com/doguyilmaz/ebb/internal/units"
)

func newScanCmd(g *globals) *cobra.Command {
	var opt app.ScanOptions
	var profile string
	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan the disk (or a folder) and find what takes space",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				opt.Root = args[0]
			}
			if profile != "" {
				f, err := os.Create(profile)
				if err != nil {
					return err
				}
				defer f.Close()
				if err := pprof.StartCPUProfile(f); err != nil {
					return err
				}
				defer pprof.StopCPUProfile()
			}
			res, err := a.Scan(cmd.Context(), opt, progress(os.Stderr))
			if err != nil {
				return err
			}
			rep := report.Build(a.View(res), a.Links, Version)
			if g.json {
				return emitJSON(cmd.OutOrStdout(), map[string]any{"meta": res.Meta, "volume": res.Volume, "findings": len(res.Findings),
					"reclaimable": rep.Reclaimable, "problems": res.ProblemCount})
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Scanned %s: %s files in %s folders, %s, in %s\n", units.Printable(res.Meta.Root), thousands(res.Meta.Files), thousands(res.Meta.Dirs),
				units.Bytes(res.Meta.Scanned), res.Meta.Elapsed.Round(1e8))
			fmt.Fprintf(out, "%d findings, %s reclaimable. Next: ebb findings · ebb report\n", len(res.Findings), units.Bytes(rep.Reclaimable))
			if res.ProblemCount > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %d folders could not be read (see ebb report)\n", res.ProblemCount)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.IntVar(&opt.Threads, "threads", 0, "parallel readers (default: min(8, CPUs))")
	f.BoolVar(&opt.Gentle, "gentle", false, "throttle disk I/O so other work stays fast")
	f.StringVar(&profile, "profile", "", "write a CPU profile to FILE")
	return cmd
}
