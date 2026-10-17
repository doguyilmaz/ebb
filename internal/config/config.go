// Package config reads and writes ~/.config/ebb/config.toml.
package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/doguyilmaz/ebb/internal/state"
	"github.com/doguyilmaz/ebb/internal/sys"
)

type Scan struct {
	Root      string `toml:"root,omitempty"`
	Threads   int    `toml:"threads,omitempty"`
	StaleDays int    `toml:"stale_days"`
	LargeFile string `toml:"large_file"`
}

type Server struct {
	Port         int      `toml:"port"`
	WebOrigins   []string `toml:"web_origins"`
	WebConfirmed []string `toml:"web_confirmed"`
}

type Clean struct {
	Protect []string `toml:"protect"`
}

type MCP struct {
	// AutoExecute lets AI agents run plans without a confirmation when every
	// item is "regenerable"; "never" always asks.
	AutoExecute string `toml:"auto_execute"`
}

type Config struct {
	Scan   Scan   `toml:"scan"`
	Server Server `toml:"server"`
	Clean  Clean  `toml:"clean"`
	MCP    MCP    `toml:"mcp"`
}

const (
	DefaultPort         = 47340
	DefaultHostedOrigin = "https://ebb.apps.doguyilmaz.com"
)

func Default() Config {
	return Config{
		Scan:   Scan{StaleDays: 30, LargeFile: "1GiB"},
		Server: Server{Port: DefaultPort, WebOrigins: []string{DefaultHostedOrigin}},
		MCP:    MCP{AutoExecute: "regenerable"},
	}
}

func Path(env sys.Env) string { return filepath.Join(sys.ConfigDir(env), "config.toml") }

// Load returns the defaults overlaid with config.toml when it exists.
func Load(env sys.Env) (Config, error) {
	c := Default()
	b, err := os.ReadFile(Path(env))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := toml.Unmarshal(b, &c); err != nil {
		return c, err
	}
	return c, nil
}

func Save(env sys.Env, c Config) error {
	b, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	return state.WriteAtomic(Path(env), b, 0o600)
}
