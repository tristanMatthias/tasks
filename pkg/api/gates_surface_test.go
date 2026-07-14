package api

import "testing"

// The gate trust boundary is enforced by which surfaces each op is exposed on.
// These assertions lock that in so a refactor can't silently widen it.
func TestGateSurfaceBoundaries(t *testing.T) {
	r := Ops()

	verify := r.Lookup("verify")
	if verify == nil {
		t.Fatal("verify op missing")
	}
	if !verify.Local {
		t.Error("verify must be Local (CLI-only)")
	}
	if verify.OnHTTP() {
		t.Error("verify must NOT be mounted on HTTP")
	}
	if verify.OnMCP() {
		t.Error("verify must NOT be an MCP tool — the API/MCP cannot verify")
	}

	// gate rm is a bypass risk (an agent deleting its own gates), so it's kept
	// off the MCP tool surface but stays on HTTP + CLI.
	rm := r.Lookup("gate rm")
	if rm == nil {
		t.Fatal("gate rm op missing")
	}
	if rm.OnMCP() {
		t.Error("gate rm must NOT be an MCP tool")
	}
	if !rm.OnHTTP() {
		t.Error("gate rm should still be on HTTP")
	}

	// gate add is a normal op — every surface, including MCP.
	add := r.Lookup("gate add")
	if add == nil || !add.OnMCP() || !add.OnHTTP() {
		t.Fatalf("gate add should be exposed on all surfaces: %+v", add)
	}
}
