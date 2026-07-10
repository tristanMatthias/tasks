package core

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/store"
)

func TestReadyUnknownBlockerSatisfied(t *testing.T) {
	c := newCore(t)
	// Dep on a nonexistent blocker id -> treated as satisfied (not blocking).
	tk, _ := c.Create(CreateParams{Title: "x", Deps: []string{"blocks:ghost"}})
	got, _ := c.Ready(ReadyOptions{Limit: 50})
	found := false
	for _, r := range got {
		if r.ID == tk.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("task with unknown blocker should be ready")
	}
}

func TestReadyErrorOnClosedStore(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(st, Options{Prefix: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	if _, err := c.Ready(ReadyOptions{}); err == nil {
		t.Fatal("expected Ready error on closed store")
	}
	if _, err := c.List(store.Filter{}); err == nil {
		t.Fatal("expected List error on closed store")
	}
	if _, err := c.All(); err == nil {
		t.Fatal("expected All error on closed store")
	}
}

func TestNewErrorOnClosedStore(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	if _, err := New(st, Options{}); err == nil {
		t.Fatal("expected New error when meta read fails on closed store")
	}
}

func TestNextChildIndexGaps(t *testing.T) {
	// grandchildren and non-matching ids are ignored; max+1 wins.
	got := nextChildIndex("p.1", []string{"p.1.1", "p.1.3", "p.1.3.9", "other", "p.10"})
	if got != 4 {
		t.Fatalf("nextChildIndex = %d, want 4", got)
	}
	if nextChildIndex("p.1", nil) != 1 {
		t.Fatal("empty children -> 1")
	}
}

func TestSearchFuzzy(t *testing.T) {
	c := newCore(t)
	a, _ := c.Create(CreateParams{Title: "parallel test runner"})
	c.Create(CreateParams{Title: "unrelated widget factory"})

	// Misspelled query still ranks the intended task first (fuzzy).
	res, err := c.Search("parllel tst", SearchOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 || res[0].ID != a.ID {
		t.Fatalf("fuzzy search wrong top hit: %v", ids(res))
	}
	// Empty query returns the (filtered) set.
	all, _ := c.Search("", SearchOptions{})
	if len(all) != 2 {
		t.Fatalf("empty query = %d, want 2", len(all))
	}
	// Limit is honored.
	if lim, _ := c.Search("", SearchOptions{Limit: 1}); len(lim) != 1 {
		t.Fatalf("limit not applied")
	}
	// Status filter applies before fuzzy.
	c.Close(a.ID, CloseParams{})
	open, _ := c.Search("", SearchOptions{Statuses: []string{"open"}})
	if len(open) != 1 {
		t.Fatalf("status filter = %d, want 1", len(open))
	}
	// Type filter.
	if byType, _ := c.Search("", SearchOptions{Type: "task"}); len(byType) != 2 {
		t.Fatalf("type filter = %d", len(byType))
	}
}

func TestTree(t *testing.T) {
	c := newCore(t)
	p, _ := c.Create(CreateParams{Title: "epic", IssueType: "epic"})
	ch, _ := c.Create(CreateParams{Title: "child", Parent: p.ID})
	gc, _ := c.Create(CreateParams{Title: "grandchild", Parent: ch.ID})

	sub, err := c.Tree(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, x := range sub {
		got[x.ID] = true
	}
	if len(sub) != 3 || !got[p.ID] || !got[ch.ID] || !got[gc.ID] {
		t.Fatalf("subtree wrong: %v", got)
	}
	// Leaf subtree is just itself.
	if leaf, _ := c.Tree(gc.ID); len(leaf) != 1 {
		t.Fatalf("leaf subtree = %d, want 1", len(leaf))
	}
	// Missing root -> not found.
	if _, err := c.Tree("nope"); err == nil {
		t.Fatal("expected not-found for missing root")
	}
}

func TestShortID(t *testing.T) {
	if shortID("proj-abcd") != "abcd" {
		t.Error("shortID split")
	}
	if shortID("noprefix") != "noprefix" {
		t.Error("shortID no dash")
	}
}

func TestResolveShortID(t *testing.T) {
	c := newCore(t) // prefix "proj"
	full, _ := c.Create(CreateParams{Title: "x"})
	short := full.ID[len("proj-"):] // e.g. "abcd"

	// Reads and writes accept the short id.
	if got, err := c.Show(short); err != nil || got.ID != full.ID {
		t.Fatalf("show short: %v %v", got, err)
	}
	if _, err := c.Comment(short, "hi", ""); err != nil {
		t.Fatalf("comment short: %v", err)
	}
	if _, err := c.Update(short, UpdateParams{Claim: true}); err != nil {
		t.Fatalf("update short: %v", err)
	}
	// dep by short ids
	other, _ := c.Create(CreateParams{Title: "y"})
	if err := c.AddDep(short, other.ID[len("proj-"):], "", ""); err != nil {
		t.Fatalf("dep short: %v", err)
	}
	if _, err := c.Close(short, CloseParams{}); err != nil {
		t.Fatalf("close short: %v", err)
	}
	// Full id still works; unknown still errors.
	if _, err := c.Show(full.ID); err != nil {
		t.Fatal("full id should resolve")
	}
	if _, err := c.Show("nope"); err == nil {
		t.Fatal("unknown short id should error")
	}
	// child (dotted) short id
	ch, _ := c.Create(CreateParams{Title: "child", Parent: short})
	if !strings.HasPrefix(ch.ID, full.ID+".") {
		t.Fatalf("short-parent child id = %s", ch.ID)
	}
	if got, _ := c.Show(ch.ID[len("proj-"):]); got == nil || got.ID != ch.ID {
		t.Fatalf("resolve dotted short id failed")
	}
}

func TestResolveIDEmptyPrefix(t *testing.T) {
	c := newCore(t)
	c.prefix = ""
	if c.ResolveID("x") != "x" {
		t.Fatal("empty prefix should pass through")
	}
}

func TestTreeAll(t *testing.T) {
	c := newCore(t)
	c.Create(CreateParams{Title: "a"})
	p, _ := c.Create(CreateParams{Title: "b"})
	c.Create(CreateParams{Title: "child", Parent: p.ID})
	all, err := c.Tree("") // whole forest
	if err != nil || len(all) != 3 {
		t.Fatalf("tree-all = %d (%v), want 3", len(all), err)
	}
}
