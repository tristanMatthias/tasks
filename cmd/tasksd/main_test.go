package main

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/tristanMatthias/tasks/internal/store"
)

const fixture = `{"id":"proj-a","title":"a","status":"open","priority":2,"issue_type":"task"}
{"id":"proj-b","title":"b","status":"open","priority":1,"issue_type":"bug"}
`

func writeFixture(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "issues.jsonl")
	if err := os.WriteFile(p, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaultDB(t *testing.T) {
	t.Setenv("TASKS_DB", "/custom/path.db")
	if defaultDB() != "/custom/path.db" {
		t.Error("defaultDB env")
	}
	os.Unsetenv("TASKS_DB")
	if defaultDB() != filepath.Join("data", "tasks.db") {
		t.Error("defaultDB default")
	}
}

func TestMustOpen(t *testing.T) {
	db := filepath.Join(t.TempDir(), "nested", "x.db") // dir does not exist yet
	st := mustOpen(db)
	defer st.Close()
	if _, err := os.Stat(filepath.Dir(db)); err != nil {
		t.Fatal("mustOpen did not create dir")
	}
}

func TestRunImportAndExport(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "t.db")
	src := writeFixture(t)

	// import
	runImport([]string{"--db", db, src})
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := st.Count()
	st.Close()
	if n != 2 {
		t.Fatalf("imported count = %d", n)
	}

	// export
	out := filepath.Join(dir, "out.jsonl")
	runExport([]string{"--db", db, "--out", out})
	data, err := os.ReadFile(out)
	if err != nil || len(data) == 0 {
		t.Fatalf("export produced no file: %v", err)
	}
}

// freePort returns an unused localhost address.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

// TestServeLifecycle boots the real server (with startup import + export wiring)
// and shuts it down via SIGTERM, covering serve()'s happy path end to end.
func TestServeLifecycle(t *testing.T) {
	dir := t.TempDir()
	addr := freePort(t)
	src := writeFixture(t)
	done := make(chan error, 1)
	go func() {
		done <- serve([]string{
			"--addr", addr,
			"--db", filepath.Join(dir, "t.db"),
			"--import", src,
			"--export", filepath.Join(dir, "out.jsonl"),
			"--actor", "srv",
		}, func(string) string { return "" })
	}()

	// Wait for the server to accept requests.
	base := "http://" + addr
	up := false
	for i := 0; i < 100; i++ {
		if resp, err := http.Get(base + "/api/meta"); err == nil {
			resp.Body.Close()
			up = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !up {
		t.Fatal("server did not come up")
	}
	// Trigger a mutation so the exporter's Notify path runs.
	http.Post(base+"/api/v1/tasks", "application/json", nil) // 400 body but exercises route

	// Signal graceful shutdown; serve() should return nil.
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not shut down after SIGTERM")
	}
}
