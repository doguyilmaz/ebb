// Package rules loads the data-driven cleanup rules: built-in TOML files
// embedded in the binary, then the person's own in ~/.config/ebb/rules.
package rules

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/sys"
)

// DetectorCategories are produced by Go detectors rather than data rules.
var DetectorCategories = []string{"simulator", "build-artifact", "sdk", "stale-project", "large-file", "duplicate",
	"leftover", "snapshot", "growth"}

const (
	defaultMinSize   = 100 << 20
	defaultMinTarget = 10 << 20
)

type Match struct {
	Name    string   `toml:"name"`    // glob on the directory name
	Project bool     `toml:"project"` // parent directory must be a project
	Under   []string `toml:"under"`   // only below these paths (~ expanded)
}

type Rule struct {
	ID        string         `toml:"id"`
	Title     string         `toml:"title"`
	Owner     string         `toml:"owner"`
	Category  string         `toml:"category"`
	Paths     []string       `toml:"paths"`
	Match     *Match         `toml:"match"`
	Safety    finding.Safety `toml:"safety"`
	Why       string         `toml:"why"`
	Impact    string         `toml:"impact"`
	MinSize   Size           `toml:"min_size"`   // whole finding; default 100 MB
	MinTarget Size           `toml:"min_target"` // each location; default 10 MB
	Action    finding.Action `toml:"action"`
}

type Preset struct {
	ID         string         `toml:"id" json:"id"`
	Title      string         `toml:"title" json:"title"`
	Categories []string       `toml:"categories" json:"categories,omitempty"`
	Rules      []string       `toml:"rules" json:"rules,omitempty"`
	MaxSafety  finding.Safety `toml:"max_safety" json:"maxSafety"`
}

type Set struct {
	Rules   []Rule
	Presets []Preset
}

type file struct {
	Rule   []Rule   `toml:"rule"`
	Preset []Preset `toml:"preset"`
}

// Load reads built-in files in name order, then user files; a user rule with
// a built-in rule's ID replaces it in place.
func Load(env sys.Env, builtin fs.FS) (*Set, error) {
	set := &Set{}
	names, err := fs.Glob(builtin, "rules/*.toml")
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	for _, n := range names {
		b, err := fs.ReadFile(builtin, n)
		if err != nil {
			return nil, err
		}
		if err := set.add(n, b); err != nil {
			return nil, err
		}
	}
	user, _ := filepath.Glob(filepath.Join(sys.ConfigDir(env), "rules", "*.toml"))
	slices.Sort(user)
	for _, n := range user {
		b, err := os.ReadFile(n)
		if err != nil {
			return nil, err
		}
		if err := set.add(n, b); err != nil {
			return nil, err
		}
	}
	return set, nil
}

func (s *Set) add(name string, b []byte) error {
	var f file
	if err := toml.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	for _, r := range f.Rule {
		if r.ID == "" || r.Safety.Rank() < 0 {
			return fmt.Errorf("%s: rule %q needs an id and a valid safety", name, r.ID)
		}
		if r.MinSize == 0 {
			r.MinSize = defaultMinSize
		}
		if r.MinTarget == 0 {
			r.MinTarget = defaultMinTarget
		}
		if i := slices.IndexFunc(s.Rules, func(x Rule) bool { return x.ID == r.ID }); i >= 0 {
			s.Rules[i] = r
		} else {
			s.Rules = append(s.Rules, r)
		}
	}
	for _, p := range f.Preset {
		if i := slices.IndexFunc(s.Presets, func(x Preset) bool { return x.ID == p.ID }); i >= 0 {
			s.Presets[i] = p
		} else {
			s.Presets = append(s.Presets, p)
		}
	}
	return nil
}

func (s *Set) Rule(id string) (Rule, bool) {
	i := slices.IndexFunc(s.Rules, func(r Rule) bool { return r.ID == id })
	if i < 0 {
		return Rule{}, false
	}
	return s.Rules[i], true
}

func (s *Set) Preset(id string) (Preset, bool) {
	i := slices.IndexFunc(s.Presets, func(p Preset) bool { return p.ID == id })
	if i < 0 {
		return Preset{}, false
	}
	return s.Presets[i], true
}

// Expand replaces a leading ~ with the home directory.
func Expand(env sys.Env, p string) string {
	if p == "~" {
		return env.Home()
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(env.Home(), rest)
	}
	return p
}

// Size is a byte count written as "500MB", "1GiB" or a plain number.
type Size uint64

func (s *Size) UnmarshalText(b []byte) error {
	t := strings.TrimSpace(string(b))
	i := strings.IndexFunc(t, func(r rune) bool { return (r < '0' || r > '9') && r != '.' })
	num, unit := t, ""
	if i >= 0 {
		num, unit = strings.TrimSpace(t[:i]), strings.ToUpper(strings.TrimSpace(t[i:]))
	}
	v, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return fmt.Errorf("size %q: %w", t, err)
	}
	mult := map[string]float64{"": 1, "B": 1, "KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12,
		"KIB": 1 << 10, "MIB": 1 << 20, "GIB": 1 << 30, "TIB": 1 << 40}
	m, ok := mult[unit]
	if !ok {
		return fmt.Errorf("size %q: unknown unit", t)
	}
	*s = Size(v * m)
	return nil
}
