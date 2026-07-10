package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/tristanMatthias/tasks/internal/model"
)

// capture returns whatever fn writes to os.Stdout.
func capture(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestPrintTaskDetailFull(t *testing.T) {
	pr := 1
	tk := &model.Task{
		ID: "proj-a", Title: "Full", Status: "closed", Priority: &pr, IssueType: "bug",
		Assignee: "bob", Labels: []string{"x", "y"}, Description: "the description",
		Dependencies: []model.Dependency{{Type: "blocks", DependsOnID: "proj-b"}},
		Comments:     []model.Comment{{Author: "me", Text: "hi"}},
	}
	tk.Owner = "alice"
	tk.CreatedAt = "2026-06-13T10:00:00Z"
	tk.ClosedAt = "2026-07-01T12:34:00Z"
	tk.CloseReason = "fixed it"
	tk.Notes = "some notes"
	tk.AcceptanceCriteria = "must pass"
	tk.Comments[0].CreatedAt = "2026-07-01T12:30:00Z"
	out := capture(func() { printTaskDetail(tk) })
	for _, want := range []string{
		"✓", "proj-a", "[BUG]", "Full", "P1", "CLOSED", // header
		"Owner: alice", "Assignee: bob", "Created: 2026-06-13", "Closed: 2026-07-01",
		"Labels: x, y", "Close reason: fixed it",
		"DESCRIPTION", "the description", "ACCEPTANCE CRITERIA", "must pass", "NOTES", "some notes",
		"DEPENDENCIES", "blocked by  proj-b",
		"COMMENTS (1)", "2026-07-01 12:30", "me", "hi",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q in:\n%s", want, out)
		}
	}
}

func TestPrintTasksEmptyAndPrio(t *testing.T) {
	if out := capture(func() { printTasks(nil) }); !strings.Contains(out, "no tasks") {
		t.Errorf("empty print: %q", out)
	}
	// nil priority -> "?"
	tk := model.Task{ID: "proj-a", Title: "x", Status: "open", IssueType: "task"}
	if prioStr(&tk) != "?" {
		t.Error("nil priority should be ?")
	}
	p := 2
	tk.Priority = &p
	if prioStr(&tk) != "2" {
		t.Error("priority 2")
	}
	out := capture(func() { printTasks([]model.Task{tk}) })
	if !strings.Contains(out, "proj-a") {
		t.Errorf("printTasks: %q", out)
	}
}

func TestPrintJSONBytes(t *testing.T) {
	// valid json is pretty-printed
	if out := capture(func() { printJSONBytes([]byte(`{"a":1}`)) }); !strings.Contains(out, "\"a\": 1") {
		t.Errorf("pretty json: %q", out)
	}
	// invalid json falls back to raw passthrough
	if out := capture(func() { printJSONBytes([]byte(`not json`)) }); !strings.Contains(out, "not json") {
		t.Errorf("raw fallback: %q", out)
	}
}
