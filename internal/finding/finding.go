// Package finding is the shared vocabulary of the report: what was found,
// why it is large, what removing it costs and how safe that is.
package finding

type Safety string

const (
	Regenerable Safety = "regenerable" // tools rebuild or re-download it
	Caution     Safety = "caution"     // safe to remove, but losing it costs something
	UserData    Safety = "user-data"   // the person's own data: guidance only
	System      Safety = "system"      // only through the OS or an admin command
)

// Rank orders safety from least to most risky; -1 for unknown values.
func (s Safety) Rank() int {
	switch s {
	case Regenerable:
		return 0
	case Caution:
		return 1
	case UserData:
		return 2
	case System:
		return 3
	}
	return -1
}

type Action struct {
	Kind     string   `json:"kind" toml:"kind"` // command | delete | trash | empty-trash | manual
	Argv     []string `json:"argv,omitempty" toml:"argv"`
	Requires string   `json:"requires,omitempty" toml:"requires"`
	Fallback string   `json:"fallback,omitempty" toml:"fallback"`
	Hint     string   `json:"hint,omitempty" toml:"hint"`
	Quit     []string `json:"quit,omitempty" toml:"quit"`
	Unload   bool     `json:"unload,omitempty" toml:"unload"` // a launch agent: stopped before it goes
}

type Target struct {
	Path    string   `json:"path"`
	Bytes   uint64   `json:"bytes"`
	Alloc   uint64   `json:"alloc"`
	Command []string `json:"command,omitempty"`
}

type Finding struct {
	ID       string   `json:"id"` // stable across scans: one finding per rule
	RuleID   string   `json:"ruleId"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Owner    string   `json:"owner,omitempty"`
	Safety   Safety   `json:"safety"`
	Bytes    uint64   `json:"bytes"`
	Alloc    uint64   `json:"alloc"`
	Targets  []Target `json:"targets"`
	Why      string   `json:"why"`
	Impact   string   `json:"impact"`
	Evidence []string `json:"evidence,omitempty"`
	Actions  []Action `json:"actions,omitempty"`
	Notes    string   `json:"notes,omitempty"`
	UserNote string   `json:"userNote,omitempty"` // the person's own note, added when shown
}

// Total sums target bytes into the finding.
func (f *Finding) Total() {
	f.Bytes, f.Alloc = 0, 0
	for _, t := range f.Targets {
		f.Bytes += t.Bytes
		f.Alloc += t.Alloc
	}
}

// Actionable reports whether ebb can carry out the default action itself
// (manual and informational findings only explain).
func (f Finding) Actionable() bool { return len(f.Actions) > 0 && f.Actions[0].Kind != "manual" }
