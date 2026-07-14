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
	"bytes"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
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
	primaryCore         *core.Core   // the single/default core; may be nil in a pure multi-core host
	resolve             CoreResolver // optional per-request core selection
	auth                Authenticator
	loginURL            string
	resourceMetadataURL string
	injectHead          string
	static              fs.FS
	logger              *slog.Logger
	mcp                 http.Handler

	maxBody     int64
	corsOrigins []string
	behindProxy bool
	metricsOn   bool

	rl      *rateLimiter
	metrics *metrics

	hub         *wsHub
	topicFor    func(r *http.Request) string
	csp         string
	allowDelete func(r *http.Request) bool

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

	// APIKeys enables DB-backed API-key auth (Bearer tasks_<secret>) verified
	// against Core, layered in front of the Token/Auth login flow. Requires Core.
	APIKeys bool

	// Resolve selects the Core per request (for embedders). nil -> always Core.
	Resolve CoreResolver

	// TopicFor maps a request to the tenant/board key its live (WebSocket) updates
	// belong to, so a mutation in one workspace only reaches that workspace's
	// sockets. nil -> a single global topic (single-tenant). It MUST derive the
	// same key used to route Publish calls (typically the org id).
	TopicFor func(r *http.Request) string

	// CSP overrides the Content-Security-Policy header. Set it when the UI must
	// reach an external origin (e.g. a hosted identity provider whose scripts and
	// API calls the default 'self'-only policy would block). Empty -> the strict
	// default. Must be kept in sync with whatever InjectHead loads.
	CSP string

	// AllowDelete gates hard task deletion. When nil, DELETE /api/v1/tasks/{id} is
	// NOT registered at all — deletion is impossible (and never an MCP tool or CLI
	// command, since it's deliberately not in the op registry). When set, the
	// route exists and each request must pass the predicate — an embedder returns
	// true only for a human session, never an API key / agent.
	AllowDelete func(r *http.Request) bool

	// LoginURL, when set, is reported by /api/authinfo so the UI redirects an
	// unauthenticated visitor to a hosted sign-in page (used with a custom Auth).
	LoginURL string

	// ResourceMetadataURL, when set, is advertised in the 401 WWW-Authenticate
	// header (RFC 9728) so an OAuth-capable MCP client discovers the authorization
	// server. Generic OAuth-resource-server support; the AS itself lives in the host.
	ResourceMetadataURL string

	// InjectHead, when set, is spliced into index.html before </head>. A host uses
	// it to add e.g. an identity-provider script that keeps a session alive on the
	// board page; the engine stays agnostic about what it injects.
	InjectHead string

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
	// Layer DB-backed API keys in front of the base authenticator (single-tenant).
	if cfg.APIKeys && cfg.Core != nil {
		authn = newKeyAuth(authn, cfg.Core)
	}
	now := time.Now()
	s := &Server{
		primaryCore:         cfg.Core,
		resolve:             cfg.Resolve,
		auth:                authn,
		loginURL:            cfg.LoginURL,
		resourceMetadataURL: cfg.ResourceMetadataURL,
		injectHead:          cfg.InjectHead,
		static:              cfg.Static,
		logger:              logger,
		mcp:                 cfg.MCP,
		maxBody:             maxBody,
		corsOrigins:         cfg.CORSOrigins,
		behindProxy:         cfg.BehindProxy,
		metricsOn:           cfg.Metrics,
		metrics:             newMetrics(now),
		hub:                 newWSHub(),
		topicFor:            cfg.TopicFor,
		csp:                 cfg.CSP,
		allowDelete:         cfg.AllowDelete,
		lastChange:          now,
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
	app.HandleFunc("POST /api/v1/import", s.handleImport)
	app.HandleFunc("GET /api/ws", s.handleWS)
	// Hard delete is a dedicated, gated route — deliberately NOT an op, so it's
	// never exposed via MCP or the CLI. Only registered when an embedder opts in.
	if s.allowDelete != nil {
		app.HandleFunc("DELETE /api/v1/tasks/{id}", s.handleDelete)
	}
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
	// Everything else. A browser navigating to a client-side route (e.g.
	// /tasks/abc) gets the SPA shell so a deep-link refresh works; API + MCP
	// requests stay auth-gated. Detection: an HTML GET that isn't /api or /mcp.
	gated := s.authGate(app)
	root.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		// A public file that lives at the bundle root — favicon.ico,
		// apple-touch-icon.png, manifest.webmanifest, … — served without the gate
		// (browsers request these at the site root, ignoring the <link> paths).
		if r.Method == http.MethodGet && s.static != nil && isStaticFile(s.static, p) {
			staticFS.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && s.static != nil &&
			strings.Contains(r.Header.Get("Accept"), "text/html") &&
			!strings.HasPrefix(p, "/api/") && !strings.HasPrefix(p, "/mcp") {
			serveIndex(w, r)
			return
		}
		gated.ServeHTTP(w, r)
	}))

	var h http.Handler = root
	h = gzipMiddleware(h) // innermost: compress the router's response
	h = bodyLimit(s.maxBody, h)
	if s.rl != nil {
		h = s.rl.middleware(h)
	}
	if len(s.corsOrigins) > 0 {
		h = cors(s.corsOrigins, h)
	}
	h = securityHeaders(s.csp, h)
	h = s.metrics.middleware(h)
	h = accessLog(s.logger, s.behindProxy, h)
	h = requestIDMiddleware(h)
	h = recoverer(s.logger, h)
	return h
}

// isStaticFile reports whether urlPath maps to a regular file in the bundle
// (so the root handler can serve public assets like /favicon.ico directly).
func isStaticFile(fsys fs.FS, urlPath string) bool {
	name := strings.Trim(path.Clean("/"+urlPath), "/")
	if name == "" {
		return false
	}
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request, name, ctype string) {
	data, err := fs.ReadFile(s.static, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if name == "index.html" && s.injectHead != "" {
		data = bytes.Replace(data, []byte("</head>"), append([]byte(s.injectHead), "</head>"...), 1)
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}
