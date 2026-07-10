package store

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/model"
)

func open(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func p(n int) *int { return &n }

func mk(id, status, itype string, prio *int) *model.Task {
	return &model.Task{ID: id, Title: id, Status: status, IssueType: itype, Priority: prio, CreatedAt: "2026-01-01T00:00:00Z"}
}

func TestInsertGetAndCounts(t *testing.T) {
	st := open(t)
	a := mk("proj-a", "open", "task", p(2))
	a.Labels = []string{"x", "y"}
	a.Comments = []model.Comment{{ID: "c1", IssueID: "proj-a", Author: "me", Text: "hi", CreatedAt: "2026-01-01T00:00:00Z"}}
	if err := st.Insert(a); err != nil {
		t.Fatal(err)
	}
	b := mk("proj-b", "open", "task", p(1))
	b.Dependencies = []model.Dependency{{IssueID: "proj-b", DependsOnID: "proj-a", Type: "blocks", Metadata: "{}"}}
	if err := st.Insert(b); err != nil {
		t.Fatal(err)
	}

	got, err := st.Get("proj-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Labels) != 2 || got.CommentCount != 1 || got.DependentCount != 1 {
		t.Fatalf("proj-a wrong: labels=%v comments=%d dependents=%d", got.Labels, got.CommentCount, got.DependentCount)
	}
	gb, _ := st.Get("proj-b")
	if gb.DependencyCount != 1 || gb.Dependencies[0].Type != "blocks" {
		t.Fatalf("proj-b deps wrong: %+v", gb.Dependencies)
	}

	if _, err := st.Get("missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	n, _ := st.Count()
	if n != 2 {
		t.Fatalf("count = %d", n)
	}
	if ok, _ := st.Exists("proj-a"); !ok {
		t.Error("Exists(proj-a) false")
	}
	if ok, _ := st.Exists("nope"); ok {
		t.Error("Exists(nope) true")
	}
}

func TestUpsertReplacesDepsAndComments(t *testing.T) {
	st := open(t)
	a := mk("proj-a", "open", "task", p(2))
	a.Comments = []model.Comment{{ID: "c1", IssueID: "proj-a", Text: "one"}}
	a.Dependencies = []model.Dependency{{IssueID: "proj-a", DependsOnID: "x", Type: "blocks"}}
	if err := st.Upsert(a); err != nil {
		t.Fatal(err)
	}
	// Upsert again with different deps/comments — must replace, not accumulate.
	a.Comments = []model.Comment{{ID: "c2", IssueID: "proj-a", Text: "two"}}
	a.Dependencies = nil
	a.Title = "updated"
	if err := st.Upsert(a); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("proj-a")
	if got.Title != "updated" || got.CommentCount != 1 || got.Comments[0].Text != "two" || got.DependencyCount != 0 {
		t.Fatalf("upsert did not replace: %+v", got)
	}
}

func TestListFilters(t *testing.T) {
	st := open(t)
	must(t, st.Insert(withLabels(mk("proj-1", "open", "task", p(2)), "backend")))
	must(t, st.Insert(mk("proj-2", "closed", "bug", p(0))))
	must(t, st.Insert(withAssignee(mk("proj-3", "open", "bug", p(1)), "alice")))
	// parent-child so proj-4 is a child of proj-1
	child := mk("proj-1.1", "open", "task", p(2))
	child.Dependencies = []model.Dependency{{IssueID: "proj-1.1", DependsOnID: "proj-1", Type: "parent-child"}}
	must(t, st.Insert(child))

	if got := listIDs(t, st, Filter{Statuses: []string{"open"}}); len(got) != 3 {
		t.Fatalf("open filter = %v", got)
	}
	if got := listIDs(t, st, Filter{Types: []string{"bug"}}); len(got) != 2 {
		t.Fatalf("bug filter = %v", got)
	}
	if got := listIDs(t, st, Filter{Assignee: "alice"}); len(got) != 1 || got[0] != "proj-3" {
		t.Fatalf("assignee filter = %v", got)
	}
	if got := listIDs(t, st, Filter{Priority: p(0)}); len(got) != 1 || got[0] != "proj-2" {
		t.Fatalf("priority filter = %v", got)
	}
	if got := listIDs(t, st, Filter{Labels: []string{"backend"}}); len(got) != 1 || got[0] != "proj-1" {
		t.Fatalf("label filter = %v", got)
	}
	if got := listIDs(t, st, Filter{Parent: "proj-1"}); len(got) != 1 || got[0] != "proj-1.1" {
		t.Fatalf("parent filter = %v", got)
	}
	if got := listIDs(t, st, Filter{Limit: 2}); len(got) != 2 {
		t.Fatalf("limit = %v", got)
	}
	if got := listIDs(t, st, Filter{OrderByPriority: true}); got[0] != "proj-2" {
		t.Fatalf("priority order first = %v", got)
	}
	all, _ := st.All()
	if len(all) != 4 {
		t.Fatalf("All = %d", len(all))
	}
}

func TestPatchAndClaim(t *testing.T) {
	st := open(t)
	must(t, st.Insert(mk("proj-a", "open", "task", p(2))))

	if err := st.Patch("proj-a", map[string]any{"status": "in_progress", "title": "new"}, "2026-02-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get("proj-a")
	if got.Status != "in_progress" || got.Title != "new" || got.UpdatedAt != "2026-02-01T00:00:00Z" {
		t.Fatalf("patch wrong: %+v", got)
	}
	// unknown column rejected
	if err := st.Patch("proj-a", map[string]any{"bogus": 1}, "t"); err == nil {
		t.Fatal("expected error for bad column")
	}
	// patch missing id
	if err := st.Patch("nope", map[string]any{"title": "x"}, "t"); err != ErrNotFound {
		t.Fatalf("patch missing = %v", err)
	}
	// empty patch is a no-op
	if err := st.Patch("proj-a", nil, "t"); err != nil {
		t.Fatalf("empty patch: %v", err)
	}

	// Claim: first wins, idempotent for same actor, blocked for other.
	ok, err := st.Claim("proj-a", "alice", "s", "u")
	if err != nil || !ok {
		t.Fatalf("first claim: ok=%v err=%v", ok, err)
	}
	ok, _ = st.Claim("proj-a", "alice", "s", "u") // idempotent
	if !ok {
		t.Fatal("re-claim by same actor should succeed")
	}
	ok, _ = st.Claim("proj-a", "bob", "s", "u")
	if ok {
		t.Fatal("claim by other actor should fail")
	}
	if _, err := st.Claim("missing", "x", "s", "u"); err != ErrNotFound {
		t.Fatalf("claim missing = %v", err)
	}
}

func TestMetaAndAddHelpers(t *testing.T) {
	st := open(t)
	if v, _ := st.Meta("prefix"); v != "" {
		t.Fatal("unset meta should be empty")
	}
	must(t, st.SetMeta("prefix", "proj"))
	if v, _ := st.Meta("prefix"); v != "proj" {
		t.Fatalf("meta = %q", v)
	}
	must(t, st.SetMeta("prefix", "proj2")) // upsert
	if v, _ := st.Meta("prefix"); v != "proj2" {
		t.Fatalf("meta upsert = %q", v)
	}
	must(t, st.Insert(mk("proj-a", "open", "task", p(2))))
	must(t, st.AddComment(model.Comment{ID: "c9", IssueID: "proj-a", Text: "note"}))
	must(t, st.AddDependency(model.Dependency{IssueID: "proj-a", DependsOnID: "x", Type: "related"}))
	got, _ := st.Get("proj-a")
	if got.CommentCount != 1 || got.DependencyCount != 1 {
		t.Fatalf("add helpers: %+v", got)
	}
}

// TestConcurrentClaimStore hammers Claim from many goroutines: exactly one wins.
func TestConcurrentClaimStore(t *testing.T) {
	st := open(t)
	must(t, st.Insert(mk("proj-a", "open", "task", p(2))))
	const n = 16
	var wg sync.WaitGroup
	wins := make([]bool, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			ok, _ := st.Claim("proj-a", "agent-"+string(rune('a'+i)), "s", "u")
			wins[i] = ok
		}(i)
	}
	wg.Wait()
	won := 0
	for _, w := range wins {
		if w {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", won)
	}
}

// ---- helpers ----

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func withLabels(t *model.Task, l ...string) *model.Task { t.Labels = l; return t }
func withAssignee(t *model.Task, a string) *model.Task  { t.Assignee = a; return t }
func listIDs(t *testing.T, st *Store, f Filter) []string {
	t.Helper()
	ts, err := st.List(f)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(ts))
	for i := range ts {
		ids[i] = ts[i].ID
	}
	return ids
}
