package config

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func noEnv(string) string { return "" }

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDefaults(t *testing.T) {
	c, err := Load(nil, noEnv, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != "127.0.0.1:7842" || c.MaxBodyBytes != 1<<20 || c.RateLimit != 20 || !c.Metrics {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c.AuthEnabled() {
		t.Error("no token -> auth disabled")
	}
}

func TestEnvOverrides(t *testing.T) {
	env := envMap(map[string]string{
		"TASKS_ADDR":            "127.0.0.1:9",
		"TASKS_TOKEN":           "envtok",
		"TASKS_LOG_FORMAT":      "json",
		"TASKS_LOG_LEVEL":       "debug",
		"TASKS_RATE_LIMIT":      "3.5",
		"TASKS_RATE_BURST":      "7",
		"TASKS_MAX_BODY_BYTES":  "2048",
		"TASKS_EXPORT_INTERVAL": "30s",
		"TASKS_GIT":             "true",
		"TASKS_METRICS":         "false",
		"TASKS_CORS_ORIGINS":    "https://a.com, https://b.com",
		"TASKS_BEHIND_PROXY":    "true",
	})
	c, err := Load(nil, env, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	if c.Token != "envtok" || c.LogFormat != "json" || c.LogLevel != "debug" ||
		c.RateLimit != 3.5 || c.RateBurst != 7 || c.MaxBodyBytes != 2048 ||
		c.ExportInterval != 30*time.Second || !c.Git || c.Metrics || !c.BehindProxy {
		t.Fatalf("env not applied: %+v", c)
	}
	if len(c.CORSOrigins) != 2 || c.CORSOrigins[0] != "https://a.com" {
		t.Fatalf("cors origins: %v", c.CORSOrigins)
	}
	if !c.AuthEnabled() {
		t.Error("token set -> auth enabled")
	}
}

func TestFlagsOverrideEnv(t *testing.T) {
	env := envMap(map[string]string{"TASKS_ADDR": "127.0.0.1:1", "TASKS_TOKEN": "envtok"})
	c, err := Load([]string{"--addr", "127.0.0.1:2", "--token", "flagtok"}, env, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != "127.0.0.1:2" || c.Token != "flagtok" {
		t.Fatalf("flags did not override env: %+v", c)
	}
}

func TestTokenFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "token")
	os.WriteFile(f, []byte("  filetoken\n"), 0o600)
	c, err := Load([]string{"--token-file", f}, noEnv, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	if c.Token != "filetoken" {
		t.Fatalf("token from file = %q", c.Token)
	}
	// missing token file -> error
	if _, err := Load([]string{"--token-file", "/no/such/file"}, noEnv, os.Stderr); err == nil {
		t.Fatal("expected token-file read error")
	}
}

func TestValidation(t *testing.T) {
	// public bind without token -> refuse
	if _, err := Load([]string{"--addr", "0.0.0.0:80"}, noEnv, os.Stderr); err == nil {
		t.Fatal("expected public-bind fail-safe")
	}
	// override with allow-no-auth
	if _, err := Load([]string{"--addr", "0.0.0.0:80", "--allow-no-auth"}, noEnv, os.Stderr); err != nil {
		t.Fatalf("allow-no-auth should permit: %v", err)
	}
	// bad log level
	if _, err := Load([]string{"--log-level", "loud"}, noEnv, os.Stderr); err == nil {
		t.Fatal("expected bad log-level error")
	}
	// bad log format
	if _, err := Load([]string{"--log-format", "xml"}, noEnv, os.Stderr); err == nil {
		t.Fatal("expected bad log-format error")
	}
	// bad max body
	if _, err := Load([]string{"--max-body-bytes", "0"}, noEnv, os.Stderr); err == nil {
		t.Fatal("expected max-body error")
	}
}

func TestHelp(t *testing.T) {
	var buf bytes.Buffer
	_, err := Load([]string{"-h"}, noEnv, &buf)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	// bad flag surfaces an error too
	if _, err := Load([]string{"--nonsense"}, noEnv, &buf); err == nil {
		t.Fatal("expected bad-flag error")
	}
}

func TestSlogLevelAndLogger(t *testing.T) {
	for _, lvl := range []string{"debug", "info", "warn", "warning", "error"} {
		c := Config{LogLevel: lvl, LogFormat: "text", MaxBodyBytes: 1}
		if _, err := c.SlogLevel(); err != nil {
			t.Errorf("level %q: %v", lvl, err)
		}
	}
	c := Config{LogLevel: "info", LogFormat: "json"}
	if c.NewLogger(&bytes.Buffer{}) == nil {
		t.Error("nil logger")
	}
	c.LogFormat = "text"
	if c.NewLogger(&bytes.Buffer{}) == nil {
		t.Error("nil text logger")
	}
}

func TestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:80": true, "localhost:1": true, "[::1]:1": true, "127.5.5.5:9": true,
		":8080": true, "0.0.0.0:80": false, "192.168.1.5:80": false,
	}
	for addr, want := range cases {
		if got := isLoopbackAddr(addr); got != want {
			t.Errorf("isLoopbackAddr(%q) = %v, want %v", addr, got, want)
		}
	}
}
