package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmpSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.2.3", "v1.2.3", 0},
		{"v1.2.4", "v1.2.3", 1},
		{"v1.3.0", "v1.2.9", 1},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.2.3", "v1.2.4", -1},
		{"1.2.3", "v1.2.3", 0},       // 'v' optional
		{"v1.2.3-rc.1", "v1.2.3", 0}, // prerelease suffix ignored
		{"v1.2.0", "v1.2", 0},        // missing patch = 0
	}
	for _, c := range cases {
		if got := cmpSemver(c.a, c.b); got != c.want {
			t.Errorf("cmpSemver(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestChecksumFor(t *testing.T) {
	sums := "abc123  tasks_linux_amd64\ndef456  tasks_darwin_arm64\n"
	if got := checksumFor(sums, "tasks_darwin_arm64"); got != "def456" {
		t.Fatalf("checksumFor = %q", got)
	}
	if got := checksumFor(sums, "tasks_windows_amd64"); got != "" {
		t.Fatalf("expected empty for missing, got %q", got)
	}
}

func TestFindAsset(t *testing.T) {
	r := &ghRelease{}
	r.Assets = append(r.Assets, struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}{Name: "tasks_linux_amd64", URL: "http://x/bin"})
	if findAsset(r, "tasks_linux_amd64") != "http://x/bin" {
		t.Fatal("should find asset")
	}
	if findAsset(r, "nope") != "" {
		t.Fatal("should return empty for missing")
	}
}

func TestReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "tasks")
	if err := os.WriteFile(exe, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceExecutable(exe, []byte("NEW-BINARY")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, _ := os.ReadFile(exe)
	if string(got) != "NEW-BINARY" {
		t.Fatalf("content = %q, want NEW-BINARY", got)
	}
	if fi, _ := os.Stat(exe); fi.Mode().Perm()&0o100 == 0 {
		t.Fatal("replaced binary should be executable")
	}
	// No leftover temp files in the dir.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected only the binary, got %d entries", len(entries))
	}
}

// fakeRelease spins an httptest server that serves a "latest release" + assets,
// with a correct checksums.txt over the fake binary.
func fakeRelease(t *testing.T, tag string, binary []byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	asset := fmt.Sprintf("tasks_%s_%s", "testos", "testarch")
	sum := fmt.Sprintf("%x  %s\n", sha256.Sum256(binary), asset)
	mux.HandleFunc("/dl/bin", func(w http.ResponseWriter, r *http.Request) { w.Write(binary) })
	mux.HandleFunc("/dl/sums", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(sum)) })
	var srv *httptest.Server
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		rel := ghRelease{TagName: tag, HTMLURL: "https://example/releases/" + tag}
		rel.Assets = []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: asset, URL: srv.URL + "/dl/bin"},
			{Name: "checksums.txt", URL: srv.URL + "/dl/sums"},
		}
		json.NewEncoder(w).Encode(rel)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestUpdater(t *testing.T, srv *httptest.Server, current string) (*updater, string) {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "tasks")
	os.WriteFile(exe, []byte("OLD"), 0o755)
	return &updater{
		repo: "x/y", apiBase: srv.URL, current: current,
		goos: "testos", goarch: "testarch", exePath: exe, client: srv.Client(),
	}, exe
}

