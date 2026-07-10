package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/tristanMatthias/tasks/pkg/model"
)

// TokenPrefix is the display prefix on every API token, so a token is
// recognizable on sight and distinguishable from other bearer credentials
// (e.g. a JWT) by an authenticator. The material after the prefix is the raw
// secret in single-tenant mode; a multi-tenant host may insert a routing
// selector (see the control plane) — but only the raw secret is ever hashed.
const TokenPrefix = "tasks_"

// ErrKeyRevoked is returned by VerifyKey when the key exists but was revoked.
var ErrKeyRevoked = errors.New("api key revoked")

// HashSecret returns the hex SHA-256 of a raw secret — the value persisted and
// compared. The raw secret itself is never stored.
func HashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// CreateKey mints an API key: it generates a random secret, stores only its
// hash, and returns the key with Secret set to the display token ("tasks_" +
// secret) — shown to the caller exactly once and never recoverable after.
func (c *Core) CreateKey(label, actor string) (*model.APIKey, error) {
	secret := randToken(40)
	k := model.APIKey{
		ID:        "k" + randSuffix(8),
		Hash:      HashSecret(secret),
		Label:     label,
		CreatedBy: c.actorOr(actor),
		CreatedAt: c.now(),
	}
	if err := c.st.InsertKey(k); err != nil {
		return nil, wrap("create key", err)
	}
	out := k
	out.Hash = "" // never hand the hash back to callers; only the one-time secret
	out.Secret = TokenPrefix + secret
	if c.keySelector != "" {
		// Route-first token: tasks_<selector>_<secret>. Only <secret> is hashed.
		out.Secret = TokenPrefix + c.keySelector + "_" + secret
	}
	return &out, nil
}

// VerifyKey resolves a raw secret (already stripped of the "tasks_" prefix and
// any routing selector) to its key, recording last-used. Returns ErrNotFound if
// no key matches and ErrKeyRevoked if the matching key was revoked.
func (c *Core) VerifyKey(secret string) (*model.APIKey, error) {
	k, err := c.st.KeyByHash(HashSecret(secret))
	if err != nil {
		return nil, err
	}
	if k.Revoked() {
		return nil, ErrKeyRevoked
	}
	now := c.now()
	_ = c.st.TouchKey(k.ID, now) // best-effort audit; never fails the auth
	k.LastUsedAt = now
	return k, nil
}

// ListKeys returns all keys (active + revoked), newest first, without secrets.
func (c *Core) ListKeys() ([]model.APIKey, error) { return c.st.ListKeys() }

// RevokeKey revokes a key by id and returns the updated record. Revoking an
// unknown or already-revoked id is a not-found error, so the result is
// unambiguous to the caller.
func (c *Core) RevokeKey(id string) (*model.APIKey, error) {
	ok, err := c.st.RevokeKey(id, c.now())
	if err != nil {
		return nil, wrap("revoke key", err)
	}
	if !ok {
		return nil, ErrNotFound
	}
	return c.st.KeyByID(id)
}

// SplitToken parses a display token into its routing selector and raw secret.
// The secret is base62 (never contains "_"), so the split is on the LAST "_":
//   - "tasks_<secret>"            -> selector "",   secret "<secret>"
//   - "tasks_<selector>_<secret>" -> selector, secret  (selector may contain "_")
//
// ok is false if the token lacks the "tasks_" prefix. Only the secret is hashed;
// the selector is used purely to route a bare token to the right Core.
func SplitToken(token string) (selector, secret string, ok bool) {
	if !strings.HasPrefix(token, TokenPrefix) {
		return "", "", false
	}
	rest := token[len(TokenPrefix):]
	if i := strings.LastIndexByte(rest, '_'); i >= 0 {
		return rest[:i], rest[i+1:], true
	}
	return "", rest, true
}

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// randToken returns an n-char base62 secret from a CSPRNG. base62 has no "_" so
// the token stays unambiguous when a routing selector is joined with "_".
func randToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = byte(i)
		}
	}
	out := make([]byte, n)
	for i, c := range b {
		out[i] = base62[int(c)%len(base62)]
	}
	return string(out)
}
