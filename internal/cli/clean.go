package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/clean"
	"github.com/doguyilmaz/ebb/internal/units"
)

func newCleanCmd(g *globals) *cobra.Command {
	var preset string
	var paths []string
	var dry, yes bool
	cmd := &cobra.Command{
		Use:   "clean [finding|category]...",
		Short: "Plan, review and carry out a cleanup",
		Long: "Picks findings by ID (see ebb findings) or category, a preset, or paths, shows exactly what will happen,\n" +
			"and asks before doing it. --yes skips the question only when everything can be rebuilt by its tool.",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := g.app()
			if err != nil {
				return err
			}
			req, err := cleanRequest(a, args, preset, paths)
			if err != nil {
				return err
			}
			p, err := a.Plan(req)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			runnable := printPlan(out, p)
			if dry {
				return nil
			}
			if runnable == 0 {
				return fmt.Errorf("nothing in this plan can be done (see the reasons above)")
			}
			if !(yes && !p.NeedsApproval) && !confirm(cmd.InOrStdin(), cmd.ErrOrStderr()) {
				return fmt.Errorf("this plan needs your confirmation: answer y in a terminal, or approve it in the UI")
			}
			if _, err := a.Approve(p.ID); err != nil {
				return err
			}
			res, err := a.Execute(cmd.Context(), p.ID, func(e clean.Event) {
				switch e.State {
				case "done":
					fmt.Fprintf(out, "  done     %s\n", e.Path)
				case "refused", "failed":
					fmt.Fprintf(out, "  %-8s %s: %s\n", e.State, e.Path, e.Err)
				}
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "Freed about %s: %d done, %d refused, %d failed.\n", units.Bytes(res.Freed), res.Done, res.Refused, res.Failed)
			if slices.ContainsFunc(p.Items, func(it clean.Item) bool { return it.Action.Kind == "trash" }) {
				fmt.Fprintln(out, "Items moved to the Trash free their space when the Trash is emptied; ebb undo puts the last one back.")
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&preset, "preset", "", "a preset: dev-caches, simulators, stale-projects, logs, installers, all-regenerable")
	f.StringArrayVar(&paths, "path", nil, "a folder or file to remove (to the Trash unless a finding says otherwise)")
	f.BoolVar(&dry, "dry-run", false, "only show the plan")
	f.BoolVar(&yes, "yes", false, "do not ask when every item can be rebuilt by its tool")
	return cmd
}

func cleanRequest(a *app.App, args []string, preset string, paths []string) (clean.Request, error) {
	req := clean.Request{Preset: preset, Source: "cli"}
	last, err := a.Last()
	if err != nil {
		return req, err
	}
	findings := a.View(last).Findings
	for _, arg := range args {
		found := false
		for _, f := range findings {
			if f.ID == arg || f.Category == arg {
				req.FindingIDs = append(req.FindingIDs, f.ID)
				found = true
			}
		}
		if !found {
			return req, fmt.Errorf("unknown finding or category %q (see ebb findings)", arg)
		}
	}
	for _, p := range paths {
		abs, err := absPath(p)
		if err != nil {
			return req, err
		}
		req.Paths = append(req.Paths, a.Links.Display(abs))
	}
	return req, nil
}

func printPlan(w io.Writer, p *clean.Plan) int {
	var rows [][]string
	runnable := 0
	for _, it := range p.Items {
		state := "ok"
		switch {
		case it.Blocked != "":
			state = "blocked"
		case it.Action.Kind == "manual":
			state = "manual"
		default:
			runnable++
		}
		what := it.Action.Kind
		if it.Action.Kind == "command" {
			what = strings.Join(it.Command, " ")
		}
		rows = append(rows, []string{state, what, units.Bytes(it.Bytes), string(it.Safety), it.Path})
	}
	printTable(w, []string{"STATE", "ACTION", "SIZE", "SAFETY", "PATH"}, rows)
	for _, it := range p.Items {
		for _, warn := range it.Warnings {
			fmt.Fprintf(w, "note: %s: %s\n", it.Path, warn)
		}
		if it.Blocked != "" {
			fmt.Fprintf(w, "blocked: %s: %s\n", it.Path, it.Blocked)
		}
		if it.Action.Kind == "manual" && it.Action.Hint != "" {
			fmt.Fprintf(w, "manual: %s: %s\n", it.Path, it.Action.Hint)
		}
	}
	fmt.Fprintf(w, "Plan %s: %d to do, about %s.\n", p.ID, runnable, units.Bytes(p.Bytes))
	return runnable
}

// confirm asks on stderr and reads one answer; no answer means no.
func confirm(in io.Reader, errOut io.Writer) bool {
	fmt.Fprint(errOut, "Proceed? [y/N] ")
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(errOut)
		return false
	}
	a := strings.ToLower(strings.TrimSpace(line))
	return a == "y" || a == "yes"
}

func absPath(p string) (string, error) {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if h := os.Getenv("EBB_HOME"); h != "" {
			home, err = h, nil
		}
		if err != nil {
			return "", err
		}
		p = home + p[1:]
	}
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		p = wd + "/" + p
	}
	return cleanPath(p), nil
}
