package httpapi

import (
	"net/http"
	"strings"

	"github.com/tristanMatthias/tasks/pkg/core"
)

// keyAuth adds DB-backed API-key authentication in front of a base
// authenticator. A request carrying "Authorization: Bearer tasks_<secret>" is
// verified against the core's key store; any other request falls through to the
// base (shared-token or no-auth), which also powers the browser login. This is
// the single-tenant path — a multi-tenant host implements its own key routing.
type keyAuth struct {
	base   Authenticator
	verify func(secret string) (Identity, bool)
}

// newKeyAuth wraps base with API-key verification against c.
func newKeyAuth(base Authenticator, c *core.Core) keyAuth {
	return keyAuth{
		base: base,
		verify: func(secret string) (Identity, bool) {
			k, err := c.VerifyKey(secret)
			if err != nil {
				return Identity{}, false
			}
			return Identity{Subject: "key:" + k.ID}, true
		},
	}
}

func (k keyAuth) Authorize(r *http.Request) (Identity, bool) {
	if secret, ok := bearerSecret(r); ok {
		// A tasks_ bearer is an API-key attempt: verify it or reject — never fall
		// back to the base for a malformed/revoked key.
		return k.verify(secret)
	}
	return k.base.Authorize(r)
}

// Login/Logout delegate to the base authenticator when it supports them, so the
// browser token-login flow keeps working with key auth layered on top.
func (k keyAuth) Login(w http.ResponseWriter, r *http.Request) {
	if lp, ok := k.base.(LoginProvider); ok {
		lp.Login(w, r)
		return
	}
	http.NotFound(w, r)
}

func (k keyAuth) Logout(w http.ResponseWriter, r *http.Request) {
	if lp, ok := k.base.(LoginProvider); ok {
		lp.Logout(w, r)
		return
	}
	http.NotFound(w, r)
}

// baseAuth exposes the wrapped authenticator so authMode/legacy bootstrap can
// see through the key layer to the underlying token/none mode.
func (k keyAuth) baseAuth() Authenticator { return k.base }

// bearerSecret extracts the raw secret from an "Authorization: Bearer tasks_…"
// header. In single-tenant mode the material after the prefix is the secret.
func bearerSecret(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const p = "Bearer " + core.TokenPrefix
	if strings.HasPrefix(h, p) {
		return strings.TrimSpace(h[len(p):]), true
	}
	return "", false
}
