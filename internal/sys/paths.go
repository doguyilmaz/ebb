package sys

import "path/filepath"

// ConfigDir is where config.toml lives. Under LOCKSTEP_HOME everything is
// relative to that home so a sandbox is fully self-contained.
func ConfigDir(env Env) string {
	if env.Getenv(HomeEnv) != "" {
		return filepath.Join(env.Home(), ".config", "ebb")
	}
	switch env.OS() {
	case "windows":
		if a := env.Getenv("APPDATA"); a != "" {
			return filepath.Join(a, "ebb")
		}
		return filepath.Join(env.Home(), "AppData", "Roaming", "ebb")
	default:
		if x := env.Getenv("XDG_CONFIG_HOME"); x != "" {
			return filepath.Join(x, "ebb")
		}
		return filepath.Join(env.Home(), ".config", "ebb")
	}
}

// StateDir holds device identity, caches, sync bases and conflict copies.
func StateDir(env Env) string {
	if env.Getenv(HomeEnv) != "" {
		return filepath.Join(env.Home(), ".local", "state", "ebb")
	}
	switch env.OS() {
	case "windows":
		if a := env.Getenv("LOCALAPPDATA"); a != "" {
			return filepath.Join(a, "ebb")
		}
		return filepath.Join(env.Home(), "AppData", "Local", "ebb")
	default:
		if x := env.Getenv("XDG_STATE_HOME"); x != "" {
			return filepath.Join(x, "ebb")
		}
		return filepath.Join(env.Home(), ".local", "state", "ebb")
	}
}
