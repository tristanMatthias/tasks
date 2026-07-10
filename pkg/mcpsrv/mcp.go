package mcpsrv

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tristanMatthias/tasks/pkg/api"
	"github.com/tristanMatthias/tasks/pkg/core"
)

// newHandler builds the streamable-HTTP MCP handler over c. Mount at /mcp.
func newHandler(c *core.Core) http.Handler {
	srv := NewServer(c)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}

// NewServer constructs the MCP server, generating one tool per registry op —
// the same single source of truth that drives HTTP and CLI.
func NewServer(c *core.Core) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "tasks", Version: "1.0.0"}, nil)
	for _, op := range api.Ops() {
		s.AddTool(&mcp.Tool{
			Name:        op.Name,
			Description: op.Summary,
			InputSchema: op.Schema(),
		}, toolHandler(c, op))
	}
	return s
}

// toolHandler adapts an api.Op into an MCP ToolHandler: unmarshal args into the
// op's input struct, validate, run, and return the result as JSON text.
func toolHandler(c *core.Core, op *api.Op) mcp.ToolHandler {
	fields := op.Fields()
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		in := op.NewInput()
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, in); err != nil {
				return errResult(err), nil
			}
		}
		if err := api.Validate(fields, in); err != nil {
			return errResult(err), nil
		}
		out, err := op.Handle(c, in)
		if err != nil {
			return errResult(err), nil
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil
	}
}

func errResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: "error: " + err.Error()}},
	}
}
