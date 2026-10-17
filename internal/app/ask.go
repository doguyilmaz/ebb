package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/sys"
)

var ErrAskUnavailable = errors.New("Ask needs Claude Code (the claude command) on this Mac")

// claude runs with no tools at all (an empty allowlist, not a denylist that a
// new tool or the person's own MCP servers could slip past), in an empty
// folder: names on disk can carry instructions, and with nothing to call they
// stay words in a prompt.
var askArgs = []string{"-p", "--output-format", "json", "--tools", "", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--"}

func (a *App) AskAvailable() bool {
	if sys.Sandboxed(a.Env) {
		return false
	}
	_, err := a.Env.LookPath("claude")
	return err == nil
}

// Ask answers a question about a path or a finding with the person's own
// claude. The prompt carries ebb's metadata only, never file contents.
func (a *App) Ask(ctx context.Context, req api.AskRequest) (api.AskResponse, error) {
	if !a.AskAvailable() {
		return api.AskResponse{}, ErrAskUnavailable
	}
	q := strings.TrimSpace(req.Question)
	if q == "" {
		q = "Why is this so big, and is it safe to remove?"
	}
	if len(q) > 600 {
		q = q[:600]
	}
	facts, err := a.askFacts(req)
	if err != nil {
		return api.AskResponse{}, err
	}
	b, _ := json.MarshalIndent(facts, "", "  ")
	prompt := "You help someone understand what takes disk space on their Mac. Answer in Markdown, briefly (at most about 150 words): " +
		"what it is, why it grew, and whether removing it is safe and what that would cost. Use only the facts below, collected by " +
		"ebb from file metadata (paths, sizes, dates and what its rules know; never file contents). Do not invent paths or commands. " +
		"If ebb marks something as the person's own data, say they should decide themselves.\n\nFacts:\n```json\n" + string(b) +
		"\n```\n\nQuestion: " + q
	claude, err := a.Env.LookPath("claude")
	if err != nil {
		return api.AskResponse{}, ErrAskUnavailable
	}
	dir, err := os.MkdirTemp("", "ebb-ask-")
	if err != nil {
		return api.AskResponse{}, err
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	out, err := a.Env.Run(ctx, dir, claude, append(slices.Clone(askArgs), prompt)...)
	if err != nil {
		return api.AskResponse{}, fmt.Errorf("claude did not answer: %w", err)
	}
	var res struct {
		Result  string `json:"result"`
		IsError bool   `json:"is_error"`
	}
	if err := json.Unmarshal(out, &res); err != nil || res.Result == "" {
		return api.AskResponse{}, errors.New("claude gave no answer")
	}
	if res.IsError {
		return api.AskResponse{}, errors.New(res.Result)
	}
	return api.AskResponse{Answer: res.Result}, nil
}

func (a *App) askFacts(req api.AskRequest) (map[string]any, error) {
	brief := func(f api.FindingView) map[string]any {
		m := map[string]any{"title": f.Title, "category": f.Category, "safety": f.Safety, "bytes": f.Bytes, "why": f.Why, "impact": f.Impact}
		if len(f.Evidence) > 0 {
			m["evidence"] = f.Evidence[:min(5, len(f.Evidence))]
		}
		if len(f.Actions) > 0 {
			m["howEbbCleansIt"] = f.Actions[0].Kind
		}
		return m
	}
	if req.FindingID != "" {
		res, err := a.Findings("", 0, true)
		if err != nil {
			return nil, err
		}
		for _, f := range res.Findings {
			if f.ID == req.FindingID {
				m := brief(f)
				var where []map[string]any
				for _, t := range f.Targets[:min(10, len(f.Targets))] {
					where = append(where, map[string]any{"path": t.Path, "bytes": t.Bytes})
				}
				m["locations"] = where
				return m, nil
			}
		}
		return nil, fmt.Errorf("no finding %q in the last scan", req.FindingID)
	}
	ex, err := a.Explain(req.Path)
	if err != nil {
		return nil, err
	}
	it := ex.Item
	m := map[string]any{"path": it.Path, "kind": it.Kind, "allocatedBytes": it.Alloc, "reclaimableBytes": it.Bytes, "files": it.Files}
	if it.Modified > 0 {
		m["lastChanged"] = time.Unix(it.Modified, 0).UTC().Format("2006-01-02")
	}
	var fs, inside []map[string]any
	for _, f := range ex.Findings {
		fs = append(fs, brief(f))
	}
	for _, c := range ex.Biggest {
		inside = append(inside, map[string]any{"name": c.Name, "kind": c.Kind, "bytes": c.Alloc})
	}
	m["findings"], m["biggestInside"] = fs, inside
	return m, nil
}
