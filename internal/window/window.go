// Package window decides where ebb's window opens: the hosted page, connected
// through a one-time link, or ebb's own page on loopback.
package window

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/sys"
)

// Target is the page to open, asked for by name or else the first allowed
// one; "" means ebb's own window.
func Target(ctx context.Context, env sys.Env, allowed []string, asked string) (string, error) {
	return origin(allowed, asked, safari(defaultBrowser(ctx, env)))
}

// Open hands url to the default browser.
func Open(env sys.Env, url string) error {
	if sys.Sandboxed(env) {
		return nil
	}
	return exec.Command("open", url).Start()
}

// origin is the page the window opens on; "" means ebb's own
// window. Safari keeps https pages away from loopback, so it gets the window.
func origin(allowed []string, asked string, safari bool) (string, error) {
	if asked != "" {
		o, ok := app.NormalizeOrigin(asked)
		if !ok || !slices.Contains(allowed, o) {
			return "", fmt.Errorf("this machine has not allowed %s; run: ebb web allow %s", asked, asked)
		}
		return o, nil
	}
	if safari || len(allowed) == 0 {
		return "", nil
	}
	return allowed[0], nil
}

// Link is the address that opens origin connected; it carries the secret in the fragment, which never reaches a server.
func Link(a *app.App, origin string, port int, token string) (string, error) {
	if origin == "" {
		return fmt.Sprintf("http://127.0.0.1:%d/#token=%s", port, token), nil
	}
	key, err := a.IssueLinkKey(origin)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/#key=%s&port=%d", origin, key, port), nil
}

// defaultBrowser is the bundle id of the app that opens https links, read from
// LaunchServices; "" when it cannot tell.
func defaultBrowser(ctx context.Context, env sys.Env) string {
	if env.OS() != "darwin" {
		return ""
	}
	plist := filepath.Join(env.Home(), "Library", "Preferences", "com.apple.LaunchServices", "com.apple.launchservices.secure.plist")
	raw, err := env.Run(ctx, "", "plutil", "-convert", "json", "-o", "-", plist)
	if err != nil {
		return ""
	}
	return httpsHandler(raw)
}

func httpsHandler(raw []byte) string {
	var ls struct {
		LSHandlers []struct {
			Scheme string `json:"LSHandlerURLScheme"`
			App    string `json:"LSHandlerRoleAll"`
		}
	}
	if json.Unmarshal(raw, &ls) != nil {
		return ""
	}
	for _, h := range ls.LSHandlers {
		if h.Scheme == "https" {
			return h.App
		}
	}
	// Nobody picked another browser.
	return "com.apple.Safari"
}

// safari counts an unknown browser too: ebb's own window works in every browser.
func safari(bundleID string) bool {
	return bundleID == "" || strings.HasPrefix(strings.ToLower(bundleID), "com.apple.safari")
}
