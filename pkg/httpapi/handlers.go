package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tristanMatthias/tasks/pkg/api"
	"github.com/tristanMatthias/tasks/pkg/model"
	"github.com/tristanMatthias/tasks/pkg/store"
)

// ---- UI endpoints (compatible with beads_ui/server.py) ----

func (s *Server) handleIssues(w http.ResponseWriter, r *http.Request) {
	c, err := s.coreFor(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	tasks, err := c.All()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if tasks == nil {
		tasks = []model.Task{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issues": tasks,
		"mtime":  s.mtime(),
		"count":  len(tasks),
	})
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	c, err := s.coreFor(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	n, err := c.Store().Count()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mtime": s.mtime(), "count": n, "path": "sqlite"})
}

// handlePull is a no-op compatibility shim: the server is the single source of
// truth, so there is no Dolt remote to pull. Returns ok so the UI reload works.
func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": "single source of truth — nothing to pull"})
}

// ---- generic REST handler generated from an api.Op ----

func (s *Server) opHandler(op *api.Op) http.HandlerFunc {
	fields := op.Fields()
	return func(w http.ResponseWriter, r *http.Request) {
		in := op.NewInput()
		var decode func(any) error
		if op.HasBody() {
			decode = func(v any) error {
				if r.Body == nil || r.ContentLength == 0 {
					return nil
				}
				return json.NewDecoder(r.Body).Decode(v)
			}
		}
		if err := api.BindHTTP(fields, in, r, decode); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if err := api.Validate(fields, in); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		c, err := s.coreFor(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		out, err := op.Handle(c, in)
		if err != nil {
			writeCoreErr(w, err)
			return
		}
		status := http.StatusOK
		if op.Name == "create" {
			status = http.StatusCreated
		}
		writeJSON(w, status, out)
	}
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func writeCoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeErr(w, http.StatusBadRequest, err)
}
