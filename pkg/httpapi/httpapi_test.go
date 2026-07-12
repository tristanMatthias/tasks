package httpapi

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/core"
	"github.com/tristanMatthias/tasks/pkg/model"
	"github.com/tristanMatthias/tasks/pkg/store"
	"github.com/tristanMatthias/tasks/web"
)

func newServer(t *testing.T, token string) *httptest.Server {
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
	changes := 0
	c.SetOnChange(func() { changes++ })
	srv := New(Config{Core: c, Static: web.Static(), Token: token})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func do(t *testing.T, ts *httptest.Server, method, path, token, body string) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, ts.URL+path, r)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func TestAuth(t *testing.T) {
	ts := newServer(t, "sekret")
	// no token -> 401
	if code, _ := do(t, ts, "GET", "/api/issues", "", ""); code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
	// wrong token -> 401
	if code, _ := do(t, ts, "GET", "/api/issues", "nope", ""); code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d", code)
	}
	// right token -> 200
	if code, _ := do(t, ts, "GET", "/api/issues", "sekret", ""); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestAuthCookieBootstrap(t *testing.T) {
	ts := newServer(t, "sekret")
	// /auth?token= with a client that does NOT follow redirects.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(ts.URL + "/auth?token=sekret")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect, got %d", resp.StatusCode)
	}
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == cookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no auth cookie set")
	}
	// Cookie now authorizes /api/issues.
	req, _ := http.NewRequest("GET", ts.URL+"/api/issues", nil)
	req.AddCookie(cookie)
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("cookie auth failed: %d", r2.StatusCode)
	}
	// bad token at /auth -> 401
	resp2, _ := client.Get(ts.URL + "/auth?token=wrong")
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad /auth token = %d", resp2.StatusCode)
	}
}

func TestNoAuthMode(t *testing.T) {
	ts := newServer(t, "") // dev mode: no token
	if code, _ := do(t, ts, "GET", "/api/issues", "", ""); code != http.StatusOK {
		t.Fatalf("no-auth mode should allow, got %d", code)
	}
}

func TestInjectHead(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	c, _ := core.New(st, core.Options{Prefix: "p"})
	srv := New(Config{Core: c, Static: web.Static(), InjectHead: `<script id="inject-probe"></script>`})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	_, body := do(t, ts, "GET", "/", "", "")
	if !strings.Contains(string(body), `id="inject-probe"`) || !strings.Contains(string(body), "</head>") {
		t.Fatalf("InjectHead not spliced before </head>: %s", body)
	}
}

func TestGzipCompression(t *testing.T) {
	ts := newServer(t, "") // no-auth so /api/issues is reachable
	req, _ := http.NewRequest("GET", ts.URL+"/api/issues", nil)
	req.Header.Set("Accept-Encoding", "gzip") // set manually so Go returns raw gzip
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip, got %q", resp.Header.Get("Content-Encoding"))
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("body is not valid gzip: %v", err)
	}
	body, _ := io.ReadAll(gz)
	if !strings.Contains(string(body), `"issues"`) {
		t.Fatalf("decompressed body wrong: %.60s", body)
	}
}

func TestSPAFallback(t *testing.T) {
	ts := newServer(t, "tok")
	// An HTML navigation to a client-side route serves the SPA shell (public),
	// so a deep-link refresh of /tasks/:id works.
	req, _ := http.NewRequest("GET", ts.URL+"/tasks/proj-abc", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `id="app"`) {
		t.Fatalf("SPA route should serve the shell: %d", resp.StatusCode)
	}
	// A JSON data request on the same-shaped path stays auth-gated (not the shell).
	req2, _ := http.NewRequest("GET", ts.URL+"/api/issues", nil)
	req2.Header.Set("Accept", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("data route must stay gated, got %d", resp2.StatusCode)
	}
}

