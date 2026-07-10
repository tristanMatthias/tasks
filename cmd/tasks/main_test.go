package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/core"
	"github.com/tristanMatthias/tasks/pkg/httpapi"
	"github.com/tristanMatthias/tasks/pkg/model"
	"github.com/tristanMatthias/tasks/pkg/store"
	"github.com/tristanMatthias/tasks/web"
)

// startServer spins up a real tasksd HTTP server and points the CLI at it.
func startServer(t *testing.T) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	c, err := core.New(st, core.Options{Prefix: "proj", Actor: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httpapi.New(httpapi.Config{Core: c, Static: web.Static(), Token: "tok"})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	t.Setenv("TASKS_URL", ts.URL)
	t.Setenv("TASKS_TOKEN", "tok")
}

// captureRun runs the CLI and returns stdout.
func captureRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := Run(args)
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out), err
}

func TestCLIFullWorkflow(t *testing.T) {
	startServer(t)

	// create --silent -> prints id
	out, err := captureRun(t, "create", "a real task", "-p", "1", "-t", "bug", "--silent")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := strings.TrimSpace(out)
	if !strings.HasPrefix(id, "proj-") {
		t.Fatalf("bad id: %q", id)
	}

	// create without --silent
	if out, _ := captureRun(t, "create", "another"); !strings.Contains(out, "created") {
		t.Fatalf("create verbose: %q", out)
	}

	// ready (human)
	if out, _ := captureRun(t, "ready", "-n", "50"); !strings.Contains(out, id) {
		t.Fatalf("ready missing id: %q", out)
	}
	// ready --json
	if out, _ := captureRun(t, "ready", "--json"); !strings.Contains(out, "\"id\"") {
		t.Fatalf("ready json: %q", out)
	}
	// list
	if out, _ := captureRun(t, "list", "-s", "open"); !strings.Contains(out, id) {
		t.Fatalf("list: %q", out)
	}
	// show human + json
	if out, _ := captureRun(t, "show", id); !strings.Contains(out, "a real task") {
		t.Fatalf("show: %q", out)
	}
	if out, _ := captureRun(t, "show", id, "--json"); !strings.Contains(out, id) {
		t.Fatalf("show json: %q", out)
	}
	// claim
	if out, _ := captureRun(t, "update", id, "--claim"); !strings.Contains(out, "in_progress") {
		t.Fatalf("claim: %q", out)
	}
	// comment
	if out, _ := captureRun(t, "comment", id, "a", "note"); !strings.Contains(out, "comment added") {
		t.Fatalf("comment: %q", out)
	}
	// dep (with leading "add" bd-compat) to a second task
	out2, _ := captureRun(t, "create", "downstream", "--silent")
	id2 := strings.TrimSpace(out2)
	if out, _ := captureRun(t, "dep", "add", id2, id); !strings.Contains(out, "dependency added") {
		t.Fatalf("dep: %q", out)
	}
	// close
	if out, _ := captureRun(t, "close", id, "-r", "done"); !strings.Contains(out, "closed") {
		t.Fatalf("close: %q", out)
	}
}

func TestCLIErrorsAndMeta(t *testing.T) {
	startServer(t)
	// no command
	if _, err := captureRun(t); err == nil {
		t.Error("expected error for no command")
	}
	// help / prime
	if out, _ := captureRun(t, "help"); !strings.Contains(out, "Commands:") {
		t.Errorf("help: %q", out)
	}
	if out, _ := captureRun(t, "prime"); !strings.Contains(out, "workflow context") {
		t.Errorf("prime: %q", out)
	}
	// unknown command
	if _, err := captureRun(t, "bogus"); err == nil {
		t.Error("expected unknown command error")
	}
	// per-command help prints flags with descriptions (no server call)
	if out, err := captureRun(t, "create", "--help"); err != nil || !strings.Contains(out, "--priority") || !strings.Contains(out, "Priority 0-4") {
		t.Errorf("create --help: %q (%v)", out, err)
	}
	if out, _ := captureRun(t, "show", "-h"); !strings.Contains(out, "Usage: tasks show") {
		t.Errorf("show -h: %q", out)
	}
	// validation error surfaces
	if _, err := captureRun(t, "create", "x", "-t", "nonsense"); err == nil {
		t.Error("expected validation error")
	}
	// missing required arg (show with no id)
	if _, err := captureRun(t, "show"); err == nil {
		t.Error("expected missing id error")
	}
	// bad priority parse
	if _, err := captureRun(t, "create", "x", "-p", "notanint"); err == nil {
		t.Error("expected priority parse error")
	}
}

func TestCLIServerDown(t *testing.T) {
	t.Setenv("TASKS_URL", "http://127.0.0.1:1") // nothing listening
	t.Setenv("TASKS_TOKEN", "x")
	if _, err := captureRun(t, "ready"); err == nil {
		t.Error("expected connection error")
	}
}

