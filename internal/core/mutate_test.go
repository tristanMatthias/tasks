package core

import (
	"path/filepath"
	"testing"

	"github.com/tristanMatthias/tasks/internal/model"
	"github.com/tristanMatthias/tasks/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func sp(s string) *string { return &s }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewDerivesPrefixAndActor(t *testing.T) {
	st := openStore(t)
	// Seed a task so prefix can be derived from data.
	must(t, st.Insert(&model.Task{ID: "derived-abcd", Title: "x", Status: "open", IssueType: "task"}))
	c, err := New(st, Options{}) // no prefix, no actor
	if err != nil {
		t.Fatal(err)
	}
	if c.Prefix() != "derived" {
		t.Fatalf("derived prefix = %q", c.Prefix())
	}
	if c.Actor() != "agent" {
		t.Fatalf("default actor = %q", c.Actor())
	}
	// prefix now persisted in meta; a second New reads it back.
	c2, _ := New(st, Options{})
	if c2.Prefix() != "derived" {
		t.Fatalf("meta prefix = %q", c2.Prefix())
	}
	if c.Store() == nil {
		t.Fatal("Store() nil")
	}
}

func TestCreateVariants(t *testing.T) {
	c := newCore(t)
	// explicit id + deps (type:id and bare)
	other, _ := c.Create(CreateParams{Title: "other"})
	tk, err := c.Create(CreateParams{
		Title: "explicit", ID: "proj-xyz", Deps: []string{"blocks:" + other.ID, "related:" + other.ID},
		Design: "d", AcceptanceCriteria: "ac", Notes: "n", Labels: []string{"l1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tk.ID != "proj-xyz" || tk.Design != "d" || len(tk.Dependencies) != 2 {
		t.Fatalf("explicit create wrong: %+v", tk)
	}
	// parent not found
	if _, err := c.Create(CreateParams{Title: "orphan", Parent: "missing"}); err == nil {
		t.Fatal("expected parent-not-found error")
	}
}

func TestUpdateAllFields(t *testing.T) {
	c := newCore(t)
	tk, _ := c.Create(CreateParams{Title: "orig", Labels: []string{"keep", "drop"}})
	got, err := c.Update(tk.ID, UpdateParams{
		Title: sp("new title"), Description: sp("desc"), Status: sp("closed"),
		Priority: intPtr(0), IssueType: sp("bug"), Assignee: sp("bob"),
		Design: sp("dz"), AcceptanceCriteria: sp("acz"), Notes: sp("nz"),
		AddLabels: []string{"added"}, RemoveLabels: []string{"drop"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "new title" || got.Status != "closed" || got.ClosedAt == "" ||
		got.PriorityOr(-1) != 0 || got.IssueType != "bug" || got.Assignee != "bob" {
		t.Fatalf("update fields wrong: %+v", got)
	}
	labelSet := map[string]bool{}
	for _, l := range got.Labels {
		labelSet[l] = true
	}
	if !labelSet["keep"] || !labelSet["added"] || labelSet["drop"] {
		t.Fatalf("labels wrong: %v", got.Labels)
	}

	// SetLabels replaces
	got, _ = c.Update(tk.ID, UpdateParams{SetLabels: &[]string{"only"}})
	if len(got.Labels) != 1 || got.Labels[0] != "only" {
		t.Fatalf("set labels: %v", got.Labels)
	}
	// append notes
	got, _ = c.Update(tk.ID, UpdateParams{AppendNotes: "more"})
	if got.Notes != "nz\nmore" {
		t.Fatalf("append notes: %q", got.Notes)
	}
	// no-op update returns the task
	if _, err := c.Update(tk.ID, UpdateParams{}); err != nil {
		t.Fatalf("noop update: %v", err)
	}
}

func TestUpdateClaimConflict(t *testing.T) {
	c := newCore(t)
	tk, _ := c.Create(CreateParams{Title: "x"})
	if _, err := c.Update(tk.ID, UpdateParams{Claim: true, Actor: "alice"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Update(tk.ID, UpdateParams{Claim: true, Actor: "bob"}); err == nil {
		t.Fatal("expected claim conflict error")
	}
	// claim missing id
	if _, err := c.Update("nope", UpdateParams{Claim: true}); err == nil {
		t.Fatal("expected claim missing error")
	}
}

func TestCommentAndNote(t *testing.T) {
	c := newCore(t)
	tk, _ := c.Create(CreateParams{Title: "x"})
	got, err := c.Comment(tk.ID, "a comment", "author1")
	if err != nil || got.CommentCount != 1 || got.Comments[0].Author != "author1" {
		t.Fatalf("comment: %+v %v", got, err)
	}
	if _, err := c.Comment("missing", "x", ""); err == nil {
		t.Fatal("expected comment missing error")
	}
	got, _ = c.Note(tk.ID, "note text", "")
	if got.Notes != "note text" {
		t.Fatalf("note: %q", got.Notes)
	}
}

func TestAddDepErrors(t *testing.T) {
	c := newCore(t)
	a, _ := c.Create(CreateParams{Title: "a"})
	b, _ := c.Create(CreateParams{Title: "b"})
	if err := c.AddDep(b.ID, a.ID, "", ""); err != nil { // default type blocks
		t.Fatal(err)
	}
	gb, _ := c.Show(b.ID)
	if gb.Dependencies[0].Type != "blocks" {
		t.Fatalf("default dep type = %s", gb.Dependencies[0].Type)
	}
	if err := c.AddDep("missing", a.ID, "blocks", ""); err == nil {
		t.Fatal("expected missing blocked error")
	}
	if err := c.AddDep(b.ID, "missing", "related", ""); err == nil {
		t.Fatal("expected missing blocker error")
	}
}

func TestCloseMissing(t *testing.T) {
	c := newCore(t)
	if _, err := c.Close("nope", CloseParams{}); err == nil {
		t.Fatal("expected close-missing error")
	}
}

func TestListAndAll(t *testing.T) {
	c := newCore(t)
	c.Create(CreateParams{Title: "a", IssueType: "bug"})
	c.Create(CreateParams{Title: "b", IssueType: "task"})
	if got, _ := c.List(store.Filter{Types: []string{"bug"}}); len(got) != 1 {
		t.Fatalf("list bug = %d", len(got))
	}
	if got, _ := c.All(); len(got) != 2 {
		t.Fatalf("all = %d", len(got))
	}
}

func TestReadyFilters(t *testing.T) {
	c := newCore(t)
	c.Create(CreateParams{Title: "p0", Priority: intPtr(0), IssueType: "bug"})
	c.Create(CreateParams{Title: "p2", Priority: intPtr(2), IssueType: "task", Assignee: "me"})
	// type filter
	if got, _ := c.Ready(ReadyOptions{Type: "bug"}); len(got) != 1 || got[0].IssueType != "bug" {
		t.Fatalf("ready type filter: %v", got)
	}
	// priority filter
	if got, _ := c.Ready(ReadyOptions{Priority: intPtr(2)}); len(got) != 1 {
		t.Fatalf("ready prio filter: %v", got)
	}
	// limit
	if got, _ := c.Ready(ReadyOptions{Limit: 1}); len(got) != 1 {
		t.Fatalf("ready limit: %v", got)
	}
}

func TestFreshIDNoPrefix(t *testing.T) {
	st := openStore(t) // empty store, no derivable prefix
	c, _ := New(st, Options{})
	if c.Prefix() != "" {
		t.Fatalf("expected empty prefix, got %q", c.Prefix())
	}
	if _, err := c.Create(CreateParams{Title: "x"}); err == nil {
		t.Fatal("expected error creating without a prefix")
	}
}

func TestParseDepSpec(t *testing.T) {
	if ty, id := parseDepSpec("blocks:proj-1"); ty != "blocks" || id != "proj-1" {
		t.Fatalf("typed spec: %s %s", ty, id)
	}
	if ty, id := parseDepSpec("proj-2"); ty != "blocks" || id != "proj-2" {
		t.Fatalf("bare spec: %s %s", ty, id)
	}
}

func TestLabelHelpersAndSplit(t *testing.T) {
	got := addLabels([]string{"a"}, []string{"a", "b"})
	if len(got) != 2 {
		t.Fatalf("addLabels dedup: %v", got)
	}
	got = removeLabels([]string{"a", "b", "c"}, []string{"b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Fatalf("removeLabels: %v", got)
	}
	if removeLabels([]string{"a"}, nil) == nil {
		t.Fatal("removeLabels nil rm should return input")
	}
	if len(splitCSV(" a , ,b ")) != 2 {
		t.Fatalf("splitCSV: %v", splitCSV(" a , ,b "))
	}
	if wrap("op", nil) != nil {
		t.Fatal("wrap nil should be nil")
	}
	if wrap("op", ErrNotFound) != ErrNotFound {
		t.Fatal("wrap should pass through ErrNotFound")
	}
}

func TestDerivePrefixDirect(t *testing.T) {
	if got := DerivePrefix([]string{"memory:x", "proj-abc"}); got != "proj" {
		t.Fatalf("DerivePrefix = %q", got)
	}
	if got := DerivePrefix([]string{"noprefix"}); got != "" {
		t.Fatalf("DerivePrefix no-dash = %q", got)
	}
}

func TestRandSuffix(t *testing.T) {
	s := randSuffix(4)
	if len(s) != 4 {
		t.Fatalf("randSuffix len = %d", len(s))
	}
}
