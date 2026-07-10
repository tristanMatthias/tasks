package model

// APIKey is a long-lived credential a bot/agent uses to authenticate to a task
// board (CLI, MCP, or REST) without an interactive login. Only the secret's
// hash is ever persisted; the plaintext is shown exactly once, at mint time, in
// the Secret field. The hash is never serialized to any surface.
type APIKey struct {
	ID         string `json:"id"`                     // short public identifier (e.g. "k7f3a9c2")
	Hash       string `json:"-"`                      // sha256 hex of the raw secret; never exposed
	Label      string `json:"label,omitempty"`        // human note ("ci", "claude-web")
	CreatedBy  string `json:"created_by,omitempty"`   // actor that minted it
	CreatedAt  string `json:"created_at,omitempty"`   // RFC3339
	LastUsedAt string `json:"last_used_at,omitempty"` // RFC3339; updated on each successful auth
	RevokedAt  string `json:"revoked_at,omitempty"`   // RFC3339; set once revoked (empty = active)
	Secret     string `json:"secret,omitempty"`       // the display token, populated ONLY by CreateKey
}

// Revoked reports whether the key has been revoked.
func (k APIKey) Revoked() bool { return k.RevokedAt != "" }
