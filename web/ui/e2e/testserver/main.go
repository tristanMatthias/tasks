//go:build e2etestserver

// Command testserver runs the real httpapi engine in "custom" auth mode (an
// embedder-style cookie session), so the e2e suite can exercise the browser
// login/logout paths that token mode never touches — the ones where the two
// logout regressions hid. Test-only; not shipped.
//
//	testserver -addr 127.0.0.1:PORT -db /tmp/x.db -prefix e2e
//
// Auth: a first-party HMAC-signed cookie (mode reports "custom"). GET /login sets
// it (simulating an OAuth callback) and redirects; POST /api/logout clears it.
// API seeding uses a fixed bearer token so the harness can create tasks without
// the browser flow.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tristanMatthias/tasks/pkg/core"
	"github.com/tristanMatthias/tasks/pkg/httpapi"
	"github.com/tristanMatthias/tasks/pkg/store"
	"github.com/tristanMatthias/tasks/web"
)

const (
	sessionCookie = "e2e_session"
	seedToken     = "e2e-seed-token"
	loginPath     = "/login"
)

var secret = []byte("e2e-custom-auth-secret")

// customAuth is a minimal embedder-style authenticator: anything that isn't
// noAuth/tokenAuth makes the engine report mode "custom". It trusts the signed
// session cookie (browser) or a fixed bearer token (harness seeding).
type customAuth struct{}

func (customAuth) Authorize(r *http.Request) (httpapi.Identity, bool) {
	if h := r.Header.Get("Authorization"); h == "Bearer "+seedToken {
		return httpapi.Identity{Subject: "e2e-seed"}, true
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil || !valid(c.Value) {
		return httpapi.Identity{}, false
	}
	return httpapi.Identity{Subject: "e2e-user"}, true
}

// Login/Logout make customAuth a LoginProvider so POST /api/logout is mounted.
func (customAuth) Login(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }
func (customAuth) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "listen address")
	db := flag.String("db", "", "sqlite path")
	prefix := flag.String("prefix", "e2e", "id prefix")
	flag.Parse()

	st, err := store.Open(*db)
	if err != nil {
		log.Fatal(err)
	}
	c, err := core.New(st, core.Options{Prefix: *prefix, Actor: "e2e"})
	if err != nil {
		log.Fatal(err)
	}
	srv := httpapi.New(httpapi.Config{
		Core:     c,
		Static:   web.Static(),
		Auth:     customAuth{},
		LoginURL: loginPath, // the landing page's "log in" target
		// Delete only for the cookie session (a human), never the seed bearer.
		AllowDelete: func(r *http.Request) bool {
			id, ok := httpapi.IdentityFrom(r.Context())
			return ok && id.Subject == "e2e-user"
		},
	})

	mux := http.NewServeMux()
	// GET /login: establish the session cookie (as an OAuth callback would) and
	// return to the requested same-site path. Public (ahead of the auth gate).
	mux.HandleFunc("GET "+loginPath, func(w http.ResponseWriter, r *http.Request) {
		dest := "/"
		if rp := r.URL.Query().Get("redirect_url"); strings.HasPrefix(rp, "/") && !strings.HasPrefix(rp, "//") {
			dest = rp
		}
		http.SetCookie(w, &http.Cookie{
			Name: sessionCookie, Value: sign(), Path: "/", HttpOnly: true,
			SameSite: http.SameSiteLaxMode, MaxAge: 3600,
		})
		http.Redirect(w, r, dest, http.StatusSeeOther)
	})
	// Stub the GitHub integration endpoints so the settings UI can be exercised.
	mux.HandleFunc("GET /api/integrations/github", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connected":false,"repos":[],"connect_url":"/integrations/github/connect","app_slug":"agenttasks-test"}`))
	})
	mux.HandleFunc("GET /integrations/github/connect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/apps/agenttasks-test/installations/new", http.StatusSeeOther)
	})

	// Seed a github-authored comment (what the webhook records) so the activity
	// rendering can be exercised.
	mux.HandleFunc("POST /e2e/gh-comment", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("task")
		text := r.URL.Query().Get("text")
		if _, err := c.Comment(id, text, "github"); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.Handle("/", srv.Handler())

	log.Printf("testserver listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

func sign() string {
	payload := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func valid(v string) bool {
	i := strings.IndexByte(v, '.')
	if i < 0 {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(v[i+1:])
	if err != nil {
		return false
	}
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(v[:i]))
	return hmac.Equal(sig, h.Sum(nil))
}