func TestUIEndpoints(t *testing.T) {
	ts := newServer(t, "")
	// index
	code, body := do(t, ts, "GET", "/", "", "")
	if code != 200 || !strings.Contains(string(body), "<title>Tasks</title>") {
		t.Fatalf("index wrong: %d %s", code, firstLine(body))
	}
	// static asset — the Vite build references a hashed bundle from index.html.
	asset := regexp.MustCompile(`/static/assets/[^"']+\.js`).FindString(string(body))
	if asset == "" {
		t.Fatalf("no built asset referenced in index: %s", body)
	}
	if code, _ := do(t, ts, "GET", asset, "", ""); code != 200 {
		t.Fatalf("built asset %s = %d", asset, code)
	}
	// issues
	code, body = do(t, ts, "GET", "/api/issues", "", "")
	var issues struct {
		Issues []model.Task `json:"issues"`
		Count  int          `json:"count"`
		Mtime  float64      `json:"mtime"`
	}
	json.Unmarshal(body, &issues)
	if code != 200 || issues.Count != 0 {
		t.Fatalf("issues wrong: %d %+v", code, issues)
	}
	// meta
	if code, _ := do(t, ts, "GET", "/api/meta", "", ""); code != 200 {
		t.Fatalf("meta = %d", code)
	}
	// pull no-op
	code, body = do(t, ts, "POST", "/api/pull", "", "")
	if code != 200 || !strings.Contains(string(body), "ok") {
		t.Fatalf("pull = %d %s", code, body)
	}
}

func TestRESTFullFlow(t *testing.T) {
	ts := newServer(t, "tok")
	tok := "tok"

	// create
	code, body := do(t, ts, "POST", "/api/v1/tasks", tok, `{"title":"rest task","priority":1,"issue_type":"bug"}`)
	if code != http.StatusCreated {
		t.Fatalf("create = %d %s", code, body)
	}
	var created model.Task
	json.Unmarshal(body, &created)
	id := created.ID
	if id == "" || created.IssueType != "bug" {
		t.Fatalf("created wrong: %+v", created)
	}

	// validation error -> 400
	if code, _ := do(t, ts, "POST", "/api/v1/tasks", tok, `{"title":"x","issue_type":"bogus"}`); code != http.StatusBadRequest {
		t.Fatalf("bad type should be 400, got %d", code)
	}
	// missing required title -> 400
	if code, _ := do(t, ts, "POST", "/api/v1/tasks", tok, `{}`); code != http.StatusBadRequest {
		t.Fatalf("missing title should be 400, got %d", code)
	}
	// malformed JSON -> 400
	if code, _ := do(t, ts, "POST", "/api/v1/tasks", tok, `{bad`); code != http.StatusBadRequest {
		t.Fatalf("bad json should be 400, got %d", code)
	}

	// ready includes it
	code, body = do(t, ts, "GET", "/api/v1/ready?limit=50", tok, "")
	if code != 200 || !hasID(body, id) {
		t.Fatalf("ready missing %s: %s", id, body)
	}
	// list filter
	code, body = do(t, ts, "GET", "/api/v1/tasks?status=open&type=bug", tok, "")
	if code != 200 || !hasID(body, id) {
		t.Fatalf("list missing %s", id)
	}

	// get
	if code, _ := do(t, ts, "GET", "/api/v1/tasks/"+id, tok, ""); code != 200 {
		t.Fatalf("get = %d", code)
	}
	// get missing -> 404
	if code, _ := do(t, ts, "GET", "/api/v1/tasks/nope", tok, ""); code != http.StatusNotFound {
		t.Fatalf("get missing = %d", code)
	}

	// claim
	code, body = do(t, ts, "POST", "/api/v1/tasks/"+id+"/claim", tok, `{}`)
	var claimed model.Task
	json.Unmarshal(body, &claimed)
	if code != 200 || claimed.Status != "in_progress" {
		t.Fatalf("claim = %d %+v", code, claimed)
	}

	// update
	if code, _ := do(t, ts, "PATCH", "/api/v1/tasks/"+id, tok, `{"priority":0}`); code != 200 {
		t.Fatalf("update = %d", code)
	}

	// comment
	if code, _ := do(t, ts, "POST", "/api/v1/tasks/"+id+"/comments", tok, `{"text":"hi"}`); code != 200 {
		t.Fatalf("comment = %d", code)
	}

	// second task + dep
	_, body2 := do(t, ts, "POST", "/api/v1/tasks", tok, `{"title":"downstream"}`)
	var t2 model.Task
	json.Unmarshal(body2, &t2)
	if code, _ := do(t, ts, "POST", "/api/v1/deps", tok, `{"blocked":"`+t2.ID+`","blocker":"`+id+`"}`); code != 200 {
		t.Fatalf("dep = %d", code)
	}
	// dep to missing -> 400
	if code, _ := do(t, ts, "POST", "/api/v1/deps", tok, `{"blocked":"nope","blocker":"nope2"}`); code != http.StatusBadRequest {
		t.Fatalf("bad dep = %d", code)
	}

	// close
	code, body = do(t, ts, "POST", "/api/v1/tasks/"+id+"/close", tok, `{"reason":"done"}`)
	var closed model.Task
	json.Unmarshal(body, &closed)
	if code != 200 || closed.Status != "closed" || closed.CloseReason != "done" {
		t.Fatalf("close = %d %+v", code, closed)
	}
}

