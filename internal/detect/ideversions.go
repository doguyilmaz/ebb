package detect

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/tree"
)

// ideVersions finds what JetBrains IDEs and Android Studio keep for versions
// you already upgraded from: each version has its own settings, caches and
// logs folder, and the newer one copies the settings at its first launch.
type ideVersions struct{}

func (ideVersions) ID() string { return "ide-versions" }
func (ideVersions) Pass() int  { return 1 }

var ideFolder = regexp.MustCompile(`^([A-Za-z]+)(\d{4}\.\d+)$`)

// idePlaces are vendor folders; the settings one (first) says which versions are in use.
var idePlaces = [][]string{
	{"Library/Application Support/JetBrains", "Library/Caches/JetBrains", "Library/Logs/JetBrains"},
	{"Library/Application Support/Google", "Library/Caches/Google", "Library/Logs/Google"},
}

func (ideVersions) Detect(_ context.Context, c *Context) ([]finding.Finding, error) {
	type version struct {
		v   string
		ids []tree.ID
	}
	f := finding.Finding{RuleID: "ide.old-versions", Title: "Folders of IDE versions you upgraded from", Category: "app-cache", Safety: finding.Caution,
		Why: "JetBrains IDEs and Android Studio keep settings, caches and logs per version. The newer version copied the settings " +
			"when it first started, and the old folders stay behind.",
		Impact:  "Going back to an old version would start it with default settings. The version you use now is not touched.",
		Actions: []finding.Action{{Kind: "trash"}}}
	for _, places := range idePlaces {
		products := map[string]map[string]*version{}
		inUse := map[string]bool{}
		for i, rel := range places {
			id, ok := c.node(c.home(rel))
			if !ok {
				continue
			}
			for _, ch := range c.Tree.Children(id) {
				n := c.Tree.Node(ch)
				m := ideFolder.FindStringSubmatch(n.Name)
				if n.Kind != tree.KindDir || m == nil {
					continue
				}
				if products[m[1]] == nil {
					products[m[1]] = map[string]*version{}
				}
				v := products[m[1]][m[2]]
				if v == nil {
					v = &version{v: m[2]}
					products[m[1]][m[2]] = v
				}
				v.ids = append(v.ids, ch)
				if i == 0 {
					inUse[m[1]+m[2]] = true
				}
			}
		}
		names := make([]string, 0, len(products))
		for p := range products {
			names = append(names, p)
		}
		sort.Strings(names)
		for _, p := range names {
			var vs []*version
			for _, v := range products[p] {
				vs = append(vs, v)
			}
			sort.Slice(vs, func(i, j int) bool { return compareVersions(vs[i].v, vs[j].v) < 0 })
			newest := vs[len(vs)-1]
			// Only when the newest version has settings of its own: it is the one in use.
			if !inUse[p+newest.v] {
				continue
			}
			for _, v := range vs[:len(vs)-1] {
				// One that still changes is in use too: a release beside an EAP, or a rollback.
				if slices.ContainsFunc(v.ids, func(id tree.ID) bool {
					return c.Now.Sub(time.Unix(c.Tree.Node(id).MaxMtime, 0)) < c.StaleAfter
				}) {
					continue
				}
				for _, id := range v.ids {
					f.Targets = append(f.Targets, c.target(id))
				}
				f.Evidence = append(f.Evidence, fmt.Sprintf("%s %s (now %s)", p, v.v, newest.v))
			}
		}
	}
	if len(f.Targets) == 0 {
		return nil, nil
	}
	return []finding.Finding{finish(f)}, nil
}
