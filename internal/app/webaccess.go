package app

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/config"
)

// NormalizeOrigin reduces an origin to scheme://host[:port], lowercased.
// Plain http is accepted only for loopback hosts.
func NormalizeOrigin(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.User != nil {
		return "", false
	}
	scheme, host := strings.ToLower(u.Scheme), strings.ToLower(u.Host)
	switch scheme {
	case "https":
	case "http":
		h := u.Hostname()
		if h != "localhost" && h != "127.0.0.1" && h != "::1" {
			return "", false
		}
	default:
		return "", false
	}
	return scheme + "://" + host, true
}

func errInvalidOrigin(o string) error {
	return fmt.Errorf("%q is not an origin like https://example.com (plain http only for localhost)", o)
}

// reloadConfig re-reads config.toml so a grant made by the CLI reaches a
// server that is already running.
func (a *App) reloadConfig() config.Config {
	if cfg, err := config.Load(a.Env); err == nil {
		a.mu.Lock()
		a.Config = cfg
		a.mu.Unlock()
		return cfg
	}
	return a.Config
}

func (a *App) SaveConfig(cfg config.Config) error {
	if err := config.Save(a.Env, cfg); err != nil {
		return err
	}
	a.mu.Lock()
	a.Config = cfg
	a.mu.Unlock()
	return nil
}

// WebOriginAllowed reports whether a page on origin may ask this machine for
// access at all (it still needs a link from ebb open).
func (a *App) WebOriginAllowed(origin string) bool {
	o, ok := NormalizeOrigin(origin)
	if !ok {
		return false
	}
	return slices.Contains(a.reloadConfig().Server.WebOrigins, o)
}

// WebAccess is what the local UI shows about web pairing.
func (a *App) WebAccess() api.WebAccess {
	cfg := a.reloadConfig()
	out := api.WebAccess{Allowed: cfg.Server.WebOrigins, Paired: []string{}}
	ts, _ := a.webTokens()
	for _, t := range ts {
		if !slices.Contains(out.Paired, t.Origin) {
			out.Paired = append(out.Paired, t.Origin)
		}
	}
	return out
}

func (a *App) AllowWebOrigin(origin string) (api.WebAccess, error) {
	norm, ok := NormalizeOrigin(origin)
	if !ok {
		return api.WebAccess{}, errInvalidOrigin(origin)
	}
	cfg := a.reloadConfig()
	if !slices.Contains(cfg.Server.WebOrigins, norm) {
		cfg.Server.WebOrigins = append(slices.Clone(cfg.Server.WebOrigins), norm)
		slices.Sort(cfg.Server.WebOrigins)
		if err := a.SaveConfig(cfg); err != nil {
			return api.WebAccess{}, err
		}
	}
	return a.WebAccess(), nil
}

// DenyWebOrigin revokes a grant and its confirmation.
func (a *App) DenyWebOrigin(origin string) (api.WebAccess, error) {
	norm, ok := NormalizeOrigin(origin)
	if !ok {
		return api.WebAccess{}, errInvalidOrigin(origin)
	}
	cfg := a.reloadConfig()
	before := len(cfg.Server.WebOrigins)
	cfg.Server.WebOrigins = slices.DeleteFunc(slices.Clone(cfg.Server.WebOrigins), func(o string) bool { return o == norm })
	if len(cfg.Server.WebOrigins) == before {
		return a.WebAccess(), fmt.Errorf("%s was not granted access", norm)
	}
	if err := a.SaveConfig(cfg); err != nil {
		return api.WebAccess{}, err
	}
	if err := a.RevokeWebTokens(norm); err != nil {
		return api.WebAccess{}, err
	}
	return a.WebAccess(), nil
}
