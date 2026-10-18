// Package sources explains where the scanned space comes from: which places
// on disk hold it, which tool or app owns it and what kind it is, and which
// JavaScript packages fill the node_modules folders of every project.
package sources

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

// Node is a place (column 0), an owner (1) or a kind (2).
type Node struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Column int    `json:"column"`
	Bytes  uint64 `json:"bytes"`
	Path   string `json:"path,omitempty"` // places: the folder to explore
}

type Link struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Bytes uint64 `json:"bytes"`
}

type Flows struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
	Total uint64 `json:"total"`
}

// Kinds outside the findings, for what no finding claims.
const (
	KindYours = "yours"     // the person's files
	KindApps  = "apps"      // applications
	KindOther = "unclaimed" // everything else
)

var personal = []string{"Desktop", "Documents", "Downloads", "Movies", "Music", "Pictures", "Public"}

const (
	maxPlaces = 12
	maxOwners = 12
)

type place struct {
	id    tree.ID
	path  string // real
	bytes uint64
}

// Build splits the scan into flows place → owner → kind. Places are the
// biggest folders of the home folder and of the scanned root (other than
// Users); findings say who owns what lies in them, the rest is the person's
// files, apps or unclaimed.
func Build(t *tree.Tree, links volume.Firmlinks, home string, findings []finding.Finding) Flows {
	places := candidates(t, links, home)
	type key struct{ a, b string }
	flow := map[key]uint64{}
	labels := map[string]Node{}
	node := func(id, label string, col int, path string) {
		if _, ok := labels[id]; !ok {
			labels[id] = Node{ID: id, Label: label, Column: col, Path: path}
		}
	}
	add := func(p place, owner, kind string, bytes uint64) {
		if bytes == 0 {
			return
		}
		pid, oid, kid := "p:"+p.path, "o:"+owner, "k:"+kind
		flow[key{pid, oid}] += bytes
		flow[key{oid, kid}] += bytes
	}
	homeReal := links.Real(home)
	rootPath := t.RootPath()
	for _, p := range places {
		node("p:"+p.path, label(links.Display(p.path), home, rootPath), 0, links.Display(p.path))
		claimed := uint64(0)
		for _, f := range findings {
			if len(f.Actions) == 0 && f.Safety != finding.UserData {
				continue
			}
			owner := ownerOf(f)
			for _, tg := range f.Targets {
				if real := links.Real(tg.Path); within(real, p.path) {
					add(p, owner, f.Category, tg.Bytes)
					claimed += tg.Bytes
					node("o:"+owner, owner, 1, "")
					node("k:"+f.Category, f.Category, 2, "")
				}
			}
		}
		rest := p.bytes - min(claimed, p.bytes)
		switch base := filepath.Base(p.path); {
		case filepath.Dir(p.path) == homeReal && slices.Contains(personal, base):
			add(p, "You", KindYours, rest)
			node("o:You", "You", 1, "")
			node("k:"+KindYours, KindYours, 2, "")
		case base == "Applications":
			add(p, "Apps", KindApps, rest)
			node("o:Apps", "Apps", 1, "")
			node("k:"+KindApps, KindApps, 2, "")
		default:
			add(p, "Not claimed", KindOther, rest)
			node("o:Not claimed", "Not claimed", 1, "")
			node("k:"+KindOther, KindOther, 2, "")
		}
	}
	out := Flows{}
	for k, b := range flow {
		out.Links = append(out.Links, Link{From: k.a, To: k.b, Bytes: b})
	}
	out.Links = foldOwners(out.Links, labels)
	sums := map[string]uint64{}
	for _, l := range out.Links {
		sums[l.From] += l.Bytes
		if strings.HasPrefix(l.To, "k:") {
			sums[l.To] += l.Bytes
		}
	}
	for id, n := range labels {
		if n.Bytes = sums[id]; n.Bytes > 0 {
			out.Nodes = append(out.Nodes, n)
		}
		if n.Column == 0 {
			out.Total += n.Bytes
		}
	}
	slices.SortFunc(out.Nodes, func(a, b Node) int {
		return cmp.Or(cmp.Compare(a.Column, b.Column), cmp.Compare(b.Bytes, a.Bytes), cmp.Compare(a.ID, b.ID))
	})
	slices.SortFunc(out.Links, func(a, b Link) int {
		return cmp.Or(cmp.Compare(b.Bytes, a.Bytes), cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})
	return out
}

// foldOwners keeps the biggest owners and joins the rest as "Other tools".
func foldOwners(links []Link, labels map[string]Node) []Link {
	size := map[string]uint64{}
	for _, l := range links {
		if strings.HasPrefix(l.To, "o:") {
			size[l.To] += l.Bytes
		}
	}
	owners := make([]string, 0, len(size))
	for o := range size {
		owners = append(owners, o)
	}
	slices.SortFunc(owners, func(a, b string) int { return cmp.Or(cmp.Compare(size[b], size[a]), cmp.Compare(a, b)) })
	if len(owners) <= maxOwners {
		return links
	}
	keep := map[string]bool{"o:You": true, "o:Apps": true, "o:Not claimed": true}
	for _, o := range owners[:maxOwners] {
		keep[o] = true
	}
	labels["o:Other tools"] = Node{ID: "o:Other tools", Label: "Other tools", Column: 1}
	merged := map[[2]string]uint64{}
	for _, l := range links {
		if !keep[l.To] && strings.HasPrefix(l.To, "o:") {
			l.To = "o:Other tools"
		}
		if !keep[l.From] && strings.HasPrefix(l.From, "o:") {
			l.From = "o:Other tools"
		}
		merged[[2]string{l.From, l.To}] += l.Bytes
	}
	out := links[:0]
	for k, b := range merged {
		out = append(out, Link{From: k[0], To: k[1], Bytes: b})
	}
	return out
}

// candidates are the biggest folders of home and of the root (without
// Users, which home stands for); the rest is one "Elsewhere".
func candidates(t *tree.Tree, links volume.Firmlinks, home string) []place {
	var ps []place
	homeID, hasHome := t.Lookup(links.Real(home))
	if hasHome {
		for _, c := range t.Children(homeID) {
			if n := t.Node(c); n.Kind == tree.KindDir {
				ps = append(ps, place{c, t.Path(c), t.Reclaimable(c)})
			}
		}
	}
	for _, c := range t.Children(t.Root()) {
		if hasHome && homeID == t.Root() {
			break
		}
		n := t.Node(c)
		if n.Kind != tree.KindDir || hasHome && (n.Name == "Users" || within(links.Real(home), t.Path(c))) {
			continue
		}
		ps = append(ps, place{c, t.Path(c), t.Reclaimable(c)})
	}
	slices.SortFunc(ps, func(a, b place) int { return cmp.Compare(b.bytes, a.bytes) })
	return ps[:min(maxPlaces, len(ps))]
}

// label names a place as people know it: ~/… in the home folder, /… at the
// top of the scanned disk.
func label(display, home, root string) string {
	if rest, ok := strings.CutPrefix(display, home+"/"); ok {
		return "~/" + rest
	}
	if rest, ok := strings.CutPrefix(display, strings.TrimSuffix(root, "/")+"/"); ok && root != "/" {
		return "/" + rest
	}
	return display
}

// ownerOf names who put a finding's data there; findings that span many apps
// or are the person's own get a group instead of repeating their kind.
func ownerOf(f finding.Finding) string {
	switch {
	case f.Owner != "":
		return f.Owner
	case f.Category == "stale-project":
		return "Your projects"
	case f.Category == "large-file", f.Category == "user-data", f.Category == "installer", f.Category == "duplicate", f.Category == "trash":
		return "You"
	}
	return "Various apps"
}

func within(p, root string) bool {
	return p == root || strings.HasPrefix(p, strings.TrimSuffix(root, "/")+"/")
}
