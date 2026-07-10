package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/store"
)

func TestKeyLifecycle(t *testing.T) {
	c := newCore(t)

	k, err := c.CreateKey("ci-bot", "alice")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if !strings.HasPrefix(k.Secret, TokenPrefix) {
		t.Fatalf("secret missing prefix: %q", k.Secret)
	}
	if k.Hash != "" {
		t.Fatalf("hash must never be exposed on the returned key, got %q", k.Hash)
	}
	if k.Label != "ci-bot" || k.CreatedBy != "alice" {
		t.Fatalf("metadata not set: %+v", k)
	}
	secret := strings.TrimPrefix(k.Secret, TokenPrefix)

	// Verify with the raw secret.
	got, err := c.VerifyKey(secret)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.ID != k.ID {
		t.Fatalf("verify returned %s, want %s", got.ID, k.ID)
	}
	if got.LastUsedAt == "" {
		t.Fatalf("last_used_at should be recorded on verify")
	}

	// Wrong secret -> not found.
	if _, err := c.VerifyKey("nope"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("wrong secret err = %v, want ErrNotFound", err)
	}

	// List shows the key without a secret or hash.
	list, err := c.ListKeys()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v (err %v), want 1", list, err)
	}
	if list[0].Secret != "" || list[0].Hash != "" {
		t.Fatalf("list leaked secret/hash: %+v", list[0])
	}
	if list[0].Revoked() {
		t.Fatalf("key should be active")
	}

	// Revoke -> verify now fails as revoked.
	rk, err := c.RevokeKey(k.ID)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if !rk.Revoked() {
		t.Fatalf("revoked key should report Revoked()")
	}
	if _, err := c.VerifyKey(secret); !errors.Is(err, ErrKeyRevoked) {
		t.Fatalf("verify after revoke = %v, want ErrKeyRevoked", err)
	}

	// Double revoke / unknown id -> not found.
	if _, err := c.RevokeKey(k.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("double revoke = %v, want ErrNotFound", err)
	}
	if _, err := c.RevokeKey("k-nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoke unknown = %v, want ErrNotFound", err)
	}
}

func TestHashAndTokenGeneration(t *testing.T) {
	if HashSecret("abc") != HashSecret("abc") {
		t.Fatal("hash not deterministic")
	}
	if HashSecret("abc") == HashSecret("abd") {
		t.Fatal("hash collision on distinct inputs")
	}
	tok := randToken(40)
	if len(tok) != 40 {
		t.Fatalf("token len = %d, want 40", len(tok))
	}
	if strings.Contains(tok, "_") {
		t.Fatalf("token must not contain '_': %q", tok)
	}
	if randToken(40) == tok {
		t.Fatal("tokens should be random")
	}
}

func TestDefaultActorOnKey(t *testing.T) {
	c := newCore(t) // actor "tester"
	k, err := c.CreateKey("", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if k.CreatedBy != "tester" {
		t.Fatalf("created_by = %q, want default actor 'tester'", k.CreatedBy)
	}
}
