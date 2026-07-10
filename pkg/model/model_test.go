package model

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// fixture covers the field variety seen in real data: normal task, closed+comment,
// epic, parent-child child, blocks dep, null priority, deferred, labels, and a
// beads "memory" record that carries only _type/key/value (no id/title/status).
const fixture = `
{"id":"proj-aaaa","title":"open task","status":"open","priority":2,"issue_type":"task","labels":["x","y"],"created_at":"2026-01-01T00:00:00Z"}
{"id":"proj-bbbb","title":"closed bug","status":"closed","priority":0,"issue_type":"bug","close_reason":"fixed","closed_at":"2026-01-02T00:00:00Z","comments":[{"id":"c1","issue_id":"proj-bbbb","author":"Claude","text":"done","created_at":"2026-01-02T00:00:00Z"}]}
{"id":"proj-cccc","title":"epic","status":"open","priority":1,"issue_type":"epic"}
{"id":"proj-cccc.1","title":"child","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"proj-cccc.1","depends_on_id":"proj-cccc","type":"parent-child","metadata":"{}"}]}
{"id":"proj-dddd","title":"blocked","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"proj-dddd","depends_on_id":"proj-cccc","type":"blocks","metadata":"{}"}]}
{"id":"proj-eeee","title":"no priority","status":"open","issue_type":"chore"}
{"id":"proj-ffff","title":"deferred","status":"deferred","priority":3,"issue_type":"feature"}
{"_type":"memory","key":"a-note","value":"remember this"}
`

func parseFixture(t *testing.T) []Task {
	t.Helper()
	ts, err := ReadJSONL(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("ReadJSONL: %v", err)
	}
	return ts
}

func TestReadJSONLFields(t *testing.T) {
	ts := parseFixture(t)
	if len(ts) != 8 {
		t.Fatalf("got %d tasks, want 8", len(ts))
	}
	byID := map[string]Task{}
	for _, x := range ts {
		byID[x.ID] = x
	}

	if p := byID["proj-aaaa"].Priority; p == nil || *p != 2 {
		t.Errorf("aaaa priority = %v, want 2", p)
	}
	if got := byID["proj-aaaa"].Labels; !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Errorf("aaaa labels = %v", got)
	}
	if b := byID["proj-bbbb"]; len(b.Comments) != 1 || b.Comments[0].Author != "Claude" || b.CloseReason != "fixed" {
		t.Errorf("bbbb comment/close wrong: %+v", b)
	}
	if e := byID["proj-eeee"]; e.Priority != nil {
		t.Errorf("eeee priority should be nil, got %v", *e.Priority)
	}
	child := byID["proj-cccc.1"]
	if len(child.Dependencies) != 1 || child.Dependencies[0].Type != "parent-child" || child.Dependencies[0].DependsOnID != "proj-cccc" {
		t.Errorf("cccc.1 dep wrong: %+v", child.Dependencies)
	}
	// memory record: no id, but _type/key/value preserved.
	var mem *Task
	for i := range ts {
		if ts[i].TypeString() == "memory" {
			mem = &ts[i]
		}
	}
	if mem == nil || mem.Key == nil || *mem.Key != "a-note" || mem.Value == nil || *mem.Value != "remember this" {
		t.Fatalf("memory record not preserved: %+v", mem)
	}
	if mem.ID != "" {
		t.Errorf("memory record should have empty id at model layer, got %q", mem.ID)
	}
}

func TestReadJSONLBadLine(t *testing.T) {
	_, err := ReadJSONL(strings.NewReader("{not json}\n"))
	if err == nil {
		t.Fatal("expected error on bad json line")
	}
}

func TestWriteJSONLRoundTrip(t *testing.T) {
	ts := parseFixture(t)
	var buf bytes.Buffer
	if err := WriteJSONL(&buf, ts); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	// Re-read and confirm same count + ids present.
	back, err := ReadJSONL(&buf)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if len(back) != len(ts) {
		t.Fatalf("round-trip count %d != %d", len(back), len(ts))
	}
	// Output must be natural-id sorted.
	var ids []string
	for _, x := range back {
		if x.ID != "" {
			ids = append(ids, x.ID)
		}
	}
	for i := 1; i < len(ids); i++ {
		if !NaturalLess(ids[i-1], ids[i]) && ids[i-1] != ids[i] {
			t.Errorf("output not sorted: %s before %s", ids[i-1], ids[i])
		}
	}
}

func TestMarshalLabels(t *testing.T) {
	if MarshalLabels(nil) != "" {
		t.Error("nil labels should marshal to empty string")
	}
	if got := MarshalLabels([]string{"a", "b"}); got != `["a","b"]` {
		t.Errorf("MarshalLabels = %q", got)
	}
}

func TestPriorityAndTypeHelpers(t *testing.T) {
	p := 3
	tk := Task{Priority: &p}
	if tk.PriorityOr(9) != 3 {
		t.Error("PriorityOr with value")
	}
	empty := Task{}
	if empty.PriorityOr(9) != 9 {
		t.Error("PriorityOr default")
	}
	if empty.TypeString() != "" {
		t.Error("TypeString nil")
	}
	mt := "memory"
	memTask := Task{Type: &mt}
	if memTask.TypeString() != "memory" {
		t.Error("TypeString value")
	}
}

// TestRoundTripRealData is a smoke test against the live beads export if present.
// Count is derived from the file (it mutates as the source repo changes).
func TestRoundTripRealData(t *testing.T) {
	const src = "../../../forge-crafting-intepreters/.beads/issues.jsonl"
	f, err := os.Open(src)
	if err != nil {
		t.Skipf("source jsonl not available: %v", err)
	}
	defer f.Close()

	tasks, err := ReadJSONL(f)
	if err != nil {
		t.Fatalf("ReadJSONL: %v", err)
	}
	want := countLines(t, src)
	if len(tasks) != want {
		t.Fatalf("parsed %d tasks, file has %d non-empty lines", len(tasks), want)
	}
	if want < 100 {
		t.Fatalf("suspiciously few records: %d", want)
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
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	_ = json.Valid
	return n
}
