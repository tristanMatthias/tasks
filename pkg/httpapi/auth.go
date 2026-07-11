package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

const cookieName = "tasks_token"

// Identity describes an authenticated principal. Subject is a stable id
// ("token" for shared-token mode, a user id for an embedder's auth); Claims
// carries extra data (e.g. an org/tenant id) that a CoreResolver can read.
type Identity struct {
	Subject string
	Claims  map[string]string
}

// Authenticator authorizes requests. Built-in implementations are none (allow
// all — loopback/dev) and token (a shared bearer token). Embedders — e.g. a
// separate multi-tenant control plane — can supply their own (say, a
// JWT-verifying authenticator) without tasksd knowing anything about it.
type Authenticator interface {
	// Authorize returns the identity for a request and whether it is allowed.
	Authorize(r *http.Request) (Identity, bool)
}

// LoginProvider is an optional interface an Authenticator may implement to power
// the browser login flow. When present, the server mounts POST /api/login and
// POST /api/logout to it (both public, ahead of the auth gate).
type LoginProvider interface {
	Login(w http.ResponseWriter, r *http.Request)
	Logout(w http.ResponseWriter, r *http.Request)
}

type identityKeyT int

const identityKey identityKeyT = 0

// IdentityFrom returns the authenticated identity stored in the request context.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

// ---- built-in authenticators ----

// noAuth allows every request (loopback / local dev).
type noAuth struct{}

func (noAuth) Authorize(*http.Request) (Identity, bool) { return Identity{Subject: "local"}, true }

// tokenAuth authorizes a shared bearer token supplied as an Authorization header
// (CLI/MCP/curl) or the session cookie (browser), and powers a token login.
type tokenAuth struct{ token string }

func (t tokenAuth) Authorize(r *http.Request) (Identity, bool) {
	if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " && constEq(h[7:], t.token) {
		return Identity{Subject: "token"}, true
	}
	if c, err := r.Cookie(cookieName); err == nil && constEq(c.Value, t.token) {
		return Identity{Subject: "token"}, true
	}
	return Identity{}, false
}

// Login sets the session cookie when the posted token matches. Accepts the token
// from a JSON body {"token":...} or the ?token= query param.
func (t tokenAuth) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	tok := body.Token
	if tok == "" {
		tok = r.URL.Query().Get("token")
	}
	if !constEq(tok, t.token) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid token"}`))
		return
	}
	setSessionCookie(w, tok)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Logout clears the session cookie.
func (t tokenAuth) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func setSessionCookie(w http.ResponseWriter, v string) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: v, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

// ---- middleware ----

// authGate wraps the app: it authorizes the request via the configured
// Authenticator, stashes the Identity in context, and 401s otherwise.
func (s *Server) authGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.auth.Authorize(r)
		if !ok {
			challenge := `Bearer realm="tasks"`
			if s.resourceMetadataURL != "" {
				// RFC 9728: point OAuth-capable clients at the resource metadata.
				challenge = `Bearer resource_metadata="` + s.resourceMetadataURL + `"`
			}
			w.Header().Set("WWW-Authenticate", challenge)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey, id)))
	})
}

// baseAuthenticator sees through the API-key layer to the underlying token/none
// authenticator, so login-mode detection and the legacy bootstrap are unaffected
// by whether keys are enabled.
func baseAuthenticator(a Authenticator) Authenticator {
	if ka, ok := a.(keyAuth); ok {
		return ka.baseAuth()
	}
	return a
}

// legacyAuthBootstrap keeps the GET /auth?token=SECRET cookie-setter working
// (used by existing deployments) when the authenticator supports token login.
func (s *Server) legacyAuthBootstrap(w http.ResponseWriter, r *http.Request) {
	ta, ok := baseAuthenticator(s.auth).(tokenAuth)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if tok := r.URL.Query().Get("token"); constEq(tok, ta.token) {
		setSessionCookie(w, tok)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.Error(w, "invalid token", http.StatusUnauthorized)
}

// authMode returns a UI hint for the login screen.
func (s *Server) authMode() string {
	switch baseAuthenticator(s.auth).(type) {
	case noAuth:
		return "none"
	case tokenAuth:
		return "token"
	default:
		return "custom"
	}
}

// handleAuthInfo (public) tells the UI which login flow to present and whether
// the current request is already authenticated.
func (s *Server) handleAuthInfo(w http.ResponseWriter, r *http.Request) {
	_, authed := s.auth.Authorize(r)
	out := map[string]any{"mode": s.authMode(), "authenticated": authed}
	if s.loginURL != "" {
		out["login_url"] = s.loginURL
	}
	writeJSON(w, http.StatusOK, out)
}

func constEq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
