// Package httpapi serves the browser UI (byte-compatible with the old Python
// beads_ui endpoints), a versioned REST API, and mounts the MCP handler. App
// routes are guarded by bearer-token / cookie auth; operational endpoints
// (/healthz, /readyz, /version, /metrics) are public. A middleware chain adds
// panic recovery, request IDs, access logs, security headers, CORS, body-size
// limits and per-IP rate limiting.
package httpapi

import (
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/tristanMatthias/tasks/internal/api"
	"github.com/tristanMatthias/tasks/internal/core"
)

// Server is the HTTP surface over a Core.
type Server struct {
	core   *core.Core
	static fs.FS
	token  string
	logger *slog.Logger
	mcp    http.Handler // optional; mounted at /mcp when non-nil

	maxBody     int64
	corsOrigins []string
	behindProxy bool
	metricsOn   bool

	rl      *rateLimiter // nil when disabled
	metrics *metrics

	mu         sync.RWMutex
	lastChange time.Time // updated on every successful mutation (drives UI mtime)
}

// Config configures the server.
type Config struct {
	Core   *core.Core
	Static fs.FS
	Token  string
	MCP    http.Handler // optional MCP handler
	Logger *slog.Logger // defaults to slog.Default()

	MaxBodyBytes int64    // request body cap (<=0 -> 1 MiB)
	RateLimit    float64  // per-IP req/s (0 -> disabled)
	RateBurst    int      // per-IP burst
	BehindProxy  bool     // trust X-Forwarded-For
	CORSOrigins  []string // allowed CORS origins
	Metrics      bool     // expose /metrics
}

// New builds a Server.
func New(cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	maxBody := cfg.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = 1 << 20
	}
	now := time.Now()
	s := &Server{
		core:        cfg.Core,
		static:      cfg.Static,
		token:       cfg.Token,
		logger:      logger,
		mcp:         cfg.MCP,
		maxBody:     maxBody,
		corsOrigins: cfg.CORSOrigins,
		behindProxy: cfg.BehindProxy,
		metricsOn:   cfg.Metrics,
		metrics:     newMetrics(now),
		lastChange:  now,
	}
	if cfg.RateLimit > 0 {
		burst := cfg.RateBurst
		if burst <= 0 {
			burst = 1
		}
		s.rl = newRateLimiter(cfg.RateLimit, burst, cfg.BehindProxy, now)
	}
	return s
}

// Touch advances the UI's mtime freshness signal. Register it as a core
// onChange hook so any mutation (HTTP, MCP or CLI) refreshes it.
func (s *Server) Touch() {
	s.mu.Lock()
	s.lastChange = time.Now()
	s.mu.Unlock()
}

func (s *Server) mtime() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return float64(s.lastChange.UnixNano()) / 1e9
}

// Handler returns the fully-wired http.Handler (middleware + auth + routes).
func (s *Server) Handler() http.Handler {
	// App mux: everything that requires auth.
	app := http.NewServeMux()
	staticFS := http.FileServer(http.FS(s.static))
	serveIndex := func(w http.ResponseWriter, r *http.Request) {
		s.serveStatic(w, r, "index.html", "text/html; charset=utf-8")
	}
	app.HandleFunc("GET /{$}", serveIndex) // exact root only
	app.HandleFunc("GET /index.html", serveIndex)
	app.Handle("GET /static/", http.StripPrefix("/static/", staticFS))
	app.HandleFunc("GET /api/issues", s.handleIssues)
	app.HandleFunc("GET /api/meta", s.handleMeta)
	app.HandleFunc("POST /api/pull", s.handlePull)
	for _, op := range api.Ops() {
		app.HandleFunc(op.Method+" "+op.Path, s.opHandler(op))
	}
	if s.mcp != nil {
		app.Handle("/mcp", s.mcp)
		app.Handle("/mcp/", s.mcp)
	}

	// Root mux: public operational endpoints + the auth-wrapped app.
	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", s.handleHealthz)
	root.HandleFunc("GET /readyz", s.handleReadyz)
	root.HandleFunc("GET /version", s.handleVersion)
	if s.metricsOn {
		root.Handle("GET /metrics", s.metrics.handler())
	}
	root.Handle("/", s.auth(app))

	// Global middleware chain (outermost first).
	var h http.Handler = root
	h = bodyLimit(s.maxBody, h)
	if s.rl != nil {
		h = s.rl.middleware(h)
	}
	if len(s.corsOrigins) > 0 {
		h = cors(s.corsOrigins, h)
	}
	h = securityHeaders(h)
	h = s.metrics.middleware(h)
	h = accessLog(s.logger, s.behindProxy, h)
	h = requestIDMiddleware(h)
	h = recoverer(s.logger, h)
	return h
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request, name, ctype string) {
	data, err := fs.ReadFile(s.static, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}
