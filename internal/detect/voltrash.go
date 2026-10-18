package detect

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/doguyilmaz/ebb/internal/finding"
)

// volumeTrash: on a disk other than the startup disk, what you move to the
// Trash goes to the disk's own .Trashes/<your user id> folder.
type volumeTrash struct{}

func (volumeTrash) ID() string { return "volume-trash" }
func (volumeTrash) Pass() int  { return 1 }

func (volumeTrash) Detect(_ context.Context, c *Context) ([]finding.Finding, error) {
	root := c.Tree.RootPath()
	rest, ok := strings.CutPrefix(root, "/Volumes/")
	if !ok || rest == "" || strings.Contains(rest, "/") {
		return nil, nil
	}
	id, ok := c.Tree.Lookup(filepath.Join(root, ".Trashes", strconv.Itoa(os.Getuid())))
	if !ok || c.Tree.Node(id).Alloc < 10<<20 {
		return nil, nil
	}
	return []finding.Finding{finish(finding.Finding{RuleID: "trash.volume", Title: "Trash on " + rest, Category: "trash", Safety: finding.Caution,
		Targets: []finding.Target{c.target(id)},
		Why:     "Items moved to the Trash from this disk stay on it, using its space, until the Trash is emptied.",
		Impact:  "Everything in this disk's Trash is deleted permanently.",
		Actions: []finding.Action{{Kind: "empty-trash"}}})}, nil
}
