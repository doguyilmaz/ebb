package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/doguyilmaz/ebb/internal/units"
)

// WriteText prints a terminal report no wider than width columns.
func WriteText(w io.Writer, r *Report, width int) error {
	var b strings.Builder
	line := func(format string, args ...any) { b.WriteString(fit(fmt.Sprintf(format, args...), width) + "\n") }
	v := r.Volume
	line("ebb  %s  ·  scanned %s  ·  %d files in %s", r.Meta.Root, r.Meta.Time.Local().Format("2006-01-02 15:04"), r.Meta.Files, r.Meta.Elapsed.Round(1e8))
	line("Disk  %s total · %s used · %s free", units.Bytes(v.Capacity), units.Bytes(v.Used), units.Bytes(v.Free))
	parts := []string{"data " + units.Bytes(v.DataUsed)}
	if v.SystemUsed > 0 {
		parts = append(parts, "system "+units.Bytes(v.SystemUsed))
	}
	for _, o := range v.Others {
		if o.Used >= 100e6 {
			parts = append(parts, strings.ToLower(o.Role)+" "+units.Bytes(o.Used))
		}
	}
	if v.Purgeable > 0 {
		parts = append(parts, "purgeable "+units.Bytes(v.Purgeable))
	}
	if v.NotVisible > 0 {
		parts = append(parts, "not visible "+units.Bytes(v.NotVisible))
	}
	line("      %s", strings.Join(parts, " · "))
	actionable := 0
	for _, f := range r.Findings {
		if Actionable(f) {
			actionable++
		}
	}
	line("")
	line("Reclaimable  %s in %d findings ebb can act on", units.Bytes(r.Reclaimable), actionable)
	if len(r.Findings) > 0 {
		line("")
		line("Findings")
		for _, f := range r.Findings {
			where := ""
			if len(f.Targets) == 1 {
				where = f.Targets[0].Path
			} else if len(f.Targets) > 1 {
				where = fmt.Sprintf("%d locations", len(f.Targets))
			}
			line("  %9s  %-11s  %-36s  %s", units.Bytes(f.Bytes), f.Safety, f.Title, where)
			line("             %s", f.Why)
		}
	}
	line("")
	line("Largest folders")
	for _, row := range r.Breakdown[1:] {
		if row.Depth > 2 {
			continue
		}
		line("  %9s  %5.1f%%  %s%s", units.Bytes(row.Alloc), row.Share*100, strings.Repeat("  ", row.Depth-1), row.Path)
	}
	if len(r.Notes) > 0 {
		line("")
		line("Notes")
		for _, n := range r.Notes {
			for i, l := range wrap(n, width-4) {
				prefix := "  • "
				if i > 0 {
					prefix = "    "
				}
				line("%s%s", prefix, l)
			}
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func fit(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return strings.TrimRight(s, " ")
	}
	return string(r[:width-1]) + "…"
}

func wrap(s string, width int) []string {
	var lines []string
	cur := ""
	for _, word := range strings.Fields(s) {
		if cur != "" && len([]rune(cur))+1+len([]rune(word)) > width {
			lines = append(lines, cur)
			cur = ""
		}
		if cur != "" {
			cur += " "
		}
		cur += word
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}
