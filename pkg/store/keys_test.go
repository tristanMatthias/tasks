package store

import (
	"errors"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/model"
)

func TestStoreKeys(t *testing.T) {
	st := open(t)

	k := model.APIKey{ID: "k1", Hash: "h1", Label: "a", CreatedBy: "u", CreatedAt: "2026-01-01T00:00:00Z"}
	if err := st.InsertKey(k); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Duplicate hash violates the UNIQUE constraint.
	if err := st.InsertKey(model.APIKey{ID: "k2", Hash: "h1"}); err == nil {
		t.Fatal("expected unique-hash violation")
	}

	got, err := st.KeyByHash("h1")
	if err != nil || got.ID != "k1" {
		t.Fatalf("by hash = %v, %v", got, err)
	}
	if _, err := st.KeyByHash("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("by hash missing = %v, want ErrNotFound", err)
	}
	if _, err := st.KeyByID("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("by id missing = %v, want ErrNotFound", err)
	}

	if err := st.TouchKey("k1", "2026-02-02T00:00:00Z"); err != nil {
		t.Fatalf("touch: %v", err)
	}
	got, _ = st.KeyByID("k1")
	if got.LastUsedAt != "2026-02-02T00:00:00Z" {
		t.Fatalf("touch not persisted: %q", got.LastUsedAt)
	}

	// Revoke is idempotent/observable.
	ok, err := st.RevokeKey("k1", "2026-03-03T00:00:00Z")
	if err != nil || !ok {
		t.Fatalf("revoke = %v, %v", ok, err)
	}
	if ok, _ := st.RevokeKey("k1", "later"); ok {
		t.Fatal("second revoke should report no change")
	}
	if ok, _ := st.RevokeKey("ghost", "x"); ok {
		t.Fatal("revoking unknown id should report no change")
	}

	list, err := st.ListKeys()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v (err %v)", list, err)
	}
	if list[0].Hash != "" {
		t.Fatalf("ListKeys must not select the hash: %q", list[0].Hash)
	}
	if list[0].RevokedAt == "" {
		t.Fatal("revoked_at should be set")
	}
}
