package app

import (
	"net/url"
	"slices"
	"strings"
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

// WebOriginAllowed reports whether a page on origin may ask this machine for
// access at all (pairing still needs a confirmation here).
func (a *App) WebOriginAllowed(origin string) bool {
	o, ok := NormalizeOrigin(origin)
	if !ok {
		return false
	}
	return slices.ContainsFunc(a.Config.Server.WebOrigins, func(x string) bool {
		n, _ := NormalizeOrigin(x)
		return n == o
	})
}
