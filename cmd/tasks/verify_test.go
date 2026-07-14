package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/api"
	"github.com/tristanMatthias/tasks/pkg/model"
)

// createTask makes a task via the CLI and returns its id.
func createTask(t *testing.T, title string) string {
	t.Helper()
	out, err := captureRun(t, "create", title, "--json")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var task model.Task
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("parse create output %q: %v", out, err)
	}
	return task.ID
}

// The happy path: add a passing command gate, verify it locally with --yes, and
// the CLI reports it verified + that the task can now be closed.
func TestCLIVerifyPasses(t *testing.T) {
	startServer(t)
	id := createTask(t, "prove it")

	if _, err := captureRun(t, "gate", "add", id, "--command", "true", "--description", "it works"); err != nil {
		t.Fatalf("gate add: %v", err)
	}

	// Before verifying, close is blocked with guidance.
	if _, err := captureRun(t, "close", id); err == nil {
		t.Fatal("expected close to be blocked by the pending gate")
	}

	out, err := captureRun(t, "verify", id, "--yes")
	if err != nil {
		t.Fatalf("verify: %v (%s)", err, out)
	}
	if !strings.Contains(out, "verified") || !strings.Contains(out, "tasks close "+id) {
		t.Fatalf("verify output missing success guidance: %q", out)
	}

	// Now close works.
	if _, err := captureRun(t, "close", id); err != nil {
		t.Fatalf("close after verify: %v", err)
	}
}

// A failing gate command must NOT verify, and the CLI returns an error.
func TestCLIVerifyFails(t *testing.T) {
	startServer(t)
	id := createTask(t, "cant prove it")
	captureRun(t, "gate", "add", id, "--command", "false")

	out, err := captureRun(t, "verify", id, "--yes")
	if err == nil {
		t.Fatalf("expected verify to fail; output: %q", out)
	}
	if !strings.Contains(out, "not verified") {
		t.Fatalf("expected 'not verified' in output: %q", out)
	}
	// Still can't close.
	if _, err := captureRun(t, "close", id); err == nil {
		t.Fatal("close should still be blocked after a failed verify")
	}
}

// `verify <id> <gate>` targets a single gate; an unknown gate id errors.
func TestCLIVerifySingleGate(t *testing.T) {
	startServer(t)
	id := createTask(t, "two gates")
	captureRun(t, "gate", "add", id, "--command", "true", "--description", "first")
	captureRun(t, "gate", "add", id, "--command", "true", "--description", "second")

	// Verify only g1.
	out, err := captureRun(t, "verify", id, "g1", "--yes")
	if err != nil {
		t.Fatalf("verify g1: %v (%s)", err, out)
	}
	if !strings.Contains(out, "gate g1 verified") {
		t.Fatalf("expected g1 verified, got: %q", out)
	}
	// g2 still pending → close blocked, and verify reports one remaining.
	if !strings.Contains(out, "1 gate(s) still pending") {
		t.Fatalf("expected one still pending, got: %q", out)
	}

	// An unknown gate id is an error.
	if _, err := captureRun(t, "verify", id, "g99", "--yes"); err == nil {
		t.Fatal("expected error verifying an unknown gate")
	}
}

// Interactive confirmation: a "y" on stdin runs the gate command.
func TestCLIVerifyConfirmYes(t *testing.T) {
	startServer(t)
	id := createTask(t, "confirm yes")
	captureRun(t, "gate", "add", id, "--command", "true")

	// Drive runVerify directly with a "y" answer on the reader.
	if err := runVerify(newClient(), &api.VerifyInput{ID: id}, strings.NewReader("y\n")); err != nil {
		t.Fatalf("runVerify with confirm: %v", err)
	}
	// It should now be closeable.
	if _, err := captureRun(t, "close", id); err != nil {
		t.Fatalf("close after confirmed verify: %v", err)
	}
}

// A verify against an unreachable server surfaces the connection error.
func TestCLIVerifyServerUnreachable(t *testing.T) {
	t.Setenv("TASKS_URL", "http://127.0.0.1:1")
	t.Setenv("TASKS_TOKEN", "x")
	if err := runVerify(newClient(), &api.VerifyInput{ID: "proj-x"}, strings.NewReader("")); err == nil {
		t.Fatal("expected an error reaching a dead server")
	}
}

// Without --yes and no interactive confirmation (EOF stdin in tests), the gate
// command is skipped and the gate stays pending.
func TestCLIVerifySkipsWithoutConfirm(t *testing.T) {
	startServer(t)
	id := createTask(t, "needs confirm")
	captureRun(t, "gate", "add", id, "--command", "true")

	out, err := captureRun(t, "verify", id)
	if err != nil {
		t.Fatalf("verify (skip) returned error: %v", err)
	}
	if !strings.Contains(out, "skipped") {
		t.Fatalf("expected a skip, got: %q", out)
	}
	if _, err := captureRun(t, "close", id); err == nil {
		t.Fatal("close should be blocked — the gate was skipped, not verified")
	}
}
