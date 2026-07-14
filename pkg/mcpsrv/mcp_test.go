package mcpsrv

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tristanMatthias/tasks/pkg/core"
	"github.com/tristanMatthias/tasks/pkg/model"
	"github.com/tristanMatthias/tasks/pkg/store"
)

func newCore(t *testing.T) *core.Core {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	c, err := core.New(st, core.Options{Prefix: "proj", Actor: "mcp-tester"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func connect(t *testing.T, c *core.Core) *mcp.ClientSession {
	t.Helper()
	srv := NewServer(c)
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return res
}

func text(res *mcp.CallToolResult) string {
	if len(res.Content) == 0 {
		return ""
	}
	if tc, ok := res.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

func task(t *testing.T, res *mcp.CallToolResult) model.Task {
	t.Helper()
	var tk model.Task
	if err := json.Unmarshal([]byte(text(res)), &tk); err != nil {
		t.Fatalf("unmarshal task from %q: %v", text(res), err)
	}
	return tk
}

func tasks(t *testing.T, res *mcp.CallToolResult) []model.Task {
	t.Helper()
	var ts []model.Task
	if err := json.Unmarshal([]byte(text(res)), &ts); err != nil {
		t.Fatalf("unmarshal tasks from %q: %v", text(res), err)
	}
	return ts
}

func containsID(ts []model.Task, id string) bool {
	for _, t := range ts {
		if t.ID == id {
			return true
		}
	}
	return false
}

func TestMCPToolsListed(t *testing.T) {
	cs := connect(t, newCore(t))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"ready": false, "show": false, "list": false, "create": false,
		"update": false, "claim": false, "close": false, "dep": false, "comment": false}
	for _, tool := range res.Tools {
		want[tool.Name] = true
		if tool.InputSchema == nil {
			t.Errorf("tool %q has nil InputSchema", tool.Name)
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("tool %q not registered", name)
		}
	}
	// gate add is a normal tool; verify (Local) + gate rm (HideMCP) must NOT be
	// exposed — the API/MCP cannot verify a gate or delete one to bypass it.
	tools := map[string]bool{}
	for _, tool := range res.Tools {
		tools[tool.Name] = true
	}
	if !tools["gate_add"] {
		t.Error("gate_add should be an MCP tool")
	}
	if tools["verify"] {
		t.Error("verify must NOT be an MCP tool")
	}
	if tools["gate_rm"] {
		t.Error("gate_rm must NOT be an MCP tool")
	}
}

func TestMCPWorkflow(t *testing.T) {
	cs := connect(t, newCore(t))

	created := task(t, call(t, cs, "create", map[string]any{"title": "mcp task", "priority": 1, "issue_type": "bug"}))
	id := created.ID
	if id == "" || created.IssueType != "bug" {
		t.Fatalf("bad created task: %+v", created)
	}

	if !containsID(tasks(t, call(t, cs, "ready", map[string]any{"limit": 50})), id) {
		t.Fatalf("ready did not include %s", id)
	}

	claimed := task(t, call(t, cs, "claim", map[string]any{"id": id}))
	if claimed.Status != "in_progress" || claimed.Assignee == "" {
		t.Fatalf("claim failed: %+v", claimed)
	}

	call(t, cs, "comment", map[string]any{"id": id, "text": "hi"})
	shown := task(t, call(t, cs, "show", map[string]any{"id": id}))
	if shown.CommentCount != 1 {
		t.Fatalf("expected 1 comment, got %d", shown.CommentCount)
	}

	closed := task(t, call(t, cs, "close", map[string]any{"id": id, "reason": "done"}))
	if closed.Status != "closed" || closed.CloseReason != "done" {
		t.Fatalf("close failed: %+v", closed)
	}

	// error path: show missing id -> IsError
	if !call(t, cs, "show", map[string]any{"id": "nope"}).IsError {
		t.Fatalf("expected error result for missing id")
	}
	// validation path: invalid status -> IsError
	if !call(t, cs, "create", map[string]any{"title": "x", "issue_type": "bogus"}).IsError {
		t.Fatalf("expected validation error for bad issue_type")
	}
}

func TestMCPDep(t *testing.T) {
	cs := connect(t, newCore(t))
	a := task(t, call(t, cs, "create", map[string]any{"title": "A"}))
	b := task(t, call(t, cs, "create", map[string]any{"title": "B"}))
	if call(t, cs, "dep", map[string]any{"blocked": b.ID, "blocker": a.ID}).IsError {
		t.Fatal("dep failed")
	}
	if containsID(tasks(t, call(t, cs, "ready", map[string]any{"limit": 50})), b.ID) {
		t.Fatalf("B should be blocked by A")
	}
}

func TestMCPOverHTTP(t *testing.T) {
	h := Handler(newCore(t))
	ts := httptest.NewServer(h)
	defer ts.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "http-test", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: ts.URL}, nil)
	if err != nil {
		t.Fatalf("connect over http: %v", err)
	}
	defer cs.Close()
	out := task(t, call(t, cs, "create", map[string]any{"title": "via http"}))
	if out.Title != "via http" {
		t.Fatalf("http create wrong: %+v", out)
	}
}
