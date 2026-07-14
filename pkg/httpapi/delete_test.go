package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/core"
	"github.com/tristanMatthias/tasks/pkg/store"
	"github.com/tristanMatthias/tasks/web"
)

func deleteServer(t *testing.T, allowDelete func(*http.Request) bool) (*httptest.Server, *core.Core) {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/d.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	c, err := core.New(st, core.Options{Prefix: "proj", Actor: "srv"})
	if err != nil {
		t.Fatal(err)
	}
	srv := New(Config{Core: c, Static: web.Static(), Token: "sess", APIKeys: true, AllowDelete: allowDelete})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, c
}

// sessionOnly mirrors what an embedder supplies: humans yes, API keys never.
func sessionOnly(r *http.Request) bool {
	id, ok := IdentityFrom(r.Context())
	return ok && !strings.HasPrefix(id.Subject, "key:")
}

func newTask(t *testing.T, ts *httptest.Server, token string) string {
	t.Helper()
	code, body := do(t, ts, "POST", "/api/v1/tasks", token, `{"title":"doomed"}`)
	if code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &out)
	return out.ID
}

func TestDelete_HumanSessionOnly(t *testing.T) {
	ts, c := deleteServer(t, sessionOnly)
	key, _ := c.CreateKey("bot", "tester")

	// A bot/API key is REFUSED even though it can otherwise read/write.
	id := newTask(t, ts, "sess")
	if code, _ := do(t, ts, "DELETE", "/api/v1/tasks/"+id, key.Secret, ""); code != http.StatusForbidden {
		t.Fatalf("delete via API key = %d, want 403", code)
	}
	if _, err := c.Show(id); err != nil {
		t.Fatal("task must still exist after a refused delete")
	}

	// The human session succeeds and the task is gone.
	if code, _ := do(t, ts, "DELETE", "/api/v1/tasks/"+id, "sess", ""); code != http.StatusNoContent {
		t.Fatalf("delete via session = %d, want 204", code)
	}
	if _, err := c.Show(id); err == nil {
		t.Fatal("task should be gone after delete")
	}
}

func TestDelete_RouteAbsentWhenNotConfigured(t *testing.T) {
	ts, _ := deleteServer(t, nil) // no AllowDelete -> no route at all
	id := newTask(t, ts, "sess")
	code, _ := do(t, ts, "DELETE", "/api/v1/tasks/"+id, "sess", "")
	if code == http.StatusNoContent {
		t.Fatal("delete must not work when AllowDelete is unset")
	}
}
