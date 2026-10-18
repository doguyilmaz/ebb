// Package report renders a scan as text, JSON, CSV or a self-contained HTML
// page, with notes that explain what the numbers do and do not include.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/history"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/units"
	"github.com/doguyilmaz/ebb/internal/volume"
)

type Row struct {
	Depth  int     `json:"depth"`
	Path   string  `json:"path"`
	Name   string  `json:"name"`
	Alloc  uint64  `json:"alloc"`
	Unique uint64  `json:"unique"`
	Files  uint32  `json:"files"`
	Share  float64 `json:"share"`
}

type Report struct {
	Version     string            `json:"version"`
	Generated   time.Time         `json:"generated"`
	Meta        history.Meta      `json:"meta"`
	Volume      volume.Breakdown  `json:"volume"`
	Findings    []finding.Finding `json:"findings"`
	Reclaimable uint64            `json:"reclaimable"`
	Breakdown   []Row             `json:"breakdown"`
	TopFiles    []Row             `json:"topFiles"`
	Problems    int               `json:"problems"`
	Notes       []string          `json:"notes"`
}

// Width is how wide text reports wrap, wherever they are made.
const Width = 100

// Notes are the report's explanations of the last scan (what is not visible,
// shared space, permissions), for answers that show findings without the report.
func Notes(a *app.App, version string) []string {
	notes := []string{}
	if last, err := a.Last(); err == nil {
		notes = append(notes, Build(a.View(last), a.Links, version).Notes...)
	}
	return notes
}

func Build(r *app.ScanResult, links volume.Firmlinks, version string) *Report {
	t := r.Tree
	root := t.Node(t.Root())
	rep := &Report{Version: version, Generated: time.Now(), Meta: r.Meta, Volume: r.Volume, Findings: r.Findings, Problems: r.ProblemCount}
	if rep.Findings == nil {
		rep.Findings = []finding.Finding{}
	}
	for _, f := range r.Findings {
		if f.Actionable() {
			rep.Reclaimable += f.Bytes
		}
	}
	share := func(n uint64) float64 {
		if root.Alloc == 0 {
			return 0
		}
		return float64(n) / float64(root.Alloc)
	}
	var walk func(s *tree.SliceNode, depth int, parent string)
	walk = func(s *tree.SliceNode, depth int, parent string) {
		path := parent
		if s.ID != tree.None {
			path = links.Display(t.Path(s.ID))
		}
		rep.Breakdown = append(rep.Breakdown, Row{Depth: depth, Path: path, Name: s.Name, Alloc: s.Alloc, Unique: s.Unique, Files: s.Files, Share: share(s.Alloc)})
		for _, c := range s.Children {
			walk(c, depth+1, path)
		}
	}
	walk(t.Slice(t.Root(), 4, 0.01, 80), 0, links.Display(t.RootPath()))
	rep.Breakdown[0].Name = links.Display(t.RootPath())
	for _, id := range t.Top(t.Root(), 25) {
		n := t.Node(id)
		rep.TopFiles = append(rep.TopFiles, Row{Path: links.Display(t.Path(id)), Name: n.Name, Alloc: n.Alloc, Unique: n.Unique, Files: 1, Share: share(n.Alloc)})
	}
	rep.Notes = notes(r, root)
	return rep
}

func notes(r *app.ScanResult, root tree.Node) []string {
	v := r.Volume
	var out []string
	fda := " Grant Full Disk Access to your terminal (System Settings → Privacy & Security → Full Disk Access) and scan again to see more."
	if v.NotVisible > 0 {
		n := fmt.Sprintf("%s of the data volume was not visible to this scan: system folders, other users' homes, apps' private data and local snapshots.", units.Bytes(v.NotVisible))
		if !v.FullDiskAccess {
			n += fda
		}
		out = append(out, n)
	}
	if root.Alloc > root.Unique {
		out = append(out, fmt.Sprintf("Allocated sizes count every copy, the way du does; %s is shared between APFS clones or hardlinks, so unique and reclaimable figures are lower. Clones that were modified after copying still count fully, so reclaimable figures may be slightly high for them.", units.Bytes(root.Alloc-root.Unique)))
	}
	if r.ProblemCount > 0 {
		n := fmt.Sprintf("%d folders or files could not be read; their contents are not counted.", r.ProblemCount)
		if !v.FullDiskAccess && v.NotVisible == 0 {
			n += fda
		}
		out = append(out, n)
	}
	if len(r.Errors) > 0 {
		out = append(out, "Some checks could not run: "+strings.Join(r.Errors, "; ")+".")
	}
	if n := len(v.Snapshots); n > 0 {
		out = append(out, fmt.Sprintf("%d local Time Machine snapshots exist; their space shows as used or purgeable until macOS removes them.", n))
	}
	if v.Purgeable > 0 {
		out = append(out, fmt.Sprintf("%s is purgeable: macOS frees it automatically when space is needed.", units.Bytes(v.Purgeable)))
	}
	for _, f := range r.Findings {
		if f.Notes != "" && !slices.Contains(out, f.Notes) {
			out = append(out, f.Notes)
		}
	}
	return out
}

func WriteJSON(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
