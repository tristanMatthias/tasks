package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/core"
	"github.com/tristanMatthias/tasks/pkg/store"
	"github.com/tristanMatthias/tasks/web"
)

func prodServer(t *testing.T, cfg Config) *httptest.Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	c, err := core.New(st, core.Options{Prefix: "proj", Actor: "srv"})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Core = c
	if cfg.Static == nil {
		cfg.Static = web.Static()
	}
	srv := New(cfg)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestOpsEndpoints(t *testing.T) {
	ts := prodServer(t, Config{Token: "tok", Metrics: true})
	// health/readyz/version/metrics are public (no auth).
	for _, path := range []string{"/healthz", "/readyz", "/version", "/metrics"} {
		code, _ := do(t, ts, "GET", path, "", "")
		if code != http.StatusOK {
			t.Errorf("%s = %d, want 200 (public)", path, code)
		}
	}
	// metrics body is Prometheus text
	_, body := do(t, ts, "GET", "/metrics", "", "")
	if !strings.Contains(string(body), "tasks_requests_total") {
		t.Errorf("metrics missing counter: %s", body)
	}
}

func TestMetricsDisabled(t *testing.T) {
	// No token (auth off) so /metrics falls through to the app mux and 404s.
	ts := prodServer(t, Config{Metrics: false})
	if code, _ := do(t, ts, "GET", "/metrics", "", ""); code != http.StatusNotFound {
		t.Fatalf("metrics disabled should 404, got %d", code)
	}
}

func TestSecurityHeadersAndRequestID(t *testing.T) {
	ts := prodServer(t, Config{Token: "tok"})
	req, _ := http.NewRequest("GET", ts.URL+"/healthz", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" ||
		resp.Header.Get("X-Frame-Options") != "DENY" ||
		resp.Header.Get("Content-Security-Policy") == "" ||
		resp.Header.Get("X-Request-Id") == "" {
		t.Fatalf("missing security headers: %v", resp.Header)
	}
	// inbound request id is echoed
	req2, _ := http.NewRequest("GET", ts.URL+"/healthz", nil)
	req2.Header.Set("X-Request-Id", "my-trace-id")
	r2, _ := http.DefaultClient.Do(req2)
	r2.Body.Close()
	if r2.Header.Get("X-Request-Id") != "my-trace-id" {
		t.Fatalf("request id not echoed: %s", r2.Header.Get("X-Request-Id"))
	}
}

func TestRateLimit(t *testing.T) {
	ts := prodServer(t, Config{Token: "tok", RateLimit: 1, RateBurst: 2})
	got429 := false
	for i := 0; i < 10; i++ {
		if code, _ := do(t, ts, "GET", "/healthz", "", ""); code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatal("expected a 429 under a tight rate limit")
	}
}

func TestBodyLimit(t *testing.T) {
	ts := prodServer(t, Config{Token: "tok", MaxBodyBytes: 16})
	big := `{"title":"` + strings.Repeat("x", 1000) + `"}`
	code, _ := do(t, ts, "POST", "/api/v1/tasks", "tok", big)
	if code == http.StatusCreated {
		t.Fatal("oversized body should be rejected")
	}
}

func TestCORS(t *testing.T) {
	ts := prodServer(t, Config{Token: "tok", CORSOrigins: []string{"https://ui.example"}})
	// preflight from an allowed origin
	req, _ := http.NewRequest("OPTIONS", ts.URL+"/api/v1/ready", nil)
	req.Header.Set("Origin", "https://ui.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Access-Control-Allow-Origin") != "https://ui.example" {
		t.Fatalf("cors origin not allowed: %v", resp.Header)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight = %d", resp.StatusCode)
	}
	// disallowed origin gets no ACAO header
	req2, _ := http.NewRequest("GET", ts.URL+"/healthz", nil)
	req2.Header.Set("Origin", "https://evil.example")
	r2, _ := http.DefaultClient.Do(req2)
	r2.Body.Close()
	if r2.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("disallowed origin should not be echoed")
	}
}

func TestPanicRecovery(t *testing.T) {
	// A handler that panics is caught by the recoverer middleware -> 500.
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	t.Cleanup(func() { st.Close() })
	c, _ := core.New(st, core.Options{Prefix: "proj"})
	srv := New(Config{Core: c, Static: web.Static()})
	root := srv.Handler()
	// Wrap: replace the app by injecting a panicking path via a custom mux is hard;
	// instead drive the recoverer directly.
	rec := httptest.NewRecorder()
	panicky := recoverer(srv.logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	panicky.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic not recovered: %d", rec.Code)
	}
	_ = root
}

func TestReadyzUnavailable(t *testing.T) {
	// Closed store -> readiness 503.
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	c, _ := core.New(st, core.Options{Prefix: "proj"})
	st.Close()
	srv := New(Config{Core: c, Static: web.Static()})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("readyz on closed store = %d (%s)", resp.StatusCode, b)
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "1.2.3.4:5555"
	if ip := clientIP(r, false); ip != "1.2.3.4" {
		t.Errorf("clientIP = %q", ip)
	}
	r.Header.Set("X-Forwarded-For", "9.9.9.9, 8.8.8.8")
	if ip := clientIP(r, true); ip != "9.9.9.9" {
		t.Errorf("clientIP behind proxy = %q", ip)
	}
	// not trusted -> RemoteAddr wins
	if ip := clientIP(r, false); ip != "1.2.3.4" {
		t.Errorf("clientIP untrusted proxy = %q", ip)
	}
}
