package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tristanMatthias/tasks/pkg/api"
	"github.com/tristanMatthias/tasks/pkg/importer"
	"github.com/tristanMatthias/tasks/pkg/model"
	"github.com/tristanMatthias/tasks/pkg/store"
)

// handleImport upserts a beads-style JSONL (the request body) into the caller's
// tenant, preserving ids / dependencies / timestamps. Idempotent; a large export
// can be streamed in chunks. Auth-gated (registered under the gated app mux).
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	c, err := s.coreFor(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	n, err := importer.ImportReader(c.Store(), r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": n})
}

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
	// ?view=tree returns a slim projection (no large text fields), ~10x smaller,
	// for the list/tree UI. Full task detail is fetched per-task on demand.
	var issues any = tasks
	if r.URL.Query().Get("view") == "tree" {
		issues = treeView(tasks)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issues": issues,
		"mtime":  s.mtime(),
		"count":  len(tasks),
	})
}

// treeTask is the minimal shape the list/tree needs: enough to render a row and
// build the parent-child hierarchy, without the heavy description/notes/comments.
type treeTask struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Status       string        `json:"status"`
	IssueType    string        `json:"issue_type"`
	Priority     *int          `json:"priority"`
	Dependencies []treeTaskDep `json:"dependencies,omitempty"`
}

type treeTaskDep struct {
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
}

func treeView(tasks []model.Task) []treeTask {
	out := make([]treeTask, 0, len(tasks))
	for i := range tasks {
		t := &tasks[i]
		var deps []treeTaskDep
		for _, d := range t.Dependencies {
			// Containment edges build the tree; blocks edges let the detail view
			// show what a task is blocking. Everything else stays out of the list.
			if d.Type == "parent-child" || d.Type == "parent" || d.Type == "blocks" {
				deps = append(deps, treeTaskDep{DependsOnID: d.DependsOnID, Type: d.Type})
			}
		}
		out = append(out, treeTask{
			ID: t.ID, Title: t.Title, Status: t.Status,
			IssueType: t.IssueType, Priority: t.Priority, Dependencies: deps,
		})
	}
	return out
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
