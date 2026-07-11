package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tristanMatthias/tasks/pkg/core"
	"github.com/tristanMatthias/tasks/pkg/mcpsrv"
	"github.com/tristanMatthias/tasks/pkg/store"
	"github.com/tristanMatthias/tasks/web"
)

type authRT struct {
	base  http.RoundTripper
	token string
}

func (a authRT) RoundTrip(r *http.Request) (*http.Response, error) {
	if a.token != "" {
		r.Header.Set("Authorization", "Bearer "+a.token)
	}
	return a.base.RoundTrip(r)
}

// TestMCPBehindKeyAuth confirms the exact path a bot uses: the MCP endpoint,
// mounted behind the auth gate, is reachable with an API key and rejects a
// request without one — i.e. Claude-Code-style "Bearer tasks_…" header auth.
func TestMCPBehindKeyAuth(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	c, err := core.New(st, core.Options{Prefix: "proj", Actor: "srv"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Create(core.CreateParams{Title: "seeded"}); err != nil {
		t.Fatal(err)
	}
	k, err := c.CreateKey("bot", "tester")
	if err != nil {
		t.Fatal(err)
	}

	// A shared token makes the base authenticator reject uncredentialed requests,
	// so we can assert the gate blocks; the API key path is layered on top.
	srv := New(Config{Core: c, Static: web.Static(), Token: "sharedtok", APIKeys: true, MCP: mcpsrv.Handler(c)})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close) // registered first -> runs after the session cleanup (LIFO)

	// No credential -> the gate blocks the MCP handshake (plain POST, no SSE).
	if code, _ := do(t, ts, "POST", "/mcp", "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated MCP should be 401, got %d", code)
	}

	// With the API key -> tools are listed and callable, scoped to this board.
	hc := &http.Client{Transport: authRT{http.DefaultTransport, k.Secret}}
	tr := &mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp", HTTPClient: hc}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), tr, nil)
	if err != nil {
		t.Fatalf("MCP connect with key: %v", err)
	}
	t.Cleanup(func() { cs.Close() })

	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var hasReady bool
	for _, tl := range tools.Tools {
		if tl.Name == "ready" {
			hasReady = true
		}
	}
	if !hasReady {
		t.Fatal("expected the 'ready' tool over MCP")
	}
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "ready", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("call ready: %v", err)
	}
	if res.IsError {
		t.Fatalf("ready errored: %+v", res.Content)
	}
}
