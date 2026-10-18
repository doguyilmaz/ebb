package clean

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/doguyilmaz/ebb/internal/apps"
	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/rules"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

const planLifetime = 10 * time.Minute

type Item struct {
	Path      string         `json:"path"`
	Title     string         `json:"title,omitempty"`
	Dev       int64          `json:"-"`
	Ino       uint64         `json:"-"`
	Bytes     uint64         `json:"bytes"`
	Safety    finding.Safety `json:"safety"`
	Action    finding.Action `json:"action"`
	Command   []string       `json:"command,omitempty"`
	FindingID string         `json:"findingId,omitempty"`
	Warnings  []string       `json:"warnings,omitempty"`
	Blocked   string         `json:"blocked,omitempty"`
}

type Plan struct {
	ID            string    `json:"id"`
	Created       time.Time `json:"created"`
	Expires       time.Time `json:"expires"`
	Source        string    `json:"source"`
	Items         []Item    `json:"items"`
	Bytes         uint64    `json:"bytes"`
	NeedsApproval bool      `json:"needsApproval"`
	Approved      bool      `json:"approved"`
}

type Request struct {
	FindingIDs []string `json:"findingIds,omitempty"`
	Paths      []string `json:"paths,omitempty"`
	Preset     string   `json:"preset,omitempty"`
	App        string   `json:"app,omitempty"`
	Source     string   `json:"source,omitempty"`
}

var ErrEmptyPlan = errors.New("nothing to clean: pick findings, a preset or paths")

type Planner struct {
	Env         sys.Env
	Tree        *tree.Tree // optional: sizes for arbitrary paths
	Links       volume.Firmlinks
	Findings    []finding.Finding
	Apps        []apps.App // for uninstall requests
	Rules       *rules.Set
	Protect     []string
	Roots       []string
	AutoExecute string // "regenerable" (default) | "never"
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Plan resolves a request to items with an action each, checks every item
// against the disk and the guard, and decides whether it needs approval.
func (p *Planner) Plan(req Request) (*Plan, error) {
	var items []Item
	byID := map[string]finding.Finding{}
	for _, f := range p.Findings {
		byID[f.ID] = f
	}
	for _, id := range req.FindingIDs {
		f, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown finding %q (rescan if the report changed)", id)
		}
		items = append(items, p.findingItems(f)...)
	}
	if req.Preset != "" {
		pr, ok := p.preset(req.Preset)
		if !ok {
			return nil, fmt.Errorf("unknown preset %q", req.Preset)
		}
		for _, f := range p.Findings {
			if PresetMatches(pr, f) {
				items = append(items, p.findingItems(f)...)
			}
		}
	}
	for _, path := range req.Paths {
		items = append(items, p.pathItem(path))
	}
	if req.App != "" {
		its, err := p.appItems(req.App)
		if err != nil {
			return nil, err
		}
		items = append(items, its...)
	}
	items = outermost(items)
	if len(items) == 0 {
		return nil, ErrEmptyPlan
	}
	now := p.Env.Now()
	plan := &Plan{ID: newID(), Created: now, Expires: now.Add(planLifetime), Source: req.Source, NeedsApproval: p.AutoExecute == "never"}
	for i := range items {
		p.check(&items[i])
		it := items[i]
		if it.Blocked == "" && it.Action.Kind != "manual" {
			plan.Bytes += it.Bytes
			if it.Safety.Rank() > finding.Regenerable.Rank() {
				plan.NeedsApproval = true
			}
		}
	}
	plan.Items = items
	return plan, nil
}

// PresetMatches reports whether a preset covers a finding.
func PresetMatches(pr rules.Preset, f finding.Finding) bool {
	return f.Actionable() && (slices.Contains(pr.Categories, f.Category) || slices.Contains(pr.Rules, f.RuleID)) &&
		f.Safety.Rank() <= pr.MaxSafety.Rank()
}

func (p *Planner) preset(id string) (rules.Preset, bool) {
	if p.Rules == nil {
		return rules.Preset{}, false
	}
	return p.Rules.Preset(id)
}

func (p *Planner) findingItems(f finding.Finding) []Item {
	if len(f.Actions) == 0 {
		return nil
	}
	var out []Item
	for _, t := range f.Targets {
		it := Item{Path: t.Path, Title: f.Title, Bytes: t.Bytes, Safety: f.Safety, Action: f.Actions[0], Command: t.Command, FindingID: f.ID}
		switch f.Safety {
		case finding.System:
			hint := it.Action.Hint
			if hint == "" {
				hint = "remove it with the tool that owns it"
			}
			it.Action = finding.Action{Kind: "manual", Hint: hint}
		case finding.UserData:
			it.Action = finding.Action{Kind: "trash", Quit: it.Action.Quit}
			it.Warnings = append(it.Warnings, "This is your own data; it goes to the Trash so you can still restore it.")
		}
		if it.Action.Kind == "command" && len(it.Command) == 0 {
			it.Command = it.Action.Argv
		}
		out = append(out, it)
	}
	return out
}

// pathItem is a folder or file the person picked in the explorer. Unless a
// finding says more about it, it is treated with caution and trashed.
func (p *Planner) pathItem(path string) Item {
	for _, f := range p.Findings {
		for _, t := range f.Targets {
			if t.Path == path && len(f.Actions) > 0 {
				items := p.findingItems(finding.Finding{ID: f.ID, Title: f.Title, Safety: f.Safety, Actions: f.Actions, Targets: []finding.Target{t}})
				return items[0]
			}
		}
	}
	it := Item{Path: path, Title: path[strings.LastIndex(path, "/")+1:], Safety: finding.Caution, Action: finding.Action{Kind: "trash"}}
	for _, f := range p.Findings {
		if f.Safety == finding.UserData && slices.ContainsFunc(f.Targets, func(t finding.Target) bool { return within(path, t.Path) }) {
			it.Safety = finding.UserData
			it.Warnings = append(it.Warnings, "This is your own data; it goes to the Trash so you can still restore it.")
		}
	}
	if p.Tree != nil {
		if id, ok := p.Tree.Lookup(p.Links.Real(path)); ok {
			it.Bytes = p.Tree.Reclaimable(id)
		}
	}
	return it
}

