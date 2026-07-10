package importer

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/store"
)

const fixture = `
{"id":"proj-aaaa","title":"open task","status":"open","priority":2,"issue_type":"task","labels":["x"]}
{"id":"proj-bbbb","title":"closed bug","status":"closed","priority":0,"issue_type":"bug","comments":[{"id":"c1","issue_id":"proj-bbbb","author":"Claude","text":"done"}]}
{"id":"proj-cccc.1","title":"child","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"proj-cccc.1","depends_on_id":"proj-cccc","type":"parent-child","metadata":"{}"}]}
{"_type":"memory","key":"note-one","value":"v1"}
{"_type":"memory","key":"note-two","value":"v2"}
`

func writeFixture(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sample.jsonl")
	if err := os.WriteFile(p, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestImportFixtureIdempotent(t *testing.T) {
	st := newStore(t)
	p := writeFixture(t)

	n, err := ImportFile(st, p)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if n != 5 {
		t.Fatalf("imported %d, want 5", n)
	}
	// Re-import must not duplicate (idempotent upsert).
	if _, err := ImportFile(st, p); err != nil {
		t.Fatalf("re-import: %v", err)
	}
	count, _ := st.Count()
	if count != 5 {
		t.Fatalf("after re-import store has %d, want 5", count)
	}

	// memory records get distinct synthetic ids (no collapse).
	if ok, _ := st.Exists("memory:note-one"); !ok {
		t.Error("memory:note-one not imported")
	}
	if ok, _ := st.Exists("memory:note-two"); !ok {
		t.Error("memory:note-two not imported")
	}

	// comment + dep preserved.
	b, err := st.Get("proj-bbbb")
	if err != nil || b.CommentCount != 1 {
		t.Fatalf("bbbb comment lost: %+v (%v)", b, err)
	}
	child, _ := st.Get("proj-cccc.1")
	if len(child.Dependencies) != 1 || child.Dependencies[0].Type != "parent-child" {
		t.Fatalf("cccc.1 dep lost: %+v", child.Dependencies)
	}
}

func TestImportMissingFile(t *testing.T) {
	st := newStore(t)
	if _, err := ImportFile(st, filepath.Join(t.TempDir(), "nope.jsonl")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestImportRealData imports the live beads export if present (dynamic count).
func TestImportRealData(t *testing.T) {
	const src = "../../../forge-crafting-intepreters/.beads/issues.jsonl"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("source jsonl not available: %v", err)
	}
	st := newStore(t)
	n, err := ImportFile(st, src)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	want := countLines(t, src)
	if n != want {
		t.Fatalf("imported %d, file has %d lines", n, want)
	}
}

func countLines(t *testing.T, path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24)
	n := 0
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) > 0 {
			n++
		}
	}
	return n
}
