package sources

import (
	"cmp"
	"encoding/json"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

type Package struct {
	Name     string   `json:"name"`
	Bytes    uint64   `json:"bytes"` // every copy together
	Copies   int      `json:"copies"`
	Versions []string `json:"versions,omitempty"`
	Projects []string `json:"projects"` // biggest copy first
}

type Cached struct {
	Name     string   `json:"name"`
	Versions []string `json:"versions"`
	Bytes    uint64   `json:"bytes"`
}

type Packages struct {
	Projects    int       `json:"projects"`
	NodeModules uint64    `json:"nodeModules"`
	Packages    []Package `json:"packages"`
	Cache       []Cached  `json:"cache"`
	CacheBytes  uint64    `json:"cacheBytes"`
}

type copyOf struct {
	project string
	path    string
	version string
	bytes   uint64
}

const maxReads = 3000

// FindPackages groups the packages in the node_modules of every project
// below home, and the versions Bun's cache keeps of each. read gives
// package.json files (for versions) of the biggest packages only.
func FindPackages(t *tree.Tree, links volume.Firmlinks, home string, read func(string) ([]byte, error), limit int) Packages {
	var out Packages
	homeID, ok := t.Lookup(links.Real(home))
	if !ok {
		return out
	}
	// The walk holds the tree's read lock: find the folders first, read their children after.
	var found []tree.ID
	project := map[tree.ID]bool{}
	t.WalkDirs(homeID, func(n tree.Node) bool {
		project[n.ID] = n.Flags&tree.FlagProject != 0
		if n.ID != homeID && (strings.HasPrefix(n.Name, ".") || n.Parent == homeID && n.Name == "Library" || strings.HasSuffix(n.Name, ".app")) {
			return false
		}
		if n.Name == "node_modules" {
			if project[n.Parent] {
				found = append(found, n.ID)
			}
			return false
		}
		return true
	})
	byName := map[string][]copyOf{}
	for _, nm := range found {
		out.Projects++
		out.NodeModules += t.Reclaimable(nm)
		proj := links.Display(filepath.Dir(t.Path(nm)))
		for _, c := range t.Children(nm) {
			n := t.Node(c)
			switch {
			case n.Kind != tree.KindDir || n.Name == ".bin" || n.Name == ".cache":
			case n.Name == ".pnpm":
				for _, e := range t.Children(c) {
					if name, version, ok := pnpmEntry(t.Node(e).Name); ok {
						byName[name] = append(byName[name], copyOf{proj, t.Path(e), version, t.Reclaimable(e)})
					}
				}
			case strings.HasPrefix(n.Name, "@"):
				for _, s := range t.Children(c) {
					if sn := t.Node(s); sn.Kind == tree.KindDir {
						byName[n.Name+"/"+sn.Name] = append(byName[n.Name+"/"+sn.Name], copyOf{proj, t.Path(s), "", t.Reclaimable(s)})
					}
				}
			case !strings.HasPrefix(n.Name, "."):
				byName[n.Name] = append(byName[n.Name], copyOf{proj, t.Path(c), "", t.Reclaimable(c)})
			}
		}
	}
	for name, cs := range byName {
		p := Package{Name: name, Copies: len(cs)}
		for _, c := range cs {
			p.Bytes += c.bytes
		}
		out.Packages = append(out.Packages, p)
	}
	slices.SortFunc(out.Packages, func(a, b Package) int { return cmp.Or(cmp.Compare(b.Bytes, a.Bytes), cmp.Compare(a.Name, b.Name)) })
	out.Packages = out.Packages[:min(limit, len(out.Packages))]
	reads := 0
	for i := range out.Packages {
		p := &out.Packages[i]
		cs := byName[p.Name]
		slices.SortFunc(cs, func(a, b copyOf) int { return cmp.Compare(b.bytes, a.bytes) })
		seen := map[string]bool{}
		for _, c := range cs {
			if !seen[c.project] && len(p.Projects) < 20 {
				seen[c.project] = true
				p.Projects = append(p.Projects, c.project)
			}
			v := c.version
			if v == "" && reads < maxReads {
				reads++
				v = version(read, filepath.Join(c.path, "package.json"))
			}
			if v != "" && !slices.Contains(p.Versions, v) {
				p.Versions = append(p.Versions, v)
			}
		}
		slices.Sort(p.Versions)
	}
	out.Cache, out.CacheBytes = bunCache(t, links, home, limit)
	return out
}

var pnpmName = regexp.MustCompile(`^(@?[^@]+)@([^_@]+)`)

// pnpmEntry reads .pnpm/<name>@<version>[_peers]; scoped names use + for /.
func pnpmEntry(entry string) (name, version string, ok bool) {
	m := pnpmName.FindStringSubmatch(entry)
	if m == nil {
		return "", "", false
	}
	return strings.Replace(m[1], "+", "/", 1), m[2], true
}

func version(read func(string) ([]byte, error), path string) string {
	b, err := read(path)
	if err != nil {
		return ""
	}
	var pj struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(b, &pj) != nil || len(pj.Version) > 64 {
		return ""
	}
	return pj.Version
}

var bunEntry = regexp.MustCompile(`^(.+)@([^@]+)@@@\d+$`)

// bunCache lists what Bun keeps per package: <name>@<version>@@@<n>, scoped
// packages one folder down in their @scope.
func bunCache(t *tree.Tree, links volume.Firmlinks, home string, limit int) ([]Cached, uint64) {
	root, ok := t.Lookup(links.Real(filepath.Join(home, ".bun", "install", "cache")))
	if !ok {
		return nil, 0
	}
	by := map[string]*Cached{}
	var total uint64
	var visit func(dir tree.ID, scope string)
	visit = func(dir tree.ID, scope string) {
		for _, c := range t.Children(dir) {
			n := t.Node(c)
			if n.Kind != tree.KindDir {
				continue
			}
			if scope == "" && strings.HasPrefix(n.Name, "@") && !strings.Contains(n.Name[1:], "@") {
				visit(c, n.Name+"/")
				continue
			}
			m := bunEntry.FindStringSubmatch(n.Name)
			if m == nil || strings.HasPrefix(m[1], "@G") {
				continue
			}
			name := scope + m[1]
			e := by[name]
			if e == nil {
				e = &Cached{Name: name}
				by[name] = e
			}
			b := t.Reclaimable(c)
			e.Bytes += b
			total += b
			if !slices.Contains(e.Versions, m[2]) {
				e.Versions = append(e.Versions, m[2])
			}
		}
	}
	visit(root, "")
	var out []Cached
	for _, e := range by {
		slices.Sort(e.Versions)
		out = append(out, *e)
	}
	slices.SortFunc(out, func(a, b Cached) int { return cmp.Or(cmp.Compare(b.Bytes, a.Bytes), cmp.Compare(a.Name, b.Name)) })
	return out[:min(limit, len(out))], total
}
