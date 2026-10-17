// Package detect turns a scanned tree, the volume accounting and the output
// of developer tools into findings.
package detect

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
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
	Links      volume.Firmlinks
	Now        time.Time
	StaleAfter time.Duration
	LargeFile  uint64
	// Covered reports whether a display path lies inside a target of a
	// pass-1 finding; set by Run before pass 2.
	Covered func(path string) bool
}

type Detector interface {
	ID() string
	Pass() int // 1 = specific, 2 = needs Covered
	Detect(ctx context.Context, c *Context) ([]finding.Finding, error)
}

// All lists every detector in the order findings are claimed.
func All() []Detector {
	return []Detector{rulesDetector{}}
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
