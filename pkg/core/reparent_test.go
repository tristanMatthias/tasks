package core

import (
	"testing"
)

// parentOf returns a task's containment parent id (via its parent-child edge),
// or "" if it's a root.
func parentOf(t *testing.T, c *Core, id string) string {
	t.Helper()
	tk, err := c.Show(id)
	must(t, err)
	for _, d := range tk.Dependencies {
		if d.Type == "parent-child" || d.Type == "parent" {
			return d.DependsOnID
		}
	}
	return ""
}

func TestReparentMovesTask(t *testing.T) {
	c := newCore(t)
	a, _ := c.Create(CreateParams{Title: "A"})
	b, _ := c.Create(CreateParams{Title: "B"})
	child, _ := c.Create(CreateParams{Title: "child", Parent: a.ID})
	if parentOf(t, c, child.ID) != a.ID {
		t.Fatalf("precondition: child should be under A")
	}

	// Move child from A to B.
	np := b.ID
	if _, err := c.Update(child.ID, UpdateParams{Parent: &np}); err != nil {
		t.Fatalf("reparent: %v", err)
	}
	if got := parentOf(t, c, child.ID); got != b.ID {
		t.Fatalf("parent after reparent = %q, want %q", got, b.ID)
	}
	// The id is unchanged (permanent reference).
	if _, err := c.Show(child.ID); err != nil {
		t.Fatalf("child id should be unchanged: %v", err)
	}
}

func TestReparentDetachesToRoot(t *testing.T) {
	c := newCore(t)
	a, _ := c.Create(CreateParams{Title: "A"})
	child, _ := c.Create(CreateParams{Title: "child", Parent: a.ID})

	empty := ""
	if _, err := c.Update(child.ID, UpdateParams{Parent: &empty}); err != nil {
		t.Fatalf("detach: %v", err)
	}
	if got := parentOf(t, c, child.ID); got != "" {
		t.Fatalf("child should be a root after detach, parent = %q", got)
	}
}

func TestReparentRejectsCyclesAndBadParents(t *testing.T) {
	c := newCore(t)
	a, _ := c.Create(CreateParams{Title: "A"})
	child, _ := c.Create(CreateParams{Title: "child", Parent: a.ID})
	grand, _ := c.Create(CreateParams{Title: "grand", Parent: child.ID})

	// Under itself.
	self := child.ID
	if _, err := c.Update(child.ID, UpdateParams{Parent: &self}); err == nil {
		t.Fatal("expected error parenting a task under itself")
	}
	// Under its own descendant (cycle).
	g := grand.ID
	if _, err := c.Update(child.ID, UpdateParams{Parent: &g}); err == nil {
		t.Fatal("expected error parenting under a descendant")
	}
	// Under a non-existent parent.
	nope := "proj-nope"
	if _, err := c.Update(child.ID, UpdateParams{Parent: &nope}); err == nil {
		t.Fatal("expected error for a missing parent")
	}
	// The child is untouched by the failed attempts.
	if got := parentOf(t, c, child.ID); got != a.ID {
		t.Fatalf("child parent should still be A, got %q", got)
	}
}

func TestReparentMissingTask(t *testing.T) {
	c := newCore(t)
	a, _ := c.Create(CreateParams{Title: "A"})
	p := a.ID
	if _, err := c.Update("proj-ghost", UpdateParams{Parent: &p}); err == nil {
		t.Fatal("expected error reparenting a non-existent task")
	}
}
