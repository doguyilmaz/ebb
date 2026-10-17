package report

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"io"
	"strconv"

	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/units"
)

//go:embed html.tmpl
var htmlTemplate string

var page = template.Must(template.New("report").Funcs(template.FuncMap{
	"bytes":      units.Bytes,
	"pct":        func(f float64) string { return strconv.FormatFloat(f*100, 'f', 1, 64) + "%" },
	"indent":     func(d int) int { return (d - 1) * 16 },
	"actionable": Actionable,
}).Parse(htmlTemplate))

// WriteHTML writes a single self-contained page: the data is inlined and the
// treemap is drawn by an inline script, so it opens offline anywhere.
func WriteHTML(w io.Writer, r *Report, t *tree.Tree) error {
	var slice *tree.SliceNode
	if t != nil {
		slice = t.Slice(t.Root(), 2, 0.004, 600)
	}
	data, err := json.Marshal(map[string]any{"tree": slice})
	if err != nil {
		return err
	}
	return page.Execute(w, map[string]any{"R": r, "Data": template.JS(data)})
}
