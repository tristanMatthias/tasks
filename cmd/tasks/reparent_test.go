package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/model"
)

func parentOfTask(t *testing.T, id string) string {
	t.Helper()
	out, err := captureRun(t, "show", id, "--json")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	var task model.Task
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("parse show %q: %v", out, err)
	}
	for _, d := range task.Dependencies {
		if d.Type == "parent-child" || d.Type == "parent" {
			return d.DependsOnID
		}
	}
	return ""
}

// The agent-facing path: `tasks update <id> --parent <new>` reparents, and
// `--parent ""` detaches to a root.
func TestCLIReparent(t *testing.T) {
	startServer(t)
	a := createTask(t, "A")
	b := createTask(t, "B")

	out, err := captureRun(t, "create", "child", "--parent", a, "--json")
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	var child model.Task
	json.Unmarshal([]byte(out), &child)
	if parentOfTask(t, child.ID) != a {
		t.Fatal("child should start under A")
	}

	// Move under B.
	if _, err := captureRun(t, "update", child.ID, "--parent", b); err != nil {
		t.Fatalf("reparent: %v", err)
	}
	if got := parentOfTask(t, child.ID); got != b {
		t.Fatalf("parent after move = %q, want %q", got, b)
	}

	// Detach to a root with an empty parent.
	if _, err := captureRun(t, "update", child.ID, "--parent", ""); err != nil {
		t.Fatalf("detach: %v", err)
	}
	if got := parentOfTask(t, child.ID); got != "" {
		t.Fatalf("child should be a root, parent = %q", got)
	}

	// A cycle is rejected with a helpful error.
	_, err = captureRun(t, "update", a, "--parent", child.ID)
	_ = err // moving A under the now-detached child is fine; just exercise the flag
	if _, err := captureRun(t, "update", child.ID, "--parent", child.ID); err == nil || !strings.Contains(err.Error(), "own parent") {
		t.Fatalf("self-parent should error, got %v", err)
	}
}
