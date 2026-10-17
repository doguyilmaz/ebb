package report

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
)

// WriteCSV writes one row per finding target, breakdown folder and large file.
func WriteCSV(w io.Writer, r *Report) error {
	cw := csv.NewWriter(w)
	u := func(n uint64) string { return strconv.FormatUint(n, 10) }
	rows := [][]string{{"kind", "path", "title", "category", "safety", "bytes", "alloc", "evidence"}}
	for _, f := range r.Findings {
		for _, t := range f.Targets {
			rows = append(rows, []string{"finding", t.Path, f.Title, f.Category, string(f.Safety), u(t.Bytes), u(t.Alloc), strings.Join(f.Evidence, "; ")})
		}
		if len(f.Targets) == 0 {
			rows = append(rows, []string{"finding", "", f.Title, f.Category, string(f.Safety), u(f.Bytes), u(f.Alloc), strings.Join(f.Evidence, "; ")})
		}
	}
	for _, b := range r.Breakdown {
		rows = append(rows, []string{"breakdown", b.Path, b.Name, "", "", u(b.Unique), u(b.Alloc), ""})
	}
	for _, f := range r.TopFiles {
		rows = append(rows, []string{"file", f.Path, f.Name, "", "", u(f.Unique), u(f.Alloc), ""})
	}
	for _, r := range rows {
		for i, cell := range r {
			r[i] = csvSafe(cell)
		}
	}
	if err := cw.WriteAll(rows); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// csvSafe keeps spreadsheet apps from evaluating a file name as a formula.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}
