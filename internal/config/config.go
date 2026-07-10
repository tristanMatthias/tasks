// Package config resolves the tasksd server configuration from defaults,
// environment variables (TASKS_*), and command-line flags — in that order of
// increasing precedence (flags override env override defaults). It is pure and
// testable: Load takes the args slice and a getenv function.
package config

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every tunable for the server.
type Config struct {
	Addr   string // listen address
	DB     string // SQLite path
	Prefix string // issue id prefix (derived from data if empty)
	Actor  string // default actor for audit fields

	Token       string // bearer token (empty = no auth)
	TokenFile   string // read token from this file (overrides Token if set)
	AllowNoAuth bool   // permit running without a token on a non-loopback bind

	Import         string        // import this JSONL on first run (empty db)
	Export         string        // mirror the store to this JSONL on change
	ExportInterval time.Duration // debounce window for exports
	Git            bool          // git add+commit the export
	GitPush        bool          // also git push

	LogLevel  string // debug|info|warn|error
	LogFormat string // text|json

	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration

	MaxBodyBytes int64   // request body cap (bytes)
	RateLimit    float64 // per-IP requests/sec (0 = disabled)
	RateBurst    int     // per-IP burst
	BehindProxy  bool    // trust X-Forwarded-For for client IP
	CORSOrigins  []string
	Metrics      bool // expose /metrics
}

// Default returns the baseline configuration.
func Default() Config {
	return Config{
		Addr:            "127.0.0.1:7842",
		DB:              "data/tasks.db",
		Actor:           "agent",
		ExportInterval:  15 * time.Second,
		LogLevel:        "info",
		LogFormat:       "text",
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		MaxBodyBytes:    1 << 20, // 1 MiB
		RateLimit:       20,
		RateBurst:       40,
		Metrics:         true,
	}
}

// Load builds a Config from defaults, env (via getenv), then flags (args).
// stderr receives flag usage on -h. Returns flag.ErrHelp when -h/--help used.
func Load(args []string, getenv func(string) string, stderr io.Writer) (Config, error) {
	c := Default()
	c.fromEnv(getenv)

	fs := flag.NewFlagSet("tasksd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&c.Addr, "addr", c.Addr, "listen address (TASKS_ADDR)")
	fs.StringVar(&c.DB, "db", c.DB, "SQLite database path (TASKS_DB)")
	fs.StringVar(&c.Prefix, "prefix", c.Prefix, "issue id prefix; derived from data if empty (TASKS_PREFIX)")
	fs.StringVar(&c.Actor, "actor", c.Actor, "default actor for audit fields (TASKS_ACTOR)")
	fs.StringVar(&c.Token, "token", c.Token, "bearer token; empty disables auth (TASKS_TOKEN)")
	fs.StringVar(&c.TokenFile, "token-file", c.TokenFile, "read bearer token from this file (TASKS_TOKEN_FILE)")
	fs.BoolVar(&c.AllowNoAuth, "allow-no-auth", c.AllowNoAuth, "allow no-auth on a public bind (TASKS_ALLOW_NO_AUTH)")
	fs.StringVar(&c.Import, "import", c.Import, "import this issues.jsonl if the db is empty (TASKS_IMPORT)")
	fs.StringVar(&c.Export, "export", c.Export, "mirror the store to this issues.jsonl on change (TASKS_EXPORT)")
	fs.DurationVar(&c.ExportInterval, "export-interval", c.ExportInterval, "export debounce window (TASKS_EXPORT_INTERVAL)")
	fs.BoolVar(&c.Git, "git", c.Git, "git add+commit the export (TASKS_GIT)")
	fs.BoolVar(&c.GitPush, "git-push", c.GitPush, "also git push after commit (TASKS_GIT_PUSH)")
	fs.StringVar(&c.LogLevel, "log-level", c.LogLevel, "log level: debug|info|warn|error (TASKS_LOG_LEVEL)")
	fs.StringVar(&c.LogFormat, "log-format", c.LogFormat, "log format: text|json (TASKS_LOG_FORMAT)")
	fs.DurationVar(&c.ReadTimeout, "read-timeout", c.ReadTimeout, "HTTP read timeout (TASKS_READ_TIMEOUT)")
	fs.DurationVar(&c.WriteTimeout, "write-timeout", c.WriteTimeout, "HTTP write timeout (TASKS_WRITE_TIMEOUT)")
	fs.DurationVar(&c.IdleTimeout, "idle-timeout", c.IdleTimeout, "HTTP idle timeout (TASKS_IDLE_TIMEOUT)")
	fs.DurationVar(&c.ShutdownTimeout, "shutdown-timeout", c.ShutdownTimeout, "graceful shutdown timeout (TASKS_SHUTDOWN_TIMEOUT)")
	fs.Int64Var(&c.MaxBodyBytes, "max-body-bytes", c.MaxBodyBytes, "max request body size in bytes (TASKS_MAX_BODY_BYTES)")
	fs.Float64Var(&c.RateLimit, "rate-limit", c.RateLimit, "per-IP requests/sec; 0 disables (TASKS_RATE_LIMIT)")
	fs.IntVar(&c.RateBurst, "rate-burst", c.RateBurst, "per-IP rate-limit burst (TASKS_RATE_BURST)")
	fs.BoolVar(&c.BehindProxy, "behind-proxy", c.BehindProxy, "trust X-Forwarded-For for client IP (TASKS_BEHIND_PROXY)")
	fs.BoolVar(&c.Metrics, "metrics", c.Metrics, "expose /metrics (TASKS_METRICS)")
	var cors string
	fs.StringVar(&cors, "cors-origins", strings.Join(c.CORSOrigins, ","), "comma-separated allowed CORS origins (TASKS_CORS_ORIGINS)")

	if err := fs.Parse(args); err != nil {
		return c, err
	}
	c.CORSOrigins = splitCSV(cors)

	if err := c.resolveToken(); err != nil {
		return c, err
	}
	if err := c.Validate(); err != nil {
		return c, err
	}
	return c, nil
}

// fromEnv overlays environment variables onto c.
func (c *Config) fromEnv(getenv func(string) string) {
	str := func(k string, p *string) {
		if v := getenv(k); v != "" {
			*p = v
		}
	}
	boolean := func(k string, p *bool) {
		if v := getenv(k); v != "" {
			if b, err := strconv.ParseBool(v); err == nil {
				*p = b
			}
		}
	}
	dur := func(k string, p *time.Duration) {
		if v := getenv(k); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				*p = d
			}
		}
	}
	str("TASKS_ADDR", &c.Addr)
	// PaaS platforms (Render/Fly/Railway/…) inject $PORT; bind all interfaces to
	// it unless TASKS_ADDR was set explicitly.
	if getenv("TASKS_ADDR") == "" {
		if port := getenv("PORT"); port != "" {
			c.Addr = "0.0.0.0:" + port
		}
	}
	str("TASKS_DB", &c.DB)
	str("TASKS_PREFIX", &c.Prefix)
	str("TASKS_ACTOR", &c.Actor)
	str("TASKS_TOKEN", &c.Token)
	str("TASKS_TOKEN_FILE", &c.TokenFile)
	boolean("TASKS_ALLOW_NO_AUTH", &c.AllowNoAuth)
	str("TASKS_IMPORT", &c.Import)
	str("TASKS_EXPORT", &c.Export)
	dur("TASKS_EXPORT_INTERVAL", &c.ExportInterval)
	boolean("TASKS_GIT", &c.Git)
	boolean("TASKS_GIT_PUSH", &c.GitPush)
	str("TASKS_LOG_LEVEL", &c.LogLevel)
	str("TASKS_LOG_FORMAT", &c.LogFormat)
	dur("TASKS_READ_TIMEOUT", &c.ReadTimeout)
	dur("TASKS_WRITE_TIMEOUT", &c.WriteTimeout)
	dur("TASKS_IDLE_TIMEOUT", &c.IdleTimeout)
	dur("TASKS_SHUTDOWN_TIMEOUT", &c.ShutdownTimeout)
	if v := getenv("TASKS_MAX_BODY_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			c.MaxBodyBytes = n
		}
	}
	if v := getenv("TASKS_RATE_LIMIT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.RateLimit = f
		}
	}
	if v := getenv("TASKS_RATE_BURST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.RateBurst = n
		}
	}
	boolean("TASKS_BEHIND_PROXY", &c.BehindProxy)
	boolean("TASKS_METRICS", &c.Metrics)
	if v := getenv("TASKS_CORS_ORIGINS"); v != "" {
		c.CORSOrigins = splitCSV(v)
	}
}

