// Package detect turns a scanned tree, the volume accounting and the output
// of developer tools into findings.
package detect

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/history"
	"github.com/doguyilmaz/ebb/internal/rules"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

type Context struct {
	Tree       *tree.Tree
	Volume     volume.Breakdown
	Env        sys.Env
	Rules      *rules.Set
	History    *history.Store // nil: no growth findings
	Links      volume.Firmlinks
	Now        time.Time
	StaleAfter time.Duration
	LargeFile  uint64
	// Covered reports whether a display path lies inside a target of a
	// pass-1 finding; set by Run before pass 2.
	Covered func(path string) bool
	// Exists and ReadFile reach the real file system; nil means the OS.
	Exists   func(path string) bool
	ReadFile func(path string) ([]byte, error)
}

func (c *Context) exists(p string) bool {
	if c.Exists != nil {
		return c.Exists(p)
	}
	_, err := os.Stat(p)
	return err == nil
}

func (c *Context) readFile(p string) ([]byte, error) {
	if c.ReadFile != nil {
		return c.ReadFile(p)
	}
	return os.ReadFile(p)
}

// node finds a display path in the tree.
func (c *Context) node(display string) (tree.ID, bool) { return c.Tree.Lookup(c.Links.Real(display)) }

// target describes a tree node as a finding target.
func (c *Context) target(id tree.ID) finding.Target {
	return finding.Target{Path: c.Links.Display(c.Tree.Path(id)), Bytes: c.Tree.Reclaimable(id), Alloc: c.Tree.Node(id).Alloc}
}

func (c *Context) home(rel string) string { return filepath.Join(c.Env.Home(), rel) }

func daysAgo(now time.Time, t time.Time) string {
	d := int(now.Sub(t).Hours() / 24)
	if d == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", d)
}

// compareVersions orders dotted numeric versions ("18.6" < "26.5").
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(as), len(bs)) {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if c := cmp.Compare(x, y); c != 0 {
			return c
		}
	}
	return 0
}

func finish(f finding.Finding) finding.Finding {
	slices.SortFunc(f.Targets, func(a, b finding.Target) int { return cmp.Compare(b.Alloc, a.Alloc) })
	f.ID = finding.ID(f.RuleID, f.Targets[0].Path)
	f.Total()
	return f
}

type Detector interface {
	ID() string
	Pass() int // 1 = specific, 2 = needs Covered
	Detect(ctx context.Context, c *Context) ([]finding.Finding, error)
}

// All lists every detector in the order findings are claimed.
func All() []Detector {
	return []Detector{simulators{}, derivedData{}, xcode{}, android{}, docker{}, snapshots{}, stale{}, rulesDetector{}, large{}, growth{}}
}

func Run(ctx context.Context, c *Context) ([]finding.Finding, []error) { return RunWith(ctx, c, All()) }

func RunWith(ctx context.Context, c *Context, ds []Detector) ([]finding.Finding, []error) {
	var out []finding.Finding
	var errs []error
	run := func(pass int) {
		for _, d := range ds {
			if d.Pass() != pass {
				continue
			}
			fs, err := d.Detect(ctx, c)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", d.ID(), err))
			}
			out = append(out, fs...)
		}
	}
	run(1)
	c.Covered = coveredBy(out)
	run(2)
	slices.SortStableFunc(out, func(a, b finding.Finding) int { return cmp.Compare(b.Bytes, a.Bytes) })
	return out, errs
}

func coveredBy(fs []finding.Finding) func(string) bool {
	var paths []string
	for _, f := range fs {
		for _, t := range f.Targets {
			paths = append(paths, t.Path)
		}
	}
	return func(p string) bool {
		return slices.ContainsFunc(paths, func(t string) bool { return within(p, t) })
	}
}

// within reports whether p is root or below it.
func within(p, root string) bool {
	return p == root || strings.HasPrefix(p, strings.TrimSuffix(root, "/")+"/")
}

func overlaps(a, b string) bool { return within(a, b) || within(b, a) }
