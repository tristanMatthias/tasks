package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/tristanMatthias/tasks/pkg/model"
)

// handleVerifyBegin arms one-time tokens on a task's pending command gates and
// returns them as challenges for the CLI to run. Dedicated route (not an op):
// deliberately absent from the MCP tool surface.
func (s *Server) handleVerifyBegin(w http.ResponseWriter, r *http.Request) {
	c, err := s.coreFor(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	sess, err := c.BeginVerify(r.PathValue("id"))
	if err != nil {
		writeCoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

// handleVerifyComplete records the result of a locally-run gate command. Only
// exit 0 with a valid, unused token flips the gate to verified.
func (s *Server) handleVerifyComplete(w http.ResponseWriter, r *http.Request) {
	c, err := s.coreFor(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var res model.VerifyResult
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	}
	task, err := c.CompleteVerify(r.PathValue("id"), r.PathValue("gate"), res.Token, res.ExitCode, res.Output)
	if err != nil {
		writeCoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}
