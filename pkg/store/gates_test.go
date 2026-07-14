package store

import (
	"testing"

	"github.com/tristanMatthias/tasks/pkg/model"
)

func TestGatesPersistAndHydrate(t *testing.T) {
	st := open(t)
	task := mk("proj-a", "open", "task", p(2))
	task.Gates = []model.Gate{
		{IssueID: "proj-a", ID: "g1", Type: model.GateCommand, Command: "go test ./...",
			Description: "unit tests pass", Status: model.GatePending, CreatedAt: "2026-01-01T00:00:00Z"},
	}
	must(t, st.Insert(task))

	got, err := st.Get("proj-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.GateCount != 1 || len(got.Gates) != 1 {
		t.Fatalf("gate not hydrated: %+v", got.Gates)
	}
	if got.Gates[0].Command != "go test ./..." || got.Gates[0].Status != model.GatePending {
		t.Fatalf("gate fields wrong: %+v", got.Gates[0])
	}
}

func TestGateAddRemoveGetList(t *testing.T) {
	st := open(t)
	must(t, st.Insert(mk("proj-a", "open", "task", p(2))))
	must(t, st.AddGate(model.Gate{IssueID: "proj-a", ID: "g1", Type: model.GateCommand,
		Command: "true", Status: model.GatePending}))
	must(t, st.AddGate(model.Gate{IssueID: "proj-a", ID: "g2", Type: model.GateCommand,
		Command: "false", Status: model.GatePending}))

	gates, err := st.ListGates("proj-a")
	if err != nil || len(gates) != 2 {
		t.Fatalf("list gates: %v %+v", err, gates)
	}
	g, err := st.GetGate("proj-a", "g2")
	if err != nil || g.Command != "false" {
		t.Fatalf("get gate: %v %+v", err, g)
	}
	if _, err := st.GetGate("proj-a", "nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	must(t, st.RemoveGate("proj-a", "g1"))
	if err := st.RemoveGate("proj-a", "g1"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound removing twice, got %v", err)
	}
	gates, _ = st.ListGates("proj-a")
	if len(gates) != 1 {
		t.Fatalf("expected 1 gate after remove, got %d", len(gates))
	}
}

func TestGateVerifyTokenLifecycle(t *testing.T) {
	st := open(t)
	must(t, st.Insert(mk("proj-a", "open", "task", p(2))))
	must(t, st.AddGate(model.Gate{IssueID: "proj-a", ID: "g1", Type: model.GateCommand,
		Command: "true", Status: model.GatePending}))

	const now = "2026-01-01T00:00:00Z"
	const future = "2026-01-01T00:15:00Z"

	// No token armed yet → any verify attempt fails.
	if err := st.ConsumeGateVerify("proj-a", "g1", "tok", now, now, "me", 0, "ok"); err != ErrGateToken {
		t.Fatalf("expected ErrGateToken with no token, got %v", err)
	}

	// Arm a token, then a WRONG token is refused.
	must(t, st.SetGateToken("proj-a", "g1", "right", future))
	if err := st.ConsumeGateVerify("proj-a", "g1", "wrong", now, now, "me", 0, "ok"); err != ErrGateToken {
		t.Fatalf("wrong token should fail, got %v", err)
	}

	// Correct token verifies once…
	must(t, st.ConsumeGateVerify("proj-a", "g1", "right", now, now, "me", 0, "all pass"))
	g, _ := st.GetGate("proj-a", "g1")
	if g.Status != model.GateVerified || g.Evidence != "all pass" || g.ExitCode == nil || *g.ExitCode != 0 {
		t.Fatalf("gate not verified: %+v", g)
	}
	// …and is single-use: the same token can't be replayed.
	if err := st.ConsumeGateVerify("proj-a", "g1", "right", now, now, "me", 0, "again"); err != ErrGateToken {
		t.Fatalf("token should be single-use, got %v", err)
	}
}

func TestGateVerifyTokenExpiry(t *testing.T) {
	st := open(t)
	must(t, st.Insert(mk("proj-a", "open", "task", p(2))))
	must(t, st.AddGate(model.Gate{IssueID: "proj-a", ID: "g1", Type: model.GateCommand, Status: model.GatePending}))
	// Token expired in the past relative to now → refused.
	must(t, st.SetGateToken("proj-a", "g1", "tok", "2026-01-01T00:00:00Z"))
	if err := st.ConsumeGateVerify("proj-a", "g1", "tok", "2026-01-01T00:10:00Z", "2026-01-01T00:10:00Z", "me", 0, "x"); err != ErrGateToken {
		t.Fatalf("expired token should fail, got %v", err)
	}
	// Setting a token on a missing gate is ErrNotFound.
	if err := st.SetGateToken("proj-a", "nope", "t", "2026-01-01T00:00:00Z"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// Every gate method surfaces the underlying DB error (exercised by closing the
// store out from under them).
func TestGateMethodsClosedStore(t *testing.T) {
	st := open(t)
	must(t, st.Insert(mk("proj-a", "open", "task", p(2))))
	must(t, st.AddGate(model.Gate{IssueID: "proj-a", ID: "g1", Type: model.GateCommand, Status: model.GatePending}))
	st.Close()

	if err := st.AddGate(model.Gate{IssueID: "proj-a", ID: "g2"}); err == nil {
		t.Error("AddGate should error on a closed store")
	}
	if err := st.RemoveGate("proj-a", "g1"); err == nil {
		t.Error("RemoveGate should error on a closed store")
	}
	if _, err := st.GetGate("proj-a", "g1"); err == nil {
		t.Error("GetGate should error on a closed store")
	}
	if _, err := st.ListGates("proj-a"); err == nil {
		t.Error("ListGates should error on a closed store")
	}
	if err := st.SetGateToken("proj-a", "g1", "t", "2026-01-01T00:00:00Z"); err == nil {
		t.Error("SetGateToken should error on a closed store")
	}
	if err := st.ConsumeGateVerify("proj-a", "g1", "t", "n", "n", "me", 0, "x"); err == nil {
		t.Error("ConsumeGateVerify should error on a closed store")
	}
}

func TestDeleteRemovesGates(t *testing.T) {
	st := open(t)
	must(t, st.Insert(mk("proj-a", "open", "task", p(2))))
	must(t, st.AddGate(model.Gate{IssueID: "proj-a", ID: "g1", Type: model.GateCommand, Status: model.GatePending}))
	must(t, st.Delete("proj-a"))
	gates, _ := st.ListGates("proj-a")
	if len(gates) != 0 {
		t.Fatalf("gates not cleaned up on delete: %+v", gates)
	}
}
