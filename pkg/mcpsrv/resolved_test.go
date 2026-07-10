package mcpsrv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tristanMatthias/tasks/pkg/core"
)

// hdrRT injects a tenant header on every MCP request so the resolved handler can
// route the whole session (initialize + tools/list) to one Core.
type hdrRT struct {
	base   http.RoundTripper
	tenant string
}

func (h hdrRT) RoundTrip(r *http.Request) (*http.Response, error) {
	if h.tenant != "" {
		r.Header.Set("X-Tenant", h.tenant)
	}
	return h.base.RoundTrip(r)
}

func connectHTTP(t *testing.T, url, tenant string) *mcp.ClientSession {
	t.Helper()
	hc := &http.Client{Transport: hdrRT{http.DefaultTransport, tenant}}
	tr := &mcp.StreamableClientTransport{Endpoint: url, HTTPClient: hc}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), tr, nil)
	if err != nil {
		t.Fatalf("connect (%s): %v", tenant, err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func toolNames(t *testing.T, cs *mcp.ClientSession) map[string]bool {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tl := range res.Tools {
		names[tl.Name] = true
	}
	return names
}

// TestHandlerResolvedRoutesPerRequest proves the handler picks the Core per
// request (over real streamable HTTP) and that an unresolved request gets the
// empty (no-tools) server.
func TestHandlerResolvedRoutesPerRequest(t *testing.T) {
	cA := newCore(t)
	if _, err := cA.Create(core.CreateParams{Title: "only in A"}); err != nil {
		t.Fatal(err)
	}
	resolve := func(r *http.Request) (*core.Core, error) {
		if r.Header.Get("X-Tenant") == "A" {
			return cA, nil
		}
		return nil, http.ErrNoLocation // force the empty-server path
	}
	ts := httptest.NewServer(HandlerResolved(resolve))
	// Register before any client sessions so, by LIFO cleanup order, the sessions
	// (and their standalone SSE streams) close before the server does.
	t.Cleanup(ts.Close)

	// Tenant A gets the full tool surface, twice (exercises the server cache).
	if n := toolNames(t, connectHTTP(t, ts.URL, "A")); !n["ready"] || !n["keys_create"] {
		t.Fatalf("tenant A should list tools, got %v", n)
	}
	if n := toolNames(t, connectHTTP(t, ts.URL, "A")); !n["ready"] {
		t.Fatalf("cached server should still list tools, got %v", n)
	}
	// Unresolved requests hit the empty server: no task tools.
	if n := toolNames(t, connectHTTP(t, ts.URL, "")); n["ready"] {
		t.Fatalf("unresolved request should expose no tools, got %v", n)
	}
}