// TestCLIWrongServer points the CLI at a non-tasks HTTP server (HTML 404) and
// asserts it fails with a legible message rather than dumping the HTML body.
func TestCLIWrongServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("<!DOCTYPE HTML><html><body>Error 404</body></html>"))
	}))
	defer srv.Close()
	t.Setenv("TASKS_URL", srv.URL)
	t.Setenv("TASKS_TOKEN", "")
	_, err := captureRun(t, "ready")
	if err == nil {
		t.Fatal("expected an error against a non-tasks server")
	}
	msg := err.Error()
	if strings.Contains(msg, "<!DOCTYPE") || strings.Contains(msg, "<html") {
		t.Fatalf("error leaked HTML body: %q", msg)
	}
	if !strings.Contains(msg, "is tasksd running") {
		t.Fatalf("error not helpful: %q", msg)
	}
}

// TestCLIUnauthorized asserts the 401 path produces a token hint.
func TestCLIUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	t.Setenv("TASKS_URL", srv.URL)
	t.Setenv("TASKS_TOKEN", "")
	_, err := captureRun(t, "ready")
	if err == nil || !strings.Contains(err.Error(), "TASKS_TOKEN") {
		t.Fatalf("expected token hint on 401, got %v", err)
	}
}

// TestCLINonJSON200 covers the "2xx but not JSON" branch (wrong server that
// happens to answer 200 with HTML/text).
func TestCLINonJSON200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello not json"))
	}))
	defer srv.Close()
	t.Setenv("TASKS_URL", srv.URL)
	t.Setenv("TASKS_TOKEN", "")
	if _, err := captureRun(t, "ready"); err == nil || !strings.Contains(err.Error(), "non-JSON") {
		t.Fatalf("expected non-JSON error, got %v", err)
	}
}

// TestCLIServerErrorJSON covers a 4xx that DOES carry a JSON {"error":...}.
func TestCLIServerErrorJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"nope specific"}`))
	}))
	defer srv.Close()
	t.Setenv("TASKS_URL", srv.URL)
	t.Setenv("TASKS_TOKEN", "")
	if _, err := captureRun(t, "ready"); err == nil || !strings.Contains(err.Error(), "nope specific") {
		t.Fatalf("expected structured error, got %v", err)
	}
}

// TestPrintTasksAlignment prints varied-width rows: the widest id sets the
// column width and shorter ids are padded to it.
func TestPrintTasksAlignment(t *testing.T) {
	p0, p1 := 0, 1
	rows := []model.Task{
		{ID: "proj-a", Title: "short", Status: "open", IssueType: "bug", Priority: &p0},
		{ID: "proj-longer.1.2", Title: "long id", Status: "in_progress", IssueType: "feature", Priority: &p1},
	}
	out := capture(func() { printTasks(rows) })
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	// The "P" of the priority column must appear at the same offset on both rows.
	i0 := strings.Index(lines[0], "P0")
	i1 := strings.Index(lines[1], "P1")
	if i0 != i1 || i0 < len("proj-longer.1.2") {
		t.Fatalf("columns misaligned: P at %d vs %d\n%s", i0, i1, out)
	}
}

func TestCLISearchAndTree(t *testing.T) {
	startServer(t)
	pid := strings.TrimSpace(mustOut(t, "create", "fuzzy parent epic", "-t", "epic", "--silent"))
	cid := strings.TrimSpace(mustOut(t, "create", "child alpha node", "--parent", pid, "--silent"))

	// list --parent shows direct children.
	if out, _ := captureRun(t, "list", "--parent", pid); !strings.Contains(out, cid) {
		t.Fatalf("list --parent: %q", out)
	}

	// tree renders the hierarchy with a connector.
	out, err := captureRun(t, "tree", pid)
	if err != nil {
		t.Fatalf("tree err: %v", err)
	}
	if !strings.Contains(out, pid) || !strings.Contains(out, cid) || !strings.Contains(out, "└─") {
		t.Fatalf("tree render:\n%s", out)
	}

	// fuzzy search (misspelled) finds the parent.
	if out, _ := captureRun(t, "search", "fzzy parnt"); !strings.Contains(out, pid) {
		t.Fatalf("fuzzy search: %q", out)
	}
	// search --json returns the child.
	if out, _ := captureRun(t, "search", "alpha", "--json"); !strings.Contains(out, cid) {
		t.Fatalf("search json: %q", out)
	}
	// find alias works.
	if out, _ := captureRun(t, "find", "alpha"); !strings.Contains(out, cid) {
		t.Fatalf("find alias: %q", out)
	}
}

func mustOut(t *testing.T, args ...string) string {
	t.Helper()
	out, err := captureRun(t, args...)
	if err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	return out
}