// ---- helpers ----

func hasID(body []byte, id string) bool {
	var ts []model.Task
	if json.Unmarshal(body, &ts) != nil {
		return false
	}
	for _, t := range ts {
		if t.ID == id {
			return true
		}
	}
	return false
}

func firstLine(b []byte) string {
	if i := strings.IndexByte(string(b), '\n'); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}

func TestPublicUIAndLoginFlow(t *testing.T) {
	ts := newServer(t, "tok")
	// UI assets are PUBLIC (browser must load them to render the login screen,
	// which the SPA renders client-side at #app).
	code0, body0 := do(t, ts, "GET", "/", "", "")
	if code0 != 200 || !strings.Contains(string(body0), `id="app"`) {
		t.Fatalf("index should be public (SPA root): %d", code0)
	}
	asset := regexp.MustCompile(`/static/assets/[^"']+\.js`).FindString(string(body0))
	if asset == "" {
		t.Fatalf("no built asset referenced in index")
	}
	if code, _ := do(t, ts, "GET", asset, "", ""); code != 200 {
		t.Fatalf("static must be public: %d", code)
	}
	// authinfo is public and reports the mode.
	code, body := do(t, ts, "GET", "/api/authinfo", "", "")
	if code != 200 || !strings.Contains(string(body), `"mode":"token"`) || !strings.Contains(string(body), `"authenticated":false`) {
		t.Fatalf("authinfo: %d %s", code, body)
	}
	// data is gated.
	if code, _ := do(t, ts, "GET", "/api/issues", "", ""); code != 401 {
		t.Fatalf("issues must be gated: %d", code)
	}
	// wrong token login -> 401
	if code, _ := do(t, ts, "POST", "/api/login", "", `{"token":"nope"}`); code != 401 {
		t.Fatalf("bad login should be 401: %d", code)
	}

	// cookie-jar client: login, then use the session cookie.
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp, err := client.Post(ts.URL+"/api/login", "application/json", strings.NewReader(`{"token":"tok"}`))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("login failed: %v %v", err, resp.StatusCode)
	}
	resp.Body.Close()
	// now /api/issues works via the cookie
	r2, _ := client.Get(ts.URL + "/api/issues")
	if r2.StatusCode != 200 {
		t.Fatalf("cookie-authed issues: %d", r2.StatusCode)
	}
	r2.Body.Close()
	// logout clears it
	rl, _ := client.Post(ts.URL+"/api/logout", "", nil)
	rl.Body.Close()
	r3, _ := client.Get(ts.URL + "/api/issues")
	if r3.StatusCode != 401 {
		t.Fatalf("after logout issues should be 401: %d", r3.StatusCode)
	}
	r3.Body.Close()
}

func TestAuthInfoNoAuthMode(t *testing.T) {
	ts := newServer(t, "") // no token -> noAuth
	_, body := do(t, ts, "GET", "/api/authinfo", "", "")
	if !strings.Contains(string(body), `"mode":"none"`) {
		t.Fatalf("expected mode none: %s", body)
	}
}
