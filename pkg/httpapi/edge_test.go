package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/tristanMatthias/tasks/pkg/core"
	"github.com/tristanMatthias/tasks/pkg/store"
	"github.com/tristanMatthias/tasks/web"
)

// closedCoreServer builds a server whose store is already closed, so read
// handlers hit their 500 error branches.
func closedCoreServer(t *testing.T) *httptest.Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := core.New(st, core.Options{Prefix: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	srv := New(Config{Core: c, Static: web.Static()})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestHandlerErrorBranches(t *testing.T) {
	ts := closedCoreServer(t)
	if code, _ := do(t, ts, "GET", "/api/issues", "", ""); code != http.StatusInternalServerError {
		t.Fatalf("issues on closed store = %d, want 500", code)
	}
	if code, _ := do(t, ts, "GET", "/api/meta", "", ""); code != http.StatusInternalServerError {
		t.Fatalf("meta on closed store = %d, want 500", code)
	}
	// REST op errors surface through writeCoreErr (400 for non-NotFound).
	if code, _ := do(t, ts, "GET", "/api/v1/ready", "", ""); code != http.StatusBadRequest {
		t.Fatalf("ready on closed store = %d, want 400", code)
	}
}

func TestTouchAndMCPMount(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	t.Cleanup(func() { st.Close() })
	c, _ := core.New(st, core.Options{Prefix: "proj"})
	// dummy MCP handler to cover the mount branch
	mcpHit := false
	mcp := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mcpHit = true; w.WriteHeader(200) })
	srv := New(Config{Core: c, Static: web.Static(), MCP: mcp})
	srv.Touch() // exercise Touch directly
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/mcp")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !mcpHit {
		t.Fatal("MCP handler not mounted/called")
	}
}

func TestServeStaticMissing(t *testing.T) {
	// A static FS without index.html triggers the 404 branch.
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	t.Cleanup(func() { st.Close() })
	c, _ := core.New(st, core.Options{Prefix: "proj"})
	srv := New(Config{Core: c, Static: fstest.MapFS{}}) // no index.html
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	if code, _ := do(t, ts, "GET", "/", "", ""); code != http.StatusNotFound {
		t.Fatalf("missing index = %d, want 404", code)
	}
}