func TestUpdaterUpdatesToNewer(t *testing.T) {
	srv := fakeRelease(t, "v9.9.9", []byte("FRESH-BINARY"))
	u, exe := newTestUpdater(t, srv, "v1.0.0")
	var out strings.Builder
	if err := u.run(false, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "FRESH-BINARY" {
		t.Fatalf("binary not replaced: %q", got)
	}
	if !strings.Contains(out.String(), "updated v1.0.0 → v9.9.9") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestUpdaterSkipsWhenCurrent(t *testing.T) {
	srv := fakeRelease(t, "v1.0.0", []byte("SAME"))
	u, exe := newTestUpdater(t, srv, "v1.0.0")
	var out strings.Builder
	if err := u.run(false, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "OLD" {
		t.Fatal("binary should be untouched when already current")
	}
	if !strings.Contains(out.String(), "already on the latest") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestUpdaterForceReinstalls(t *testing.T) {
	srv := fakeRelease(t, "v1.0.0", []byte("REINSTALLED"))
	u, exe := newTestUpdater(t, srv, "v1.0.0")
	if err := u.run(true, &strings.Builder{}); err != nil {
		t.Fatalf("run --force: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "REINSTALLED" {
		t.Fatal("--force should reinstall even the same version")
	}
}

func TestUpdaterChecksumMismatch(t *testing.T) {
	srv := fakeRelease(t, "v9.9.9", []byte("GOOD"))
	// Corrupt the checksums endpoint so verification fails.
	u, exe := newTestUpdater(t, srv, "v1.0.0")
	u.apiBase = srv.URL // (already)
	// Point at a server whose checksum won't match by swapping the binary served.
	bad := fakeReleaseBadSum(t, "v9.9.9")
	u.apiBase = bad.URL
	if err := u.run(false, &strings.Builder{}); err == nil {
		t.Fatal("expected a checksum mismatch error")
	}
	if got, _ := os.ReadFile(exe); string(got) != "OLD" {
		t.Fatal("binary must not be replaced on checksum mismatch")
	}
}

func TestNewUpdaterAndHelpers(t *testing.T) {
	u, err := newUpdater()
	if err != nil {
		t.Fatalf("newUpdater: %v", err)
	}
	if u.exePath == "" || u.apiBase == "" || u.repo == "" {
		t.Fatalf("updater not populated: %+v", u)
	}
	if u.assetName() != "tasks_"+u.goos+"_"+u.goarch {
		t.Fatalf("assetName = %q", u.assetName())
	}
	if orDev("") != "dev" || orDev("v1") != "v1" {
		t.Fatal("orDev")
	}
}

func TestParseUpdateArgs(t *testing.T) {
	var out strings.Builder
	if f, done, err := parseUpdateArgs(nil, &out); f || done || err != nil {
		t.Fatalf("no args → (%v,%v,%v)", f, done, err)
	}
	if f, done, err := parseUpdateArgs([]string{"--force"}, &out); !f || done || err != nil {
		t.Fatalf("--force → (%v,%v,%v)", f, done, err)
	}
	if f, done, err := parseUpdateArgs([]string{"-h"}, &out); f || !done || err != nil {
		t.Fatalf("--help → (%v,%v,%v)", f, done, err)
	}
	if _, _, err := parseUpdateArgs([]string{"--bogus"}, &out); err == nil {
		t.Fatal("unknown flag should error")
	}
}

func TestRunSelfUpdateEarlyReturns(t *testing.T) {
	// --help and an unknown flag both return before any network call.
	if err := runSelfUpdate([]string{"--help"}); err != nil {
		t.Fatalf("--help should return nil, got %v", err)
	}
	if err := runSelfUpdate([]string{"--bogus"}); err == nil {
		t.Fatal("unknown flag should error")
	}
}

func TestUpdaterDownloadErrorMidRun(t *testing.T) {
	// Release advertises an asset whose download 404s → run fails, binary intact.
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		rel := ghRelease{TagName: "v9.9.9"}
		rel.Assets = []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{{Name: "tasks_testos_testarch", URL: srv.URL + "/missing"}}
		json.NewEncoder(w).Encode(rel)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, exe := newTestUpdater(t, srv, "v1.0.0")
	if err := u.run(false, &strings.Builder{}); err == nil {
		t.Fatal("a 404 asset download should error")
	}
	if got, _ := os.ReadFile(exe); string(got) != "OLD" {
		t.Fatal("binary must be untouched on download failure")
	}
}

func TestUpdaterChecksumsDownloadError(t *testing.T) {
	// The binary downloads fine, but checksums.txt 404s → run errors, no swap.
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/dl/bin", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("bin")) })
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		rel := ghRelease{TagName: "v9.9.9"}
		rel.Assets = []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "tasks_testos_testarch", URL: srv.URL + "/dl/bin"},
			{Name: "checksums.txt", URL: srv.URL + "/dl/missing"}, // 404
		}
		json.NewEncoder(w).Encode(rel)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, exe := newTestUpdater(t, srv, "v1.0.0")
	if err := u.run(false, &strings.Builder{}); err == nil {
		t.Fatal("a failed checksums download should error")
	}
	if got, _ := os.ReadFile(exe); string(got) != "OLD" {
		t.Fatal("binary must be untouched when checksums can't be fetched")
	}
}

func TestUpdaterWindowsUnsupported(t *testing.T) {
	u := &updater{repo: "x/y", goos: "windows", goarch: "amd64"}
	if err := u.run(false, &strings.Builder{}); err == nil {
		t.Fatal("windows should be unsupported")
	}
}

func TestUpdaterNoAssetForPlatform(t *testing.T) {
	srv := fakeRelease(t, "v9.9.9", []byte("x"))
	u, _ := newTestUpdater(t, srv, "v1.0.0")
	u.goarch = "sparc" // no such asset in the release
	if err := u.run(false, &strings.Builder{}); err == nil {
		t.Fatal("expected a 'no asset' error")
	}
}

func TestUpdaterEmptyTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":""}`))
	}))
	t.Cleanup(srv.Close)
	u, _ := newTestUpdater(t, srv, "v1.0.0")
	if err := u.run(false, &strings.Builder{}); err == nil {
		t.Fatal("empty tag should error")
	}
}

func TestLatestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	u, _ := newTestUpdater(t, srv, "v1.0.0")
	if _, err := u.latest(); err == nil {
		t.Fatal("HTTP 500 should error")
	}
}

func TestDownloadHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	u, _ := newTestUpdater(t, srv, "v1.0.0")
	if _, err := u.download(srv.URL + "/missing"); err == nil {
		t.Fatal("404 download should error")
	}
}

func TestReplaceExecutableBadDir(t *testing.T) {
	// A path whose directory doesn't exist → CreateTemp fails.
	bad := filepath.Join(t.TempDir(), "nope", "tasks")
	if err := replaceExecutable(bad, []byte("x")); err == nil {
		t.Fatal("expected an error writing into a missing directory")
	}
}

func TestReplaceExecutableRenameError(t *testing.T) {
	// Target is an existing directory → the final rename fails.
	dir := t.TempDir()
	target := filepath.Join(dir, "adir")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceExecutable(target, []byte("x")); err == nil {
		t.Fatal("renaming a file over a directory should error")
	}
}

func TestLatestInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	t.Cleanup(srv.Close)
	u, _ := newTestUpdater(t, srv, "v1.0.0")
	if _, err := u.latest(); err == nil {
		t.Fatal("invalid JSON should error")
	}
}

// A release with no checksums.txt still updates (verification is best-effort).
func TestUpdaterNoChecksums(t *testing.T) {
	bin := []byte("NO-SUMS-BINARY")
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/dl/bin", func(w http.ResponseWriter, r *http.Request) { w.Write(bin) })
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		rel := ghRelease{TagName: "v9.9.9"}
		rel.Assets = []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{{Name: "tasks_testos_testarch", URL: srv.URL + "/dl/bin"}}
		json.NewEncoder(w).Encode(rel)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, exe := newTestUpdater(t, srv, "v1.0.0")
	if err := u.run(false, &strings.Builder{}); err != nil {
		t.Fatalf("run without checksums: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "NO-SUMS-BINARY" {
		t.Fatal("should update even without a checksums file")
	}
}

// A checksums.txt that doesn't list our asset is a hard error (no silent skip).
func TestUpdaterChecksumNotListed(t *testing.T) {
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/dl/bin", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("bin")) })
	mux.HandleFunc("/dl/sums", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("abc  some_other_asset\n"))
	})
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		rel := ghRelease{TagName: "v9.9.9"}
		rel.Assets = []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "tasks_testos_testarch", URL: srv.URL + "/dl/bin"},
			{Name: "checksums.txt", URL: srv.URL + "/dl/sums"},
		}
		json.NewEncoder(w).Encode(rel)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, _ := newTestUpdater(t, srv, "v1.0.0")
	if err := u.run(false, &strings.Builder{}); err == nil {
		t.Fatal("missing checksum entry should error")
	}
}

func fakeReleaseBadSum(t *testing.T, tag string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	asset := "tasks_testos_testarch"
	mux.HandleFunc("/dl/bin", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("REAL")) })
	mux.HandleFunc("/dl/sums", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("deadbeef  " + asset + "\n")) // wrong digest
	})
	var srv *httptest.Server
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		rel := ghRelease{TagName: tag}
		rel.Assets = []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: asset, URL: srv.URL + "/dl/bin"},
			{Name: "checksums.txt", URL: srv.URL + "/dl/sums"},
		}
		json.NewEncoder(w).Encode(rel)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}
