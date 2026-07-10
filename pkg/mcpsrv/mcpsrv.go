// Package mcpsrv exposes the task tool subset over MCP (streamable HTTP) for
// Claude Code web. Handler returns an http.Handler to mount at /mcp, or nil to
// disable MCP.
package mcpsrv

import (
	"net/http"

	"github.com/tristanMatthias/tasks/pkg/core"
)

// Handler builds the MCP HTTP handler over c. (Implemented in mcp.go.)
func Handler(c *core.Core) http.Handler {
	return newHandler(c)
}
