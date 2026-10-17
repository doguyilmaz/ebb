package detect

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/plist"
)

type derivedData struct{}

func (derivedData) ID() string { return "deriveddata" }
func (derivedData) Pass() int  { return 1 }

func (derivedData) Detect(ctx context.Context, c *Context) ([]finding.Finding, error) {
	root, ok := c.node(c.home("Library/Developer/Xcode/DerivedData"))
	if !ok {
		return nil, nil
	}
	action := []finding.Action{{Kind: "delete", Quit: []string{"Xcode"}}}
	orphaned := finding.Finding{RuleID: "deriveddata.orphaned", Title: "DerivedData for projects that no longer exist", Category: "build-artifact",
		Owner: "Xcode", Safety: finding.Regenerable,
		Why:     "Xcode keeps build products and indexes for every project it ever opened, even after the project is deleted or moved.",
		Impact:  "Nothing: the projects these belong to are gone.",
		Actions: action}
	rest := finding.Finding{RuleID: "deriveddata", Title: "DerivedData", Category: "build-artifact", Owner: "Xcode", Safety: finding.Regenerable,
		Why:     "Build products, module caches and indexes for every project Xcode has built.",
		Impact:  "The next build of each project is a full build and indexing starts over.",
		Actions: action}
	for _, id := range c.Tree.Children(root) {
		t := c.target(id)
		var info struct{ WorkspacePath string }
		if plist.DecodeFile(ctx, c.Env, filepath.Join(c.Tree.Path(id), "info.plist"), &info) == nil && info.WorkspacePath != "" && !c.exists(info.WorkspacePath) {
			orphaned.Targets = append(orphaned.Targets, t)
			orphaned.Evidence = append(orphaned.Evidence, fmt.Sprintf("%s is gone", info.WorkspacePath))
			continue
		}
		rest.Targets = append(rest.Targets, t)
	}
	var fs []finding.Finding
	for _, f := range []finding.Finding{orphaned, rest} {
		if len(f.Targets) > 0 {
			fs = append(fs, finish(f))
		}
	}
	return fs, nil
}
