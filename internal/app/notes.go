package app

import (
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/state"
)

// Notes are the person's own words on a finding ("finding:<id>") or a path
// ("path:<path>"); they survive rescans and appear in every export.
func (a *App) Notes() map[string]string {
	m := map[string]string{}
	_ = state.LoadJSON(a.statePath("notes.json"), &m)
	return m
}

// SetNote stores a note; empty text removes it.
func (a *App) SetNote(key, text string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	m := a.Notes()
	if text == "" {
		delete(m, key)
	} else {
		m[key] = text
	}
	return state.SaveJSON(a.statePath("notes.json"), m)
}

type ignore struct {
	Bytes uint64    `json:"bytes"`
	Time  time.Time `json:"time"`
}

func (a *App) ignores() map[string]ignore {
	m := map[string]ignore{}
	_ = state.LoadJSON(a.statePath("ignores.json"), &m)
	return m
}

// Ignore hides a finding until it grows by half again.
func (a *App) Ignore(findingID string) error {
	r, err := a.Last()
	if err != nil {
		return err
	}
	var bytes uint64
	for _, f := range a.findings(r) {
		if f.ID == findingID {
			bytes = f.Bytes
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	m := a.ignores()
	m[findingID] = ignore{Bytes: bytes, Time: a.Env.Now()}
	return state.SaveJSON(a.statePath("ignores.json"), m)
}

func (a *App) Unignore(findingID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	m := a.ignores()
	delete(m, findingID)
	return state.SaveJSON(a.statePath("ignores.json"), m)
}

func (a *App) Ignores() map[string]time.Time {
	out := map[string]time.Time{}
	for k, v := range a.ignores() {
		out[k] = v.Time
	}
	return out
}

func ignored(ig map[string]ignore, f finding.Finding) bool {
	e, ok := ig[f.ID]
	return ok && f.Bytes < e.Bytes+e.Bytes/2
}

// View is a scan as the person sees it: ignored findings left out and their
// notes attached. Reports and the CLI show this, never the raw result.
func (a *App) View(r *ScanResult) *ScanResult {
	a.mu.Lock()
	v := *r
	a.mu.Unlock()
	notes, ig := a.Notes(), a.ignores()
	all := v.Findings
	v.Findings = nil
	for _, f := range all {
		if ignored(ig, f) {
			continue
		}
		f.UserNote = notes["finding:"+f.ID]
		v.Findings = append(v.Findings, f)
	}
	return &v
}

// findings reads r's findings, which a cleanup replaces (never edits) under a.mu.
func (a *App) findings(r *ScanResult) []finding.Finding {
	a.mu.Lock()
	defer a.mu.Unlock()
	return r.Findings
}
