package main

import "testing"

// The CLI accepts a few env-var names for the API key; TASKS_TOKEN wins, then
// AGENT_TASKS_API_KEY (the agenttasks board name), then TASKS_API_KEY.
func TestAuthTokenPrecedence(t *testing.T) {
	t.Setenv("TASKS_TOKEN", "")
	t.Setenv("AGENT_TASKS_API_KEY", "")
	t.Setenv("TASKS_API_KEY", "")
	if got := authToken(); got != "" {
		t.Fatalf("no vars set → %q, want empty", got)
	}

	t.Setenv("TASKS_API_KEY", "k3")
	if got := authToken(); got != "k3" {
		t.Fatalf("TASKS_API_KEY → %q", got)
	}
	t.Setenv("AGENT_TASKS_API_KEY", "k2")
	if got := authToken(); got != "k2" {
		t.Fatalf("AGENT_TASKS_API_KEY should beat TASKS_API_KEY, got %q", got)
	}
	t.Setenv("TASKS_TOKEN", "k1")
	if got := authToken(); got != "k1" {
		t.Fatalf("TASKS_TOKEN should win, got %q", got)
	}
}
