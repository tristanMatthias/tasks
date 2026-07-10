package exporter

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGitFailureSurfaces covers the git error branch: Git enabled but the
// directory is not a git repo, so `git add` fails and export returns an error.
func TestGitFailureSurfaces(t *testing.T) {
	dir := t.TempDir() // not a git repo
	out := filepath.Join(dir, "issues.jsonl")
	e := New(sample(), Config{Path: out, Git: true}, nil)
	if err := e.ExportOnce(); err == nil {
		t.Fatal("expected git error in a non-repo directory")
	}
}

// TestRenameFailure covers the rename error branch: the destination path is a
// directory, so the temp->dest rename cannot succeed.
func TestRenameFailure(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "issues.jsonl")
	if err := os.Mkdir(dest, 0o755); err != nil { // make dest a directory
		t.Fatal(err)
	}
	e := New(sample(), Config{Path: dest}, nil)
	if err := e.ExportOnce(); err == nil {
		t.Fatal("expected rename error when dest is a directory")
	}
}

// TestExportNowLogsError drives exportNow (the debounce-loop export path) with a
// failing source, exercising its error-log branch.
func TestExportNowLogsError(t *testing.T) {
	var logged bool
	e := New(&fakeSource{err: os.ErrClosed}, Config{Path: filepath.Join(t.TempDir(), "x.jsonl")},
		func(string, ...any) { logged = true })
	e.exportNow()
	if !logged {
		t.Fatal("exportNow should log on error")
	}
	// success path logs too
	e2 := New(sample(), Config{Path: filepath.Join(t.TempDir(), "ok.jsonl")}, func(string, ...any) {})
	e2.exportNow()
}

// TestGitPushFailure covers the git-push error branch: commit succeeds but there
// is no remote, so `git push` fails and the error surfaces.
func TestGitPushFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "t@example.com")
	gitRun(t, dir, "config", "user.name", "t")
	e := New(sample(), Config{Path: filepath.Join(dir, "issues.jsonl"), Git: true, GitPush: true, Message: "m"}, nil)
	if err := e.ExportOnce(); err == nil {
		t.Fatal("expected push failure with no remote")
	}
}
