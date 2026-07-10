// Package httpapi serves the browser UI (byte-compatible with the old Python
// beads_ui endpoints), a versioned REST API, and mounts the MCP handler. App
// routes are guarded by a pluggable Authenticator; operational endpoints
// (/healthz, /readyz, /version, /metrics) and the login endpoints are public.
//
// Two seams make tasksd embeddable by a larger host (e.g. a multi-tenant control
// plane) WITHOUT tasksd knowing anything about tenants/SaaS:
//   - Config.Auth  (Authenticator)  — plug in custom request authorization.
//   - Config.Resolve (CoreResolver) — choose the *core.Core (i.e. the DB) per
//     request; the default just returns the single global core.
package httpapi

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/tristanMatthias/tasks/pkg/api"
	"github.com/tristanMatthias/tasks/pkg/core"
)

// CoreResolver selects the Core (DB) to serve a request. Return an error to
// reject (e.g. unknown tenant). nil resolver -> the single global core is used.
type CoreResolver func(r *http.Request) (*core.Core, error)

// Server is the HTTP surface.
type Server struct {
	primaryCore *core.Core   // the single/default core; may be nil in a pure multi-core host
	resolve     CoreResolver // optional per-request core selection
	auth        Authenticator
	static      fs.FS
	logger      *slog.Logger
	mcp         http.Handler

	maxBody     int64
	corsOrigins []string
	behindProxy bool
	metricsOn   bool

	rl      *rateLimiter
	metrics *metrics

	mu         sync.RWMutex
	lastChange time.Time
}

// Config configures the server.
type Config struct {
	Core   *core.Core // the single/default core (nil allowed if Resolve is set)
	Static fs.FS
	Logger *slog.Logger

	// Auth authorizes requests. If nil, one is derived from Token: a shared-token
	// authenticator when Token != "", otherwise no-auth (loopback/dev).
	Auth  Authenticator
	Token string

	// Resolve selects the Core per request (for embedders). nil -> always Core.
	Resolve CoreResolver

	MCP http.Handler

	MaxBodyBytes int64
	RateLimit    float64
	RateBurst    int
	BehindProxy  bool
	CORSOrigins  []string
	Metrics      bool
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
	authn := cfg.Auth
	if authn == nil {
		if cfg.Token != "" {
			authn = tokenAuth{cfg.Token}
		} else {
			authn = noAuth{}
		}
	}
	now := time.Now()
	s := &Server{
		primaryCore: cfg.Core,
		resolve:     cfg.Resolve,
		auth:        authn,
		static:      cfg.Static,
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
// onChange hook so any mutation (HTTP/MCP/CLI) refreshes it.
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

// coreFor returns the Core to serve r (per the resolver, else the global core).
func (s *Server) coreFor(r *http.Request) (*core.Core, error) {
	if s.resolve != nil {
		return s.resolve(r)
	}
	if s.primaryCore != nil {
		return s.primaryCore, nil
	}
	return nil, errors.New("no core configured")
}

// Handler returns the fully-wired http.Handler (middleware + auth + routes).
func (s *Server) Handler() http.Handler {
	// Auth-gated: task DATA + MCP only.
	app := http.NewServeMux()
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

	root := http.NewServeMux()
	// Public static UI (client-side assets only; all data access below is gated).
	// The browser must be able to load these to render the login screen.
	staticFS := http.FileServer(http.FS(s.static))
	serveIndex := func(w http.ResponseWriter, r *http.Request) {
		s.serveStatic(w, r, "index.html", "text/html; charset=utf-8")
	}
	root.HandleFunc("GET /{$}", serveIndex)
	root.HandleFunc("GET /index.html", serveIndex)
	root.Handle("GET /static/", http.StripPrefix("/static/", staticFS))
	// Public operational endpoints.
	root.HandleFunc("GET /healthz", s.handleHealthz)
	root.HandleFunc("GET /readyz", s.handleReadyz)
	root.HandleFunc("GET /version", s.handleVersion)
	if s.metricsOn {
		root.Handle("GET /metrics", s.metrics.handler())
	}
	// Public login endpoints (ahead of the auth gate).
	root.HandleFunc("GET /auth", s.legacyAuthBootstrap)
	root.HandleFunc("GET /api/authinfo", s.handleAuthInfo)
	if lp, ok := s.auth.(LoginProvider); ok {
		root.HandleFunc("POST /api/login", lp.Login)
		root.HandleFunc("POST /api/logout", lp.Logout)
	}
	// Everything else (task data, MCP) is auth-gated.
	root.Handle("/", s.authGate(app))

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
