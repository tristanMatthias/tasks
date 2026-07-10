package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/core"
	"github.com/tristanMatthias/tasks/pkg/store"
	"github.com/tristanMatthias/tasks/web"
)

func newKeyServer(t *testing.T, token string) (*httptest.Server, *core.Core) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	c, err := core.New(st, core.Options{Prefix: "proj", Actor: "srv"})
	if err != nil {
		t.Fatal(err)
	}
	srv := New(Config{Core: c, Static: web.Static(), Token: token, APIKeys: true})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, c
}

func TestKeyAuth(t *testing.T) {
	ts, c := newKeyServer(t, "sharedtok")
	k, err := c.CreateKey("bot", "tester")
	if err != nil {
		t.Fatal(err)
	}
	secret := k.Secret // full "tasks_<secret>" display token

	// The API key authorizes data access.
	if code, _ := do(t, ts, "GET", "/api/issues", secret, ""); code != http.StatusOK {
		t.Fatalf("valid key = %d, want 200", code)
	}
	// The shared token still works (base authenticator).
	if code, _ := do(t, ts, "GET", "/api/issues", "sharedtok", ""); code != http.StatusOK {
		t.Fatalf("shared token = %d, want 200", code)
	}
	// A malformed tasks_ bearer is rejected (never falls back to the base).
	if code, _ := do(t, ts, "GET", "/api/issues", core.TokenPrefix+"garbage", ""); code != http.StatusUnauthorized {
		t.Fatalf("bad key = %d, want 401", code)
	}
	// No credential -> 401.
	if code, _ := do(t, ts, "GET", "/api/issues", "", ""); code != http.StatusUnauthorized {
		t.Fatalf("no cred = %d, want 401", code)
	}

	// Revoke -> the key stops working immediately.
	if _, err := c.RevokeKey(k.ID); err != nil {
		t.Fatal(err)
	}
	if code, _ := do(t, ts, "GET", "/api/issues", secret, ""); code != http.StatusUnauthorized {
		t.Fatalf("revoked key = %d, want 401", code)
	}

	// authinfo still reports the base login mode (token), not "custom".
	_, body := do(t, ts, "GET", "/api/authinfo", "", "")
	if !strings.Contains(string(body), `"mode":"token"`) {
		t.Fatalf("authinfo should see through key layer to token mode: %s", body)
	}
}

func TestKeyAuthMintViaHTTP(t *testing.T) {
	ts, _ := newKeyServer(t, "sharedtok")
	// Mint a key over HTTP, then use the returned secret to authenticate.
	code, body := do(t, ts, "POST", "/api/v1/keys", "sharedtok", `{"label":"ci"}`)
	if code != http.StatusOK {
		t.Fatalf("mint = %d: %s", code, body)
	}
	secret := extractSecret(t, body)
	if !strings.HasPrefix(secret, core.TokenPrefix) {
		t.Fatalf("minted secret malformed: %q", secret)
	}
	if code, _ := do(t, ts, "GET", "/api/v1/ready", secret, ""); code != http.StatusOK {
		t.Fatalf("minted key auth = %d, want 200", code)
	}
}

func extractSecret(t *testing.T, body []byte) string {
	t.Helper()
	// tiny extraction to avoid pulling in the model type here
	s := string(body)
	i := strings.Index(s, `"secret":"`)
	if i < 0 {
		t.Fatalf("no secret in %s", s)
	}
	rest := s[i+len(`"secret":"`):]
	j := strings.IndexByte(rest, '"')
	return rest[:j]
}

func TestKeyAuthNoAuthBase(t *testing.T) {
	// With no shared token, no-auth is the base but keys still authorize; and a
	// bad key is still rejected rather than allowed through.
	ts, c := newKeyServer(t, "")
	k, _ := c.CreateKey("bot", "")
	if code, _ := do(t, ts, "GET", "/api/issues", k.Secret, ""); code != http.StatusOK {
		t.Fatalf("key with no-auth base = %d, want 200", code)
	}
	if code, _ := do(t, ts, "GET", "/api/issues", core.TokenPrefix+"bad", ""); code != http.StatusUnauthorized {
		t.Fatalf("bad key with no-auth base = %d, want 401", code)
	}
	// A non-tasks_ request falls through to no-auth (allowed).
	if code, _ := do(t, ts, "GET", "/api/issues", "", ""); code != http.StatusOK {
		t.Fatalf("no-auth base should allow uncredentialed = %d, want 200", code)
	}
}
