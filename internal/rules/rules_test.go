package rules

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/doguyilmaz/ebb"
	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/sys"
)

func fakeEnv(t *testing.T) *sys.Fake {
	home := t.TempDir()
	return &sys.Fake{HomeDir: home, GOOS: "darwin", Envs: map[string]string{sys.HomeEnv: home}}
}

func TestLoadBuiltinRules(t *testing.T) {
	set, err := Load(fakeEnv(t), ebb.Rules)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Rules) < 20 {
		t.Fatalf("only %d built-in rules", len(set.Rules))
	}
	seen := map[string]bool{}
	cats := map[string]bool{}
	for _, r := range set.Rules {
		if seen[r.ID] {
			t.Errorf("duplicate rule id %s", r.ID)
		}
		seen[r.ID] = true
		cats[r.Category] = true
		if r.Title == "" || r.Why == "" || r.Impact == "" || r.Safety.Rank() < 0 || len(r.Paths) == 0 && r.Match == nil {
			t.Errorf("rule %s incomplete: %+v", r.ID, r)
		}
		if r.Action.Kind != "" && !slices.Contains([]string{"command", "delete", "trash", "empty-trash", "manual"}, r.Action.Kind) {
			t.Errorf("rule %s: unknown action %q", r.ID, r.Action.Kind)
		}
		if r.Safety == finding.UserData && r.Action.Kind != "" {
			t.Errorf("rule %s: user data must be informational", r.ID)
		}
	}
	if len(set.Presets) < 6 {
		t.Fatalf("presets: %d", len(set.Presets))
	}
	for _, p := range set.Presets {
		for _, c := range p.Categories {
			if !cats[c] && !slices.Contains(DetectorCategories, c) {
				t.Errorf("preset %s: unknown category %s", p.ID, c)
			}
		}
		for _, id := range p.Rules {
			if !seen[id] {
				t.Errorf("preset %s: unknown rule %s", p.ID, id)
			}
		}
	}
}

func TestUserRuleOverrides(t *testing.T) {
	env := fakeEnv(t)
	dir := filepath.Join(sys.ConfigDir(env), "rules")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "mine.toml"), []byte(`
[[rule]]
id = "bun.cache"
title = "My bun cache"
category = "package-cache"
paths = ["~/.bun/install/cache"]
safety = "regenerable"
why = "w"
impact = "i"

[[rule]]
id = "my.thing"
title = "Thing"
category = "app-cache"
paths = ["~/thing"]
safety = "caution"
why = "w"
impact = "i"
`), 0o644)
	set, err := Load(env, ebb.Rules)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := set.Rule("bun.cache")
	if !ok || r.Title != "My bun cache" {
		t.Fatalf("override lost: %+v", r)
	}
	if _, ok := set.Rule("my.thing"); !ok {
		t.Fatal("new user rule missing")
	}
	if set.Rules[len(set.Rules)-1].ID != "my.thing" {
		t.Fatal("user rules come after built-ins")
	}
}

func TestBrokenUserRuleIsAnError(t *testing.T) {
	env := fakeEnv(t)
	dir := filepath.Join(sys.ConfigDir(env), "rules")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "bad.toml"), []byte("[[rule]\nid="), 0o644)
	if _, err := Load(env, ebb.Rules); err == nil {
		t.Fatal("broken file must fail loudly")
	}
}

func TestSizeParse(t *testing.T) {
	for in, want := range map[string]uint64{"500MB": 500e6, "1GiB": 1 << 30, "10 MiB": 10 << 20, "2GB": 2e9, "123": 123} {
		var s Size
		if err := s.UnmarshalText([]byte(in)); err != nil || uint64(s) != want {
			t.Errorf("%q → %d %v, want %d", in, s, err, want)
		}
	}
	var s Size
	if err := s.UnmarshalText([]byte("x")); err == nil {
		t.Fatal("garbage must fail")
	}
}

func TestExpand(t *testing.T) {
	env := &sys.Fake{HomeDir: "/h"}
	if got := Expand(env, "~/Library/Caches"); got != "/h/Library/Caches" {
		t.Fatal(got)
	}
	if got := Expand(env, "/abs"); got != "/abs" {
		t.Fatal(got)
	}
}