// resolveToken loads the token from TokenFile when Token is unset.
func (c *Config) resolveToken() error {
	if c.Token == "" && c.TokenFile != "" {
		b, err := os.ReadFile(c.TokenFile)
		if err != nil {
			return fmt.Errorf("read token file: %w", err)
		}
		c.Token = strings.TrimSpace(string(b))
	}
	return nil
}

// Validate checks the resolved config for safety and correctness.
func (c *Config) Validate() error {
	if _, err := c.SlogLevel(); err != nil {
		return err
	}
	switch c.LogFormat {
	case "text", "json":
	default:
		return fmt.Errorf("invalid log-format %q (want text|json)", c.LogFormat)
	}
	if c.MaxBodyBytes <= 0 {
		return fmt.Errorf("max-body-bytes must be > 0")
	}
	// Fail-safe: refuse to serve unauthenticated on a public interface.
	if c.Token == "" && !c.AllowNoAuth && !isLoopbackAddr(c.Addr) {
		return fmt.Errorf("refusing to start without a token on a public bind (%s); set TASKS_TOKEN, or pass --allow-no-auth to override", c.Addr)
	}
	return nil
}

// SlogLevel maps LogLevel to a slog.Level.
func (c *Config) SlogLevel() (slog.Level, error) {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return slog.LevelInfo, fmt.Errorf("invalid log-level %q", c.LogLevel)
}

// NewLogger builds a slog.Logger per the config, writing to w.
func (c *Config) NewLogger(w io.Writer) *slog.Logger {
	lvl, _ := c.SlogLevel()
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if c.LogFormat == "json" {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h)
}

// AuthEnabled reports whether a token is configured.
func (c *Config) AuthEnabled() bool { return c.Token != "" }

func isLoopbackAddr(addr string) bool {
	host := addr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host = addr[:i]
	}
	host = strings.Trim(host, "[]")
	switch host {
	case "", "127.0.0.1", "localhost", "::1":
		return true
	}
	return strings.HasPrefix(host, "127.")
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
