package httpapi

import (
	"crypto/subtle"
	"net/http"
)

const cookieName = "tasks_token"

// auth wraps a handler with bearer-token / cookie authentication. When the
// server has no token configured (dev mode) it is a pass-through.
//
// Accepted credentials (any one):
//   - Authorization: Bearer <token>   (MCP / CLI / curl)
//   - Cookie tasks_token=<token>       (browser UI, set via /auth?token=...)
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" {
			next.ServeHTTP(w, r)
			return
		}
		// One-time cookie bootstrap: /auth?token=SECRET sets the cookie then
		// redirects to the UI root, so the browser never needs the header.
		if r.URL.Path == "/auth" {
			if tok := r.URL.Query().Get("token"); constEq(tok, s.token) {
				http.SetCookie(w, &http.Cookie{
					Name: cookieName, Value: tok, Path: "/",
					HttpOnly: true, SameSite: http.SameSiteLaxMode,
				})
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		if s.authorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="tasks"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

func (s *Server) authorized(r *http.Request) bool {
	if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
		if constEq(h[7:], s.token) {
			return true
		}
	}
	if c, err := r.Cookie(cookieName); err == nil && constEq(c.Value, s.token) {
		return true
	}
	return false
}

func constEq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
