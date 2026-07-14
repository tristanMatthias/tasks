package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/model"
)

// The full HTTP surface of the gate feature: add a gate, close is blocked (409
// with guidance), begin issues a token, complete verifies with exit 0, close
// then succeeds. And the API cannot verify without running the command.
func TestGateVerifyFlowHTTP(t *testing.T) {
	ts := newServer(t, "tok")

	// Create a task and attach a command gate (gate add is a normal op).
	_, body := do(t, ts, "POST", "/api/v1/tasks", "tok", `{"title":"needs proof"}`)
	var task model.Task
	if err := json.Unmarshal(body, &task); err != nil {
		t.Fatal(err)
	}
	code, body := do(t, ts, "POST", "/api/v1/tasks/"+task.ID+"/gates", "tok",
		`{"command":"true","description":"unit tests pass"}`)
	if code != http.StatusOK {
		t.Fatalf("gate add: %d %s", code, body)
	}

	// Close is blocked by the pending gate → 409 with LLM guidance.
	code, body = do(t, ts, "POST", "/api/v1/tasks/"+task.ID+"/close", "tok", `{}`)
	if code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", code, body)
	}
	if !strings.Contains(string(body), "tasks verify") || !strings.Contains(string(body), "cannot") {
		t.Fatalf("guidance missing from 409 body: %s", body)
	}

	// Begin → a challenge with a one-time token + the command.
	code, body = do(t, ts, "POST", "/api/v1/tasks/"+task.ID+"/gates/verify/begin", "tok", `{}`)
	if code != http.StatusOK {
		t.Fatalf("begin: %d %s", code, body)
	}
	var sess model.VerifySession
	if err := json.Unmarshal(body, &sess); err != nil {
		t.Fatal(err)
	}
	if len(sess.Challenges) != 1 || sess.Challenges[0].Token == "" {
		t.Fatalf("expected one tokened challenge, got %+v", sess.Challenges)
	}
	tok := sess.Challenges[0].Token
	gate := sess.Challenges[0].GateID

	// A wrong exit code does NOT verify (the "prove it ran" half of the contract).
	code, _ = do(t, ts, "POST", "/api/v1/tasks/"+task.ID+"/gates/"+gate+"/verify/complete", "tok",
		`{"token":"`+tok+`","exit_code":1,"output":"boom"}`)
	if code == http.StatusOK {
		t.Fatal("a failing command must not verify the gate")
	}

	// Correct exit 0 + valid token verifies.
	code, body = do(t, ts, "POST", "/api/v1/tasks/"+task.ID+"/gates/"+gate+"/verify/complete", "tok",
		`{"token":"`+tok+`","exit_code":0,"output":"all pass"}`)
	if code != http.StatusOK {
		t.Fatalf("complete: %d %s", code, body)
	}

	// Now close succeeds.
	code, body = do(t, ts, "POST", "/api/v1/tasks/"+task.ID+"/close", "tok", `{}`)
	if code != http.StatusOK {
		t.Fatalf("close after verify: %d %s", code, body)
	}
}

// Error paths on the dedicated verify routes.
func TestGateVerifyErrorsHTTP(t *testing.T) {
	ts := newServer(t, "tok")

	// begin on a non-existent task → 404.
	if code, _ := do(t, ts, "POST", "/api/v1/tasks/proj-nope/gates/verify/begin", "tok", `{}`); code != http.StatusNotFound {
		t.Fatalf("begin on missing task: expected 404, got %d", code)
	}
	// complete with a bad token → not verified (4xx).
	_, body := do(t, ts, "POST", "/api/v1/tasks", "tok", `{"title":"x"}`)
	var task model.Task
	json.Unmarshal(body, &task)
	do(t, ts, "POST", "/api/v1/tasks/"+task.ID+"/gates", "tok", `{"command":"true"}`)
	do(t, ts, "POST", "/api/v1/tasks/"+task.ID+"/gates/verify/begin", "tok", `{}`)
	code, _ := do(t, ts, "POST", "/api/v1/tasks/"+task.ID+"/gates/g1/verify/complete", "tok",
		`{"token":"bogus","exit_code":0,"output":"x"}`)
	if code == http.StatusOK {
		t.Fatal("a bogus token must not verify")
	}
	// malformed JSON body → 400.
	if code, _ := do(t, ts, "POST", "/api/v1/tasks/"+task.ID+"/gates/g1/verify/complete", "tok", `{bad`); code != http.StatusBadRequest {
		t.Fatalf("malformed complete body: expected 400, got %d", code)
	}

	// gate rm removes a gate (available on HTTP even though not on MCP).
	if code, _ := do(t, ts, "DELETE", "/api/v1/tasks/"+task.ID+"/gates/g1", "tok", ""); code != http.StatusOK {
		t.Fatalf("gate rm: expected 200, got %d", code)
	}
}

// The verify op is CLI-local, so it must NOT be reachable as an HTTP route.
func TestVerifyOpHasNoHTTPRoute(t *testing.T) {
	ts := newServer(t, "tok")
	_, body := do(t, ts, "POST", "/api/v1/tasks", "tok", `{"title":"x"}`)
	var task model.Task
	json.Unmarshal(body, &task)
	// There is no POST /api/v1/tasks/{id}/verify — the mux 404s / method-mismatches.
	code, _ := do(t, ts, "POST", "/api/v1/tasks/"+task.ID+"/verify", "tok", `{}`)
	if code == http.StatusOK {
		t.Fatal("verify must not be an HTTP op")
	}
}
