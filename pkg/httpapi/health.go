package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/tristanMatthias/tasks/pkg/buildinfo"
)

// handleHealthz is a liveness probe — always 200 while the process is up.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// handleReadyz is a readiness probe — 200 only when the database answers. It
// pings the primary core; in a pure multi-core host (no primary) it reports
// process liveness.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.primaryCore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.primaryCore.Store().Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "unavailable", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

// handleVersion returns build metadata.
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, buildinfo.Map())
}
