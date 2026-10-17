package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/units"
	"github.com/doguyilmaz/ebb/internal/walk"
)

func (g *globals) app() (*app.App, error) { return app.New(sys.Real()) }

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// progress draws a one-line live counter on a terminal stderr.
func progress(f *os.File) func(walk.Progress) {
	if !isTerminal(f) {
		return nil
	}
	return func(p walk.Progress) {
		if p.Done {
			fmt.Fprint(f, "\r\033[K")
			return
		}
		fmt.Fprintf(f, "\r\033[Kscanning  %s files · %s · %s", thousands(p.Files), units.Bytes(p.Bytes), p.Elapsed.Round(100*time.Millisecond))
	}
}

func thousands(n uint64) string {
	s := fmt.Sprint(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// resolve finds a path given in any spelling (relative, display or real) in
// the scanned tree.
func resolve(a *app.App, t *tree.Tree, p string) (tree.ID, error) {
	if p == "" {
		return t.Root(), nil
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return 0, err
	}
	for _, c := range []string{a.Links.Real(abs), abs} {
		if id, ok := t.Lookup(c); ok {
			return id, nil
		}
	}
	return 0, fmt.Errorf("%s is not in the last scan (root %s)", p, a.Links.Display(t.RootPath()))
}