// appItems uninstalls an app: the bundle and everything it keeps in the
// Library go to the Trash, its launch agents stopped first.
func (p *Planner) appItems(name string) ([]Item, error) {
	i := slices.IndexFunc(p.Apps, func(a apps.App) bool { return strings.EqualFold(a.BundleID, name) || strings.EqualFold(a.Name, name) })
	if i < 0 {
		return nil, fmt.Errorf("no app %q in Applications (see ebb apps)", name)
	}
	a := p.Apps[i]
	if a.Apple {
		return nil, fmt.Errorf("%s comes with macOS or from Apple: ebb does not uninstall it", a.Name)
	}
	var quit []string
	if a.Executable != "" {
		quit = []string{a.Executable}
	}
	bundle := Item{Path: a.Path, Title: a.Name, Bytes: a.Bytes, Safety: finding.Caution, Action: finding.Action{Kind: "trash", Quit: quit}}
	if a.RootOwned {
		bundle.Action = finding.Action{Kind: "manual", Hint: "It was installed for every user (the App Store does this): drag it to the Trash in Finder, which asks for your password."}
	}
	items := []Item{bundle}
	for _, t := range a.Leftovers {
		it := Item{Path: t.Path, Title: a.Name + " data", Bytes: t.Bytes, Safety: finding.Caution, Action: finding.Action{Kind: "trash", Quit: quit}}
		if filepath.Base(filepath.Dir(t.Path)) == "LaunchAgents" {
			it.Action.Unload = true
			it.Warnings = append(it.Warnings, "A background helper: it is stopped first, so it does not start again.")
		}
		items = append(items, it)
	}
	return items, nil
}

// OwnDirs are ebb's config and state: never cleaned, or every protection would go with them.
func OwnDirs(env sys.Env) []string { return []string{sys.ConfigDir(env), sys.StateDir(env)} }

// scanned refuses what the last scan does not vouch for: a path it never saw
// (or spelled another way), a mounted disk or a folder holding one, and
// anything in a cloud-synced folder, where a deletion reaches every device.
func (p *Planner) scanned(real string) string {
	if p.Tree == nil {
		return ""
	}
	id, ok := p.Tree.Lookup(real)
	small := false
	if !ok {
		// Files under 1 MiB are counted in their folder, not listed: the folder vouches for them.
		parent, pok := p.Tree.Lookup(filepath.Dir(real))
		if !pok || !smallFile(real) {
			return "not in the last scan: scan again first"
		}
		id, small = parent, true
	}
	switch {
	case p.Tree.FlagAbove(id, tree.FlagSyncRoot):
		return "inside a cloud-synced folder: deleting here deletes it on every device"
	case p.Tree.Node(id).Flags&tree.FlagMount != 0:
		return "a mounted disk"
	case !small && p.Tree.FlagBelow(id, tree.FlagMount):
		return "holds a mounted disk"
	}
	return ""
}

func smallFile(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && st.Blocks*512 < 1<<20
}

// outermost drops items inside another item and duplicates.
func outermost(items []Item) []Item {
	slices.SortStableFunc(items, func(a, b Item) int { return cmp.Compare(a.Path, b.Path) })
	var out []Item
	for _, it := range items {
		if len(out) > 0 && within(it.Path, out[len(out)-1].Path) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// check fills Dev/Ino and Blocked from the disk, the guard and running apps.
func (p *Planner) check(it *Item) {
	if it.Action.Kind == "manual" {
		return
	}
	if it.Action.Kind == "command" {
		bin := it.Action.Requires
		if bin == "" && len(it.Command) > 0 {
			bin = it.Command[0]
		}
		if _, err := p.Env.LookPath(bin); err != nil {
			if it.Action.Fallback == "" {
				it.Blocked = bin + " is not installed"
				return
			}
			it.Warnings = append(it.Warnings, fmt.Sprintf("%s is not installed, so ebb will %s the folder instead", bin, it.Action.Fallback))
			it.Action.Kind, it.Command = it.Action.Fallback, nil
		}
	}
	real := p.Links.Real(it.Path)
	fi, err := os.Lstat(real)
	if err != nil {
		it.Blocked = "no longer exists"
		return
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		it.Dev, it.Ino = int64(st.Dev), st.Ino
	}
	g := Guard{Home: p.Env.Home(), Roots: p.Roots, Protect: p.Protect, Keep: OwnDirs(p.Env)}
	if it.Action.Kind != "empty-trash" {
		if err := g.Check(it.Path); err != nil {
			it.Blocked = err.Error()
			return
		}
		if why := p.scanned(real); why != "" {
			it.Blocked = why
			return
		}
	}
	for _, name := range it.Action.Quit {
		if running(p.Env, name) {
			it.Blocked = "Quit " + name + " first"
			return
		}
	}
}

func running(env sys.Env, name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	args := []string{"-x", "--", name}
	// Process names are cut at 16 characters; longer ones are matched by command line.
	if len(name) > 15 {
		args = []string{"-f", "/" + regexp.QuoteMeta(name) + "( |$)"}
	}
	out, err := env.Run(ctx, "", "pgrep", args...)
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}
