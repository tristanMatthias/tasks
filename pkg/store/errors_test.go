package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/model"
)

// TestOpenBadPath covers the Open error branch (unopenable database path).
func TestOpenBadPath(t *testing.T) {
	// A path whose parent is a file, not a directory, cannot be created.
	bad := filepath.Join(t.TempDir(), "afile")
	if err := writeFile(bad, "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(filepath.Join(bad, "nested.db")); err == nil {
		t.Fatal("expected open error for bad path")
	}
}

// TestErrorsAfterClose exercises every method's error branch by closing the DB
// underneath it (fault injection without a mock).
func TestErrorsAfterClose(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Seed one row so hydrate/get have something before we break the DB.
	must(t, st.Insert(mk("proj-a", "open", "task", p(2))))
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	task := mk("proj-x", "open", "task", p(1))
	task.Comments = []model.Comment{{ID: "c", IssueID: "proj-x", Text: "t"}}
	task.Dependencies = []model.Dependency{{IssueID: "proj-x", DependsOnID: "y", Type: "blocks"}}

	checks := []func() error{
		func() error { _, e := st.Count(); return e },
		func() error { _, e := st.Get("proj-a"); return e },
		func() error { _, e := st.All(); return e },
		func() error { _, e := st.List(Filter{}); return e },
		func() error { return st.Insert(task) },
		func() error { return st.Upsert(task) },
		func() error { return st.Patch("proj-a", map[string]any{"title": "z"}, "t") },
		func() error { _, e := st.Claim("proj-a", "me", "s", "u"); return e },
		func() error { return st.AddComment(model.Comment{ID: "c2", IssueID: "proj-a"}) },
		func() error {
			return st.AddDependency(model.Dependency{IssueID: "proj-a", DependsOnID: "z", Type: "blocks"})
		},
		func() error { _, e := st.Meta("k"); return e },
		func() error { return st.SetMeta("k", "v") },
		func() error { _, e := st.Exists("proj-a"); return e },
	}
	for i, fn := range checks {
		if err := fn(); err == nil {
			t.Errorf("check %d: expected error after close", i)
		}
	}
}

func TestPlaceholders(t *testing.T) {
	if placeholders(0) != "" {
		t.Error("placeholders(0)")
	}
	if placeholders(1) != "?" {
		t.Error("placeholders(1)")
	}
	if placeholders(3) != "?,?,?" {
		t.Errorf("placeholders(3) = %q", placeholders(3))
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
