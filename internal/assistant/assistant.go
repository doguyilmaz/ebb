// Package assistant gives AI tools the ebb skill and registers `ebb mcp`. With
// lockstep installed it hands both to lockstep, which puts them into every
// tool and keeps them in sync across machines; otherwise it writes the skill
// for Claude Code and agents directly and registers the MCP server with the
// claude CLI.
package assistant

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/doguyilmaz/ebb/internal/sys"
)

//go:embed skills/ebb/SKILL.md
var Skill []byte

// Install sets ebb up for AI tools and returns what it did, one line each.
// With dryRun it only says what it would do.
func Install(ctx context.Context, env sys.Env, exe string, dryRun bool) ([]string, error) {
	var steps []string
	write := func(path string) error {
		steps = append(steps, "write "+path)
		if dryRun {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, Skill, 0o644)
	}
	run := func(bin string, args ...string) error {
		line := strings.Join(append([]string{filepath.Base(bin)}, args...), " ")
		if dryRun {
			steps = append(steps, "run "+line)
			return nil
		}
		if _, err := env.Run(ctx, "", bin, args...); err != nil {
			steps = append(steps, fmt.Sprintf("could not run %s: %v", line, err))
			return nil
		}
		steps = append(steps, "ran "+line)
		return nil
	}

	if lockstep, err := env.LookPath("lockstep"); err == nil {
		dir := filepath.Join(sys.ConfigDir(env), "skills", "ebb")
		if err := write(filepath.Join(dir, "SKILL.md")); err != nil {
			return steps, err
		}
		_ = run(lockstep, "skill", "add", dir)
		_ = run(lockstep, "mcp", "add", "ebb", "--", exe, "mcp")
		return steps, nil
	}
	for _, p := range []string{".claude/skills/ebb/SKILL.md", ".agents/skills/ebb/SKILL.md"} {
		if err := write(filepath.Join(env.Home(), p)); err != nil {
			return steps, err
		}
	}
	if claude, err := env.LookPath("claude"); err == nil {
		_ = run(claude, "mcp", "add", "--scope", "user", "ebb", "--", exe, "mcp")
	} else {
		steps = append(steps, "claude not found: add the MCP server to your tool by hand: ebb mcp (stdio)")
	}
	return steps, nil
}
