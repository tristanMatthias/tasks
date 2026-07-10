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

// HandlerResolved builds an MCP HTTP handler that picks the Core per request via
// resolve (e.g. a multi-tenant CoreResolver), so a single /mcp endpoint serves
// each tenant its own board. Mount it behind an auth gate that populates
// whatever resolve reads (e.g. the request's Identity).
func HandlerResolved(resolve func(*http.Request) (*core.Core, error)) http.Handler {
	return newResolvedHandler(resolve)
}
