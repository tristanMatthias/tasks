package importer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/model"
)

func TestEnsureID(t *testing.T) {
	// already has id -> unchanged
	tk := &model.Task{ID: "keep"}
	ensureID(tk)
	if tk.ID != "keep" {
		t.Fatal("id changed")
	}
	// memory record -> memory:<key>
	mt := "memory"
	key := "note"
	m := &model.Task{Type: &mt, Key: &key}
	ensureID(m)
	if m.ID != "memory:note" {
		t.Fatalf("memory id = %q", m.ID)
	}
	// no type but has key -> record:<key>
	k2 := "k2"
	r := &model.Task{Key: &k2}
	ensureID(r)
	if r.ID != "record:k2" {
		t.Fatalf("record id = %q", r.ID)
	}
	// no id and no key -> stays empty
	empty := &model.Task{}
	ensureID(empty)
	if empty.ID != "" {
		t.Fatalf("empty id = %q", empty.ID)
	}
}

func TestImportBadJSON(t *testing.T) {
	st := newStore(t)
	p := filepath.Join(t.TempDir(), "bad.jsonl")
	os.WriteFile(p, []byte("{not valid json}\n"), 0o644)
	if _, err := ImportFile(st, p); err == nil {
		t.Fatal("expected parse error")
	}
}
