package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/doguyilmaz/ebb/internal/state"
)

// PairWindow is how long a confirmation stays open. Long enough to switch to a
// terminal, short enough that an unnoticed request does not sit there all day.
const PairWindow = 5 * time.Minute

// ErrPairPending is returned when an origin may ask for the token but nobody at
// this machine has said yes yet.
var ErrPairPending = errors.New("waiting for someone at this machine to confirm")

// PendingPair is one unconfirmed request for the API token.
//
// The code exists so that the confirmation is about THIS request: the page shows
// it, `ebb web confirm` shows it, and a person compares the two. Without it
// a second page could ask at the same moment and be waved through by a yes that
// was meant for the first.
type PendingPair struct {
	Origin    string    `json:"origin"`
	Code      string    `json:"code"`
	Requested time.Time `json:"requested"`
	// Attempts counts wrong codes offered for this request. A confirmation is
	// only reachable with the local token, so this is not a remote brute-force
	// guard; it stops a local process from walking the six digits.
	Attempts int `json:"attempts"`
	// NonceHash binds the request to the page that made it: the token is
	// released only to a request presenting the same nonce, because any
	// process can put a confirmed origin in an Origin header.
	NonceHash string `json:"nonceHash"`
	Confirmed bool   `json:"confirmed"`
}

// ErrWeakNonce is returned for a pairing request without a proper secret.
var ErrWeakNonce = errors.New("pairing needs a random nonce of at least 24 characters")

func hashSecret(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// maxPairAttempts is how many wrong codes retire a request.
const maxPairAttempts = 5

// ErrPairBusy is returned when a different origin already has a confirmation
// waiting. Letting the second one overwrite the first is what would make a bare
// `ebb web confirm` land on a page the person was not looking at.
var ErrPairBusy = errors.New("another page is already waiting to be confirmed")

// Expired reports whether the window has closed.
func (p PendingPair) Expired(now time.Time) bool {
	return p.Requested.IsZero() || now.Sub(p.Requested) > PairWindow
}

func (a *App) pendingPairPath() string {
	return a.statePath("pending-pair.json")
}

// pairCode is six digits: short enough to read aloud, and only ever compared
// against one pending request inside a five minute window.
func pairCode() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	n := (int(b[0])<<16 | int(b[1])<<8 | int(b[2])) % 1000000
	return fmt.Sprintf("%06d", n)
}

// RequestPair records that a page on origin wants a token and returns the
// code to show on it. Asking again with the same nonce returns the same code,
// so a page that retries does not invalidate the code the person is reading.
// Another page (different origin or nonce) is refused while one is open:
// replacing it would let a bare `ebb web confirm` approve the wrong page.
func (a *App) RequestPair(origin, nonce string) (PendingPair, error) {
	norm, ok := NormalizeOrigin(origin)
	if !ok {
		return PendingPair{}, errInvalidOrigin(origin)
	}
	if len(nonce) < 24 {
		return PendingPair{}, ErrWeakNonce
	}
	nh := hashSecret(nonce)
	if p, err := a.PendingPair(); err == nil && p != nil {
		if p.Origin == norm && subtle.ConstantTimeCompare([]byte(p.NonceHash), []byte(nh)) == 1 {
			return *p, nil
		}
		return PendingPair{}, fmt.Errorf("%w (%s); confirm or reject that one first", ErrPairBusy, p.Origin)
	}
	p := PendingPair{Origin: norm, Code: pairCode(), Requested: a.Env.Now(), NonceHash: nh}
	if err := state.SaveJSON(a.pendingPairPath(), p); err != nil {
		return PendingPair{}, err
	}
	return p, nil
}

// ClaimPair releases a new token for origin when the open request was
// confirmed here and nonce is the one that request was made with. It works
// once: the request is gone afterwards.
func (a *App) ClaimPair(origin, nonce string) (string, bool, error) {
	norm, ok := NormalizeOrigin(origin)
	if !ok {
		return "", false, errInvalidOrigin(origin)
	}
	p, err := a.PendingPair()
	if err != nil || p == nil || !p.Confirmed || p.Origin != norm ||
		subtle.ConstantTimeCompare([]byte(p.NonceHash), []byte(hashSecret(nonce))) != 1 {
		return "", false, err
	}
	tok := state.NewToken()
	tokens, err := a.webTokens()
	if err != nil {
		return "", false, err
	}
	tokens = append(tokens, WebToken{Origin: norm, Hash: hashSecret(tok), Created: a.Env.Now()})
	if err := state.SaveJSON(a.statePath("web-tokens.json"), tokens); err != nil {
		return "", false, err
	}
	_ = a.clearPendingPair()
	return tok, true, nil
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

// PendingPair returns the open request, or nil when there is none or it expired.
func (a *App) PendingPair() (*PendingPair, error) {
	var p PendingPair
	if err := state.LoadJSON(a.pendingPairPath(), &p); err != nil {
		if state.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if p.Expired(a.Env.Now()) {
		return nil, nil
	}
	return &p, nil
}

// ConfirmPair is the yes. It is deliberately only reachable from this machine
// (the CLI or the local interface), never from the page that is asking. It
// approves the open request; the page then claims its token with its nonce.
//
// The code is required: comparing it with the one the page shows is what
// makes sure the page in front of the person is the one being let in, even if
// another allowed page asked a moment earlier.
func (a *App) ConfirmPair(code string) (PendingPair, error) {
	p, err := a.PendingPair()
	if err != nil {
		return PendingPair{}, err
	}
	if p == nil {
		return PendingPair{}, errors.New("nothing is waiting to be confirmed")
	}
	if code == "" {
		return PendingPair{}, fmt.Errorf("type the 6-digit code the page from %s shows, so you approve that page and no other", p.Origin)
	}
	if subtle.ConstantTimeCompare([]byte(code), []byte(p.Code)) != 1 {
		// Count the miss and retire the request once it has been guessed at
		// enough times, so the six digits cannot be walked.
		p.Attempts++
		if p.Attempts >= maxPairAttempts {
			_ = a.clearPendingPair()
			return PendingPair{}, fmt.Errorf("too many wrong codes; the request from %s was discarded, reload the page to ask again", p.Origin)
		}
		_ = state.SaveJSON(a.pendingPairPath(), *p)
		return PendingPair{}, fmt.Errorf("that code does not match the request from %s (%d of %d attempts used)", p.Origin, p.Attempts, maxPairAttempts)
	}
	p.Confirmed = true
	if err := state.SaveJSON(a.pendingPairPath(), *p); err != nil {
		return PendingPair{}, err
	}
	return *p, nil
}

// RejectPair discards the open request without confirming anything.
func (a *App) RejectPair() error { return a.clearPendingPair() }

func (a *App) clearPendingPair() error {
	if err := os.Remove(a.pendingPairPath()); err != nil && !state.IsNotExist(err) {
		return err
	}
	return nil
}
