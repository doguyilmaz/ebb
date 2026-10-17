package detect

import (
	"context"
	"fmt"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/units"
)

type docker struct{}

func (docker) ID() string { return "docker" }
func (docker) Pass() int  { return 1 }

func (docker) Detect(_ context.Context, c *Context) ([]finding.Finding, error) {
	id, ok := c.node(c.home("Library/Containers/com.docker.docker/Data/vms/0/data/Docker.raw"))
	if !ok {
		return nil, nil
	}
	n := c.Tree.Node(id)
	return []finding.Finding{finish(finding.Finding{RuleID: "docker.vm", Title: "Docker Desktop disk image", Category: "sdk", Owner: "Docker",
		Safety:   finding.Caution,
		Targets:  []finding.Target{c.target(id)},
		Why:      "Docker keeps images, containers, volumes and build cache inside one virtual disk that grows but never shrinks on its own.",
		Impact:   "Pruning removes unused images, stopped containers and build cache; running containers are not touched.",
		Evidence: []string{fmt.Sprintf("disk image is %s logically, %s allocated on disk", units.Bytes(n.Logical), units.Bytes(n.Alloc))},
		Actions:  []finding.Action{{Kind: "manual", Hint: "docker system prune -a --volumes  (then Docker Desktop → Settings → Resources → reduce the disk limit to reclaim the space)"}}})}, nil
}
