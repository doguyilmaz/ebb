package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/units"
)

type item struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Alloc uint64 `json:"alloc"`
	Bytes uint64 `json:"bytes"`
	Files uint32 `json:"files"`
}

func kindName(k tree.Kind) string {
	return [...]string{"dir", "file", "link", "other", "small"}[k]
}

func newTopCmd(g *globals) *cobra.Command {
	var n int
	var files bool
	cmd := &cobra.Command{
		Use:   "top [path]",
		Short: "Largest folders (or files) under a path of the last scan",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if n < 0 {
				return errors.New("-n must be 0 or more")
			}
			a, err := g.app()
			if err != nil {
				return err
			}
			last, err := a.Last()
			if err != nil {
				return err
			}
			t := last.Tree
			id, err := resolve(a, t, strings.Join(args, ""))
			if err != nil {
				return err
			}
			var ids []tree.ID
			if files {
				ids = t.Top(id, n)
			} else {
				ids = t.Children(id)
				ids = ids[:min(n, len(ids))]
			}
			total := t.Node(id).Alloc
			var items []item
			var rows [][]string
			for _, c := range ids {
				node := t.Node(c)
				it := item{Path: a.Links.Display(t.Path(c)), Name: node.Name, Kind: kindName(node.Kind), Alloc: node.Alloc, Bytes: t.Reclaimable(c), Files: node.Files}
				items = append(items, it)
				share := 0.0
				if total > 0 {
					share = float64(node.Alloc) / float64(total) * 100
				}
				rows = append(rows, []string{units.Bytes(it.Alloc), fmt.Sprintf("%.1f%%", share), thousands(uint64(it.Files)), it.Path})
			}
			if g.json {
				return emitJSON(cmd.OutOrStdout(), items)
			}
			printTable(cmd.OutOrStdout(), []string{"SIZE", "SHARE", "FILES", "PATH"}, rows)
			return nil
		},
	}
	cmd.Flags().IntVarP(&n, "number", "n", 20, "how many to show")
	cmd.Flags().BoolVar(&files, "files", false, "largest files instead of folders")
	return cmd
}

func newFindCmd(g *globals) *cobra.Command {
	var n int
	cmd := &cobra.Command{
		Use:   "find <pattern>",
		Short: "Find folders and large files by name (glob or substring)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if n < 0 {
				return errors.New("-n must be 0 or more")
			}
			a, err := g.app()
			if err != nil {
				return err
			}
			last, err := a.Last()
			if err != nil {
				return err
			}
			t := last.Tree
			ids := t.Find(args[0], 100000)
			slices.SortFunc(ids, func(x, y tree.ID) int { return int(int64(t.Node(y).Alloc>>10) - int64(t.Node(x).Alloc>>10)) })
			ids = ids[:min(n, len(ids))]
			var items []item
			var rows [][]string
			for _, id := range ids {
				node := t.Node(id)
				it := item{Path: a.Links.Display(t.Path(id)), Name: node.Name, Kind: kindName(node.Kind), Alloc: node.Alloc, Bytes: t.Reclaimable(id), Files: node.Files}
				items = append(items, it)
				rows = append(rows, []string{units.Bytes(it.Alloc), it.Kind, it.Path})
			}
			if g.json {
				return emitJSON(cmd.OutOrStdout(), items)
			}
			printTable(cmd.OutOrStdout(), []string{"SIZE", "KIND", "PATH"}, rows)
			return nil
		},
	}
	cmd.Flags().IntVarP(&n, "number", "n", 50, "how many to show")
	return cmd
}
