package app

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"slices"
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

// linkKey lets the page `ebb open` opened claim a token once. Only its hash is
// stored; the key itself travels in the link's fragment, which no server sees.
type linkKey struct {
	Origin  string    `json:"origin"`
	Hash    string    `json:"hash"`
	Created time.Time `json:"created"`
}

func (a *App) linkKeysPath() string { return a.statePath("link-keys.json") }

func (a *App) liveLinkKeys() ([]linkKey, error) {
	var ks []linkKey
	if err := state.LoadJSON(a.linkKeysPath(), &ks); err != nil && !state.IsNotExist(err) {
		return nil, err
	}
	now := a.Env.Now()
	return slices.DeleteFunc(ks, func(k linkKey) bool { return now.Sub(k.Created) > LinkWindow }), nil
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
	a.keysMu.Lock()
	defer a.keysMu.Unlock()
	ks, err := a.liveLinkKeys()
	if err != nil {
		return "", err
	}
	key := state.NewToken()
	ks = append(ks, linkKey{Origin: norm, Hash: hashSecret(key), Created: a.Env.Now()})
	return key, state.SaveJSON(a.linkKeysPath(), ks)
}

// ClaimLinkKey trades a key from IssueLinkKey for a token of the page's own.
// The key works once, within LinkWindow and only for the origin it was made
// for; a claim from another origin leaves it in place.
func (a *App) ClaimLinkKey(origin, key string) (string, bool, error) {
	norm, ok := NormalizeOrigin(origin)
	if !ok {
		return "", false, errInvalidOrigin(origin)
	}
	a.keysMu.Lock()
	defer a.keysMu.Unlock()
	ks, err := a.liveLinkKeys()
	if err != nil {
		return "", false, err
	}
	h := []byte(hashSecret(key))
	i := slices.IndexFunc(ks, func(k linkKey) bool {
		return k.Origin == norm && subtle.ConstantTimeCompare([]byte(k.Hash), h) == 1
	})
	if i < 0 {
		return "", false, nil
	}
	if err := state.SaveJSON(a.linkKeysPath(), slices.Delete(ks, i, i+1)); err != nil {
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
	ts, err := a.webTokens()
	if err != nil {
		return err
	}
	ts = slices.DeleteFunc(ts, func(t WebToken) bool { return t.Origin == origin })
	return state.SaveJSON(a.statePath("web-tokens.json"), ts)
}
