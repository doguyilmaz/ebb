package detect

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
)

// simulators reads simctl's own view of devices and runtimes, so it knows
// which ones are booted, unavailable or simply forgotten.
type simulators struct{}

func (simulators) ID() string { return "simulators" }
func (simulators) Pass() int  { return 1 }

type simDevice struct {
	UDID         string    `json:"udid"`
	Name         string    `json:"name"`
	State        string    `json:"state"`
	IsAvailable  bool      `json:"isAvailable"`
	LastBootedAt time.Time `json:"lastBootedAt"`
	DataPath     string    `json:"dataPath"`
	DataPathSize uint64    `json:"dataPathSize"`
}

type simRuntime struct {
	Identifier         string    `json:"identifier"`
	Version            string    `json:"version"`
	RuntimeIdentifier  string    `json:"runtimeIdentifier"`
	PlatformIdentifier string    `json:"platformIdentifier"`
	SizeBytes          uint64    `json:"sizeBytes"`
	LastUsedAt         time.Time `json:"lastUsedAt"`
	Deletable          bool      `json:"deletable"`
	Path               string    `json:"path"`
}

func (simulators) Detect(ctx context.Context, c *Context) ([]finding.Finding, error) {
	// Without simulator data there is nothing to ask about, and on a Mac
	// without developer tools xcrun would offer to install them.
	if _, ok := c.node(c.home("Library/Developer/CoreSimulator/Devices")); !ok {
		return nil, nil
	}
	out, err := c.Env.Run(ctx, "", "xcrun", "simctl", "list", "devices", "-j")
	if err != nil {
		return nil, nil
	}
	var list struct {
		Devices map[string][]simDevice `json:"devices"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("simctl list: %w", err)
	}
	unavailable := finding.Finding{RuleID: "simulators.unavailable", Title: "Unavailable simulators", Category: "simulator", Owner: "Xcode",
		Safety:  finding.Regenerable,
		Why:     "These simulators belong to runtimes that are no longer installed, so they can never boot again.",
		Impact:  "Nothing you can still use is lost.",
		Actions: []finding.Action{{Kind: "command"}}}
	unused := finding.Finding{RuleID: "simulators.unused", Title: "Simulators unused for 30+ days", Category: "simulator", Owner: "Xcode",
		Safety:  finding.Caution,
		Why:     "Each simulator keeps its own copy of installed apps, data and caches, and grows with every run.",
		Impact:  "Apps and data inside those simulators are deleted; Xcode can create fresh ones any time.",
		Actions: []finding.Action{{Kind: "command", Quit: []string{"Simulator"}}}}
	recentRuntime := map[string]bool{}
	for runtime, devs := range list.Devices {
		for _, d := range devs {
			if d.State == "Booted" || c.Now.Sub(d.LastBootedAt) < c.StaleAfter {
				recentRuntime[runtime] = true
			}
			if d.State == "Booted" || d.DataPath == "" {
				continue
			}
			id, ok := c.node(filepath.Dir(d.DataPath))
			if !ok {
				continue
			}
			t := c.target(id)
			t.Command = []string{"xcrun", "simctl", "delete", d.UDID}
			switch {
			case !d.IsAvailable:
				unavailable.Targets = append(unavailable.Targets, t)
				unavailable.Evidence = append(unavailable.Evidence, fmt.Sprintf("%s (%s)", d.Name, runtimeName(runtime)))
			case c.Now.Sub(d.LastBootedAt) >= c.StaleAfter:
				unused.Targets = append(unused.Targets, t)
				last := "never booted"
				if !d.LastBootedAt.IsZero() {
					last = "last booted " + daysAgo(c.Now, d.LastBootedAt)
				}
				unused.Evidence = append(unused.Evidence, fmt.Sprintf("%s (%s): %s", d.Name, runtimeName(runtime), last))
			}
		}
	}
	var fs []finding.Finding
	for _, f := range []finding.Finding{unavailable, unused} {
		if len(f.Targets) > 0 {
			fs = append(fs, finish(f))
		}
	}
	if f, ok := c.oldRuntimes(ctx, recentRuntime); ok {
		fs = append(fs, f)
	}
	return fs, nil
}

func runtimeName(id string) string {
	name := strings.TrimPrefix(id, "com.apple.CoreSimulator.SimRuntime.")
	if os, ver, ok := strings.Cut(name, "-"); ok {
		return os + " " + strings.ReplaceAll(ver, "-", ".")
	}
	return name
}

// oldRuntimes offers runtimes that a newer one of the same platform
// replaces and that no recently used simulator still needs.
func (c *Context) oldRuntimes(ctx context.Context, recent map[string]bool) (finding.Finding, bool) {
	out, err := c.Env.Run(ctx, "", "xcrun", "simctl", "runtime", "list", "-j")
	if err != nil {
		return finding.Finding{}, false
	}
	var rts map[string]simRuntime
	if json.Unmarshal(out, &rts) != nil {
		return finding.Finding{}, false
	}
	newest := map[string]string{}
	for _, r := range rts {
		if compareVersions(r.Version, newest[r.PlatformIdentifier]) > 0 {
			newest[r.PlatformIdentifier] = r.Version
		}
	}
	f := finding.Finding{RuleID: "simulators.runtimes", Title: "Older simulator runtimes", Category: "simulator", Owner: "Xcode",
		Safety:  finding.Caution,
		Why:     "Each simulator runtime is a multi-gigabyte disk image, and Xcode keeps old ones after installing newer versions.",
		Impact:  "You can no longer test on these OS versions until you download them again in Xcode → Settings → Components.",
		Notes:   "Runtime images are mounted under /Library/Developer/CoreSimulator/Volumes; ebb counts each disk image once because it does not descend into mount points.",
		Actions: []finding.Action{{Kind: "command", Quit: []string{"Simulator"}}}}
	for _, r := range rts {
		if !r.Deletable || r.Version == newest[r.PlatformIdentifier] || recent[r.RuntimeIdentifier] || c.Now.Sub(r.LastUsedAt) < c.StaleAfter {
			continue
		}
		if _, inScan := c.node(r.Path); !inScan {
			continue
		}
		f.Targets = append(f.Targets, finding.Target{Path: c.Links.Display(r.Path), Bytes: r.SizeBytes, Alloc: r.SizeBytes,
			Command: []string{"xcrun", "simctl", "runtime", "delete", r.Identifier}})
		f.Evidence = append(f.Evidence, fmt.Sprintf("%s %s: last used %s", runtimeName(r.RuntimeIdentifier), r.Version, daysAgo(c.Now, r.LastUsedAt)))
	}
	if len(f.Targets) == 0 {
		return f, false
	}
	return finish(f), true
}
