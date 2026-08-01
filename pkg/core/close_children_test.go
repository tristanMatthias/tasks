package core

import (
	"strings"
	"testing"
)

func TestCloseBlockedByOpenChildren(t *testing.T) {
	c := newCore(t)
	parent, _ := c.Create(CreateParams{Title: "Epic"})
	child, _ := c.Create(CreateParams{Title: "sub", Parent: parent.ID})

	// Open child blocks the parent, with a message that names it.
	_, err := c.Close(parent.ID, CloseParams{})
	if err == nil {
		t.Fatal("expected close to be blocked by an open sub-task")
	}
	for _, want := range []string{parent.ID, child.ID, "open sub-task"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %s", want, err.Error())
		}
	}

	// in_progress also blocks.
	ip := "in_progress"
	c.Update(child.ID, UpdateParams{Status: &ip})
	if _, err := c.Close(parent.ID, CloseParams{}); err == nil {
		t.Fatal("an in_progress sub-task should block close")
	}

	// Once the child is closed, the parent closes.
	if _, err := c.Close(child.ID, CloseParams{}); err != nil {
		t.Fatalf("closing the leaf child: %v", err)
	}
	if _, err := c.Close(parent.ID, CloseParams{}); err != nil {
		t.Fatalf("parent should close once children are done: %v", err)
	}
}

func TestCloseOpenChildrenErrorTruncates(t *testing.T) {
	c := newCore(t)
	parent, _ := c.Create(CreateParams{Title: "Epic"})
	for i := 0; i < 7; i++ {
		c.Create(CreateParams{Title: "sub", Parent: parent.ID})
	}
	_, err := c.Close(parent.ID, CloseParams{})
	if err == nil {
		t.Fatal("expected close to be blocked")
	}
	// 7 blockers → count is exact, list is capped with a "+N more" suffix.
	if !strings.Contains(err.Error(), "7 open sub-task(s)") || !strings.Contains(err.Error(), "+2 more") {
		t.Fatalf("expected count + truncation, got: %s", err.Error())
	}
}

func TestCloseAllowsDeferredChildren(t *testing.T) {
	c := newCore(t)
	parent, _ := c.Create(CreateParams{Title: "Epic"})
	child, _ := c.Create(CreateParams{Title: "parked", Parent: parent.ID})
	deferred := "deferred"
	c.Update(child.ID, UpdateParams{Status: &deferred})

	// A deferred (parked) child doesn't block the parent.
	if _, err := c.Close(parent.ID, CloseParams{}); err != nil {
		t.Fatalf("a deferred child should not block close: %v", err)
	}
}
