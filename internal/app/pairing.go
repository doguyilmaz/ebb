package app

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/state"
)

func hashSecret(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func (a *App) issueWebToken(origin string) (string, error) {
	tokens, err := a.webTokens()
	if err != nil {
		return "", err
	}
	tok := state.NewToken()
	tokens = append(tokens, WebToken{Origin: origin, Hash: hashSecret(tok), Created: a.Env.Now()})
	return tok, state.SaveJSON(a.statePath("web-tokens.json"), tokens)
}

// LinkWindow is how long the link `ebb open` opens can connect its page.
const LinkWindow = 5 * time.Minute

// linkKey lets the page `ebb open` opened claim a token once. The key itself
// travels only in the link's fragment. Each key is a file named by its hash, so
// the CLI adding one never rewrites another the server is claiming.
type linkKey struct {
	Origin  string    `json:"origin"`
	Created time.Time `json:"created"`
}

func (a *App) linkKeyDir() string { return a.statePath("link-keys") }

func (a *App) linkKeyPath(key string) string {
	return filepath.Join(a.linkKeyDir(), hashSecret(key)+".json")
}

// IssueLinkKey makes the key for a link that connects a page on origin.
// Running the command that asks for it is the consent: only someone at this
// machine can.
func (a *App) IssueLinkKey(origin string) (string, error) {
	norm, ok := NormalizeOrigin(origin)
	if !ok {
		return "", errInvalidOrigin(origin)
	}
	if !a.WebOriginAllowed(norm) {
		return "", fmt.Errorf("this machine has not allowed %s; run: ebb web allow %s", norm, norm)
	}
	a.pruneLinkKeys()
	key := state.NewToken()
	return key, state.SaveJSON(a.linkKeyPath(key), linkKey{Origin: norm, Created: a.Env.Now()})
}

func (a *App) pruneLinkKeys() {
	es, _ := os.ReadDir(a.linkKeyDir())
	for _, e := range es {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		var k linkKey
		p := filepath.Join(a.linkKeyDir(), e.Name())
		if state.LoadJSON(p, &k) != nil || a.Env.Now().Sub(k.Created) > LinkWindow {
			_ = os.Remove(p)
		}
	}
}

// ClaimLinkKey trades a key from IssueLinkKey for a token of the page's own.
// The key works once, within LinkWindow and only for the origin it was made
// for; a claim from another origin leaves it in place. Claims come only from
// the server, so a lock makes them one at a time: unlink does not, since APFS
// lets concurrent unlinks of one file all succeed.
func (a *App) ClaimLinkKey(origin, key string) (string, bool, error) {
	norm, ok := NormalizeOrigin(origin)
	if !ok {
		return "", false, errInvalidOrigin(origin)
	}
	a.pairMu.Lock()
	defer a.pairMu.Unlock()
	p := a.linkKeyPath(key)
	var k linkKey
	if err := state.LoadJSON(p, &k); err != nil {
		if state.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if k.Origin != norm || a.Env.Now().Sub(k.Created) > LinkWindow {
		return "", false, nil
	}
	if err := os.Remove(p); err != nil {
		return "", false, err
	}
	tok, err := a.issueWebToken(norm)
	return tok, err == nil, err
}

// WebToken is a token issued to a paired page; only its hash is stored.
type WebToken struct {
	Origin  string    `json:"origin"`
	Hash    string    `json:"hash"`
	Created time.Time `json:"created"`
}

func (a *App) webTokens() ([]WebToken, error) {
	var ts []WebToken
	if err := state.LoadJSON(a.statePath("web-tokens.json"), &ts); err != nil && !state.IsNotExist(err) {
		return nil, err
	}
	return ts, nil
}

// WebTokenValid reports whether tok was issued to a page whose origin is
// still allowed.
func (a *App) WebTokenValid(tok string) bool {
	if tok == "" {
		return false
	}
	ts, err := a.webTokens()
	if err != nil {
		return false
	}
	h := []byte(hashSecret(tok))
	allowed := a.reloadConfig().Server.WebOrigins
	for _, t := range ts {
		if subtle.ConstantTimeCompare([]byte(t.Hash), h) == 1 && slices.Contains(allowed, t.Origin) {
			return true
		}
	}
	return false
}

// RevokeWebTokens drops every token issued to origin.
func (a *App) RevokeWebTokens(origin string) error {
	a.pairMu.Lock()
	defer a.pairMu.Unlock()
	ts, err := a.webTokens()
	if err != nil {
		return err
	}
	ts = slices.DeleteFunc(ts, func(t WebToken) bool { return t.Origin == origin })
	return state.SaveJSON(a.statePath("web-tokens.json"), ts)
}
