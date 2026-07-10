package core

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/tristanMatthias/tasks/internal/model"
	"github.com/tristanMatthias/tasks/internal/store"
)

func newCore(t *testing.T) *Core {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	c, err := New(st, Options{Prefix: "proj", Actor: "tester"})
	if err != nil {
		t.Fatalf("new core: %v", err)
	}
	return c
}

func TestCreateReadyClaimClose(t *testing.T) {
	c := newCore(t)

	a, err := c.Create(CreateParams{Title: "task A", Priority: intPtr(1)})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := c.Create(CreateParams{Title: "task B", Priority: intPtr(0), Deps: []string{"blocks:" + a.ID}})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	// B depends on (is blocked by) A. B has higher priority (0) but is blocked,
	// so ready should return only A.
	ready, err := c.Ready(ReadyOptions{})
	if err != nil {
		t.Fatalf("ready: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != a.ID {
		t.Fatalf("expected only A ready, got %v", ids(ready))
	}

	// Close A -> B becomes ready.
	if _, err := c.Close(a.ID, CloseParams{Reason: "done"}); err != nil {
		t.Fatalf("close A: %v", err)
	}
	ready, _ = c.Ready(ReadyOptions{})
	if len(ready) != 1 || ready[0].ID != b.ID {
		t.Fatalf("expected only B ready after closing A, got %v", ids(ready))
	}

	got, _ := c.Show(a.ID)
	if got.Status != "closed" || got.ClosedAt == "" || got.CloseReason != "done" {
		t.Fatalf("A not closed properly: %+v", got)
	}
}

func TestChildIDMinting(t *testing.T) {
	c := newCore(t)
	parent, _ := c.Create(CreateParams{Title: "epic", IssueType: "epic"})
	c1, err := c.Create(CreateParams{Title: "child1", Parent: parent.ID})
	if err != nil {
		t.Fatalf("child1: %v", err)
	}
	c2, _ := c.Create(CreateParams{Title: "child2", Parent: parent.ID})
	if c1.ID != parent.ID+".1" || c2.ID != parent.ID+".2" {
		t.Fatalf("child ids wrong: %s %s (parent %s)", c1.ID, c2.ID, parent.ID)
	}
	// child must have a parent-child dep to parent
	if len(c1.Dependencies) != 1 || c1.Dependencies[0].Type != "parent-child" {
		t.Fatalf("child1 missing parent-child dep: %+v", c1.Dependencies)
	}
}

// TestConcurrentClaim verifies exactly one of many parallel claims wins.
func TestConcurrentClaim(t *testing.T) {
	c := newCore(t)
	task, _ := c.Create(CreateParams{Title: "contended"})

	const n = 12
	var wg sync.WaitGroup
	wins := make([]bool, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_, err := c.Update(task.ID, UpdateParams{Claim: true, Actor: "agent-" + string(rune('a'+i))})
			wins[i] = err == nil
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
		t.Fatalf("expected exactly 1 winning claim, got %d", won)
	}
	got, _ := c.Show(task.ID)
	if got.Status != "in_progress" || got.Assignee == "" {
		t.Fatalf("claimed task state wrong: status=%s assignee=%q", got.Status, got.Assignee)
	}
}

func ids(ts []model.Task) []string {
	out := make([]string, len(ts))
	for i := range ts {
		out[i] = ts[i].ID
	}
	return out
}
