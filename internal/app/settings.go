package app

import (
	"errors"
	"fmt"
	"os"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/rules"
)

func (a *App) Settings() api.Settings {
	c := a.reloadConfig()
	protect := c.Clean.Protect
	if protect == nil {
		protect = []string{}
	}
	return api.Settings{Root: c.Scan.Root, Threads: c.Scan.Threads, StaleDays: c.Scan.StaleDays, LargeFile: c.Scan.LargeFile, Protect: protect,
		BackgroundScans: c.Daemon.Scans, LowSpaceGB: c.Daemon.LowSpaceGB}
}

// SaveSettings validates and stores scan and cleanup preferences.
func (a *App) SaveSettings(s api.Settings) (api.Settings, error) {
	if s.Threads < 0 || s.Threads > 64 {
		return api.Settings{}, errors.New("threads must be between 0 (automatic) and 64")
	}
	if s.StaleDays < 1 || s.StaleDays > 3650 {
		return api.Settings{}, errors.New("stale days must be between 1 and 3650")
	}
	var size rules.Size
	if err := size.UnmarshalText([]byte(s.LargeFile)); err != nil || size < 1<<20 {
		return api.Settings{}, fmt.Errorf("large file threshold %q must be a size like 1GiB", s.LargeFile)
	}
	if s.LowSpaceGB < 0 || s.LowSpaceGB > 100_000 {
		return api.Settings{}, errors.New("the low space warning must be between 0 (never) and 100000 GB")
	}
	if s.Root != "" {
		if st, err := os.Stat(rules.Expand(a.Env, s.Root)); err != nil || !st.IsDir() {
			return api.Settings{}, fmt.Errorf("scan root %q is not a folder", s.Root)
		}
	}
	c := a.reloadConfig()
	c.Scan.Root, c.Scan.Threads, c.Scan.StaleDays, c.Scan.LargeFile, c.Clean.Protect = s.Root, s.Threads, s.StaleDays, s.LargeFile, s.Protect
	c.Daemon.Scans, c.Daemon.LowSpaceGB = s.BackgroundScans, s.LowSpaceGB
	if err := a.SaveConfig(c); err != nil {
		return api.Settings{}, err
	}
	return a.Settings(), nil
}
