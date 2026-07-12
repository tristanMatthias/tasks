package httpapi

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// gzipMiddleware compresses responses when the client accepts it. It skips
// /static/ (the file server sets Content-Length from the uncompressed size) and
// /mcp (SSE streaming), so it targets the big JSON API + the HTML shell.
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") ||
			strings.HasPrefix(p, "/static/") || strings.HasPrefix(p, "/mcp") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length") // gzip changes the length
		gz := gzip.NewWriter(w)
		defer gz.Close()
		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, gz: gz}, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) { return g.gz.Write(b) }

func (g *gzipResponseWriter) Flush() {
	_ = g.gz.Flush()
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

type ctxKey int

const requestIDKey ctxKey = iota

// responseRecorder captures the status code and byte count for logging/metrics.
type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Flush implements http.Flusher so streaming handlers (MCP SSE) keep working.
func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// recoverer turns a panic in any handler into a 500 and logs the stack.
func recoverer(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				logger.Error("panic recovered",
					"panic", v, "path", r.URL.Path, "request_id", RequestID(r.Context()),
					"stack", string(debug.Stack()))
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// requestIDMiddleware attaches a request id (honoring an inbound X-Request-Id)
// to the context and echoes it in the response.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestID extracts the request id from a context ("" if absent).
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "reqid"
	}
	return hex.EncodeToString(b)
}

// accessLog logs one structured line per request after it completes.
func accessLog(logger *slog.Logger, behindProxy bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"bytes", rec.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", clientIP(r, behindProxy),
			"request_id", RequestID(r.Context()),
		)
	})
}

// securityHeaders sets conservative security headers on every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		// The UI is self-contained (no external origins); allow inline styles/scripts
		// it already uses, block everything else.
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// cors adds permissive CORS headers for the configured origins (if any).
func cors(origins []string, next http.Handler) http.Handler {
	allowed := map[string]bool{}
	wildcard := false
	for _, o := range origins {
		if o == "*" {
			wildcard = true
		}
		allowed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (wildcard || allowed[origin]) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-Id")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// bodyLimit caps request body size for methods that carry a body.
func bodyLimit(max int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPatch || r.Method == http.MethodPut) {
			r.Body = http.MaxBytesReader(w, r.Body, max)
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimiter is a per-client-IP token-bucket limiter with idle eviction.
type rateLimiter struct {
	mu          sync.Mutex
	clients     map[string]*clientLimiter
	rate        rate.Limit
	burst       int
	behindProxy bool
	lastSweep   time.Time
}

type clientLimiter struct {
	lim  *rate.Limiter
	seen time.Time
}

func newRateLimiter(rps float64, burst int, behindProxy bool, now time.Time) *rateLimiter {
	return &rateLimiter{
		clients:     map[string]*clientLimiter{},
		rate:        rate.Limit(rps),
		burst:       burst,
		behindProxy: behindProxy,
		lastSweep:   now,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cl, ok := rl.clients[ip]
	if !ok {
		cl = &clientLimiter{lim: rate.NewLimiter(rl.rate, rl.burst)}
		rl.clients[ip] = cl
	}
	cl.seen = now
	// Evict clients idle for >10m, at most once a minute.
	if now.Sub(rl.lastSweep) > time.Minute {
		for k, v := range rl.clients {
			if now.Sub(v.seen) > 10*time.Minute {
				delete(rl.clients, k)
			}
		}
		rl.lastSweep = now
	}
	return cl.lim.Allow()
}

func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(clientIP(r, rl.behindProxy)) {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP returns the request's client IP, honoring X-Forwarded-For only when
// the server is explicitly configured to sit behind a trusted proxy.
func clientIP(r *http.Request, behindProxy bool) string {
	if behindProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i >= 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
