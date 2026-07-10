package exporter

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tristanMatthias/tasks/pkg/model"
)

type fakeSource struct {
	tasks []model.Task
	err   error
}

func (f *fakeSource) All() ([]model.Task, error) { return f.tasks, f.err }

func sample() *fakeSource {
	return &fakeSource{tasks: []model.Task{
		{ID: "proj-b", Title: "b", Status: "open", IssueType: "task"},
		{ID: "proj-a", Title: "a", Status: "open", IssueType: "task"},
	}}
}

func TestDisabledExporter(t *testing.T) {
	if e := New(sample(), Config{Path: ""}, nil); e != nil {
		t.Fatal("empty path should disable exporter")
	}
	// nil-safe methods
	var e *Exporter
	e.Notify()
	e.Run(context.Background())
	if err := e.ExportOnce(); err != nil {
		t.Fatalf("nil ExportOnce: %v", err)
	}
}

func TestExportOnce(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "nested", "issues.jsonl") // nested to test MkdirAll
	e := New(sample(), Config{Path: out}, nil)
	if err := e.ExportOnce(); err != nil {
		t.Fatal(err)
	}
	tasks := readJSONL(t, out)
	if len(tasks) != 2 || tasks[0].ID != "proj-a" { // natural-id sorted
		t.Fatalf("export wrong: %+v", tasks)
	}
}

func TestExportSourceError(t *testing.T) {
	e := New(&fakeSource{err: context.DeadlineExceeded}, Config{Path: filepath.Join(t.TempDir(), "x.jsonl")}, nil)
	if err := e.ExportOnce(); err == nil {
		t.Fatal("expected source error")
	}
}

func TestDebounceRun(t *testing.T) {
	out := filepath.Join(t.TempDir(), "issues.jsonl")
	e := New(sample(), Config{Path: out, Interval: 20 * time.Millisecond}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.Run(ctx); close(done) }()

	// Burst of notifies should coalesce into (at least) one export.
	for i := 0; i < 5; i++ {
		e.Notify()
	}
	// Wait for the debounced export to land.
	waitFor(t, func() bool { _, err := os.Stat(out); return err == nil }, time.Second)

	// Notify again, then cancel — the ctx.Done flush path exports pending work.
	e.Notify()
	cancel()
	<-done
	if len(readJSONL(t, out)) != 2 {
		t.Fatalf("expected 2 tasks after debounce+flush")
	}
}

func TestGitBackup(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "t@example.com")
	gitRun(t, dir, "config", "user.name", "t")

	out := filepath.Join(dir, "issues.jsonl")
	e := New(sample(), Config{Path: out, Git: true, Message: "test export"}, nil)
	if err := e.ExportOnce(); err != nil {
		t.Fatalf("export+git: %v", err)
	}
	// A commit should now exist touching issues.jsonl.
	logOut := gitRun(t, dir, "log", "--oneline")
	if logOut == "" {
		t.Fatal("no commit created")
	}
	// Second export with no changes must be a no-op (no error).
	if err := e.ExportOnce(); err != nil {
		t.Fatalf("second export: %v", err)
	}
}

// ---- helpers ----

func readJSONL(t *testing.T, path string) []model.Task {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ts, err := model.ReadJSONL(f)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func waitFor(t *testing.T, cond func() bool, max time.Duration) {
	t.Helper()
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
