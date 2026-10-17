package assistant

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/sys"
)

func fake(t *testing.T, bins map[string]string) (*sys.Fake, *[]string) {
	t.Helper()
	home := t.TempDir()
	var ran []string
	env := &sys.Fake{HomeDir: home, GOOS: "darwin", Time: time.Now(), Bins: bins, Envs: map[string]string{sys.HomeEnv: home},
		RunFn: func(_ string, name string, args ...string) ([]byte, error) {
			ran = append(ran, filepath.Base(name)+" "+strings.Join(args, " "))
			return nil, nil
		}}
	return env, &ran
}

func TestInstallPrefersLockstep(t *testing.T) {
	env, ran := fake(t, map[string]string{"lockstep": "/bin/lockstep", "claude": "/bin/claude"})
	steps, err := Install(context.Background(), env, "/usr/local/bin/ebb", false)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(sys.ConfigDir(env), "skills", "ebb")
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Fatal("lockstep installs the skill from ebb's own copy")
	}
	want := []string{"lockstep skill add " + dir, "lockstep mcp add ebb -- /usr/local/bin/ebb mcp"}
	if strings.Join(*ran, "|") != strings.Join(want, "|") {
		t.Fatalf("ran %q", *ran)
	}
	if _, err := os.Stat(filepath.Join(env.Home(), ".claude", "skills", "ebb")); err == nil {
		t.Fatal("lockstep owns the providers; ebb writes none itself")
	}
	if len(steps) != 3 {
		t.Fatalf("steps %q", steps)
	}
}

func TestInstallFallsBackToClaude(t *testing.T) {
	env, ran := fake(t, map[string]string{"claude": "/bin/claude"})
	if _, err := Install(context.Background(), env, "/opt/ebb", false); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".claude/skills/ebb/SKILL.md", ".agents/skills/ebb/SKILL.md"} {
		b, err := os.ReadFile(filepath.Join(env.Home(), p))
		if err != nil || !strings.HasPrefix(string(b), "---\nname: ebb\n") {
			t.Fatalf("%s: %v", p, err)
		}
	}
	if strings.Join(*ran, "|") != "claude mcp add --scope user ebb -- /opt/ebb mcp" {
		t.Fatalf("ran %q", *ran)
	}
}

func TestInstallDryRunChangesNothing(t *testing.T) {
	env, ran := fake(t, map[string]string{"claude": "/bin/claude"})
	steps, err := Install(context.Background(), env, "/opt/ebb", true)
	if err != nil || len(steps) == 0 {
		t.Fatalf("steps %q %v", steps, err)
	}
	if len(*ran) != 0 {
		t.Fatalf("dry run ran %q", *ran)
	}
	if entries, _ := os.ReadDir(env.Home()); len(entries) != 0 {
		t.Fatalf("dry run wrote %v", entries)
	}
}
