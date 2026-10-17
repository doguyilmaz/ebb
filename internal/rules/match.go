package rules

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/doguyilmaz/ebb/internal/tree"
)

// Resolve returns the tree nodes whose absolute path matches pattern. Glob
// segments (doublestar syntax: *, ?, [..], {a,b}, **) are matched against
// children one level at a time, so only matching branches are visited.
func Resolve(t *tree.Tree, pattern string) []tree.ID {
	segs := strings.Split(strings.Trim(pattern, "/"), "/")
	i := 0
	for i < len(segs) && !hasMeta(segs[i]) {
		i++
	}
	start, ok := t.Lookup("/" + strings.Join(segs[:i], "/"))
	if !ok {
		return nil
	}
	return matchSegs(t, start, segs[i:])
}

func hasMeta(s string) bool { return strings.ContainsAny(s, "*?[{") }

func matchSegs(t *tree.Tree, id tree.ID, segs []string) []tree.ID {
	if len(segs) == 0 {
		return []tree.ID{id}
	}
	var out []tree.ID
	seg := segs[0]
	if seg == "**" {
		out = append(out, matchSegs(t, id, segs[1:])...)
		for _, c := range t.Children(id) {
			if t.Node(c).Kind == tree.KindDir {
				out = append(out, matchSegs(t, c, segs)...)
			}
		}
		return out
	}
	for _, c := range t.Children(id) {
		n := t.Node(c)
		if ok, _ := doublestar.Match(seg, n.Name); !ok {
			continue
		}
		if len(segs) == 1 {
			out = append(out, c)
		} else if n.Kind == tree.KindDir {
			out = append(out, matchSegs(t, c, segs[1:])...)
		}
	}
	return out
}
