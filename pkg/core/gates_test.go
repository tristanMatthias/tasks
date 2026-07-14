package core

import (
	"strings"
	"testing"

	"github.com/tristanMatthias/tasks/pkg/model"
)

func TestAddGateMintsIdsAndValidates(t *testing.T) {
	c := newCore(t)
	tk, _ := c.Create(CreateParams{Title: "x"})

	// Type defaults to command; ids mint g1, g2…
	got, err := c.AddGate(tk.ID, GateSpec{Command: "go test ./...", Description: "tests pass"})
	must(t, err)
	if len(got.Gates) != 1 || got.Gates[0].ID != "g1" || got.Gates[0].Type != model.GateCommand {
		t.Fatalf("g1 not added: %+v", got.Gates)
	}
	got, _ = c.AddGate(tk.ID, GateSpec{Command: "go vet ./..."})
	if len(got.Gates) != 2 || got.Gates[1].ID != "g2" {
		t.Fatalf("g2 not added: %+v", got.Gates)
	}

	// A command gate requires a command.
	if _, err := c.AddGate(tk.ID, GateSpec{}); err == nil {
		t.Fatal("expected error for empty command")
	}
	// Unknown gate type is rejected.
	if _, err := c.AddGate(tk.ID, GateSpec{Type: "human", Command: "x"}); err == nil {
		t.Fatal("expected unsupported-type error")
	}
	// Missing task.
	if _, err := c.AddGate("proj-nope", GateSpec{Command: "x"}); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRemoveGate(t *testing.T) {
	c := newCore(t)
	tk, _ := c.Create(CreateParams{Title: "x"})
	c.AddGate(tk.ID, GateSpec{Command: "true"})
	got, err := c.RemoveGate(tk.ID, "g1")
	must(t, err)
	if len(got.Gates) != 0 {
		t.Fatalf("gate not removed: %+v", got.Gates)
	}
	if _, err := c.RemoveGate(tk.ID, "g1"); err == nil {
		t.Fatal("expected error removing missing gate")
	}
}

func TestCloseBlockedByPendingGate(t *testing.T) {
	c := newCore(t)
	tk, _ := c.Create(CreateParams{Title: "x"})
	c.AddGate(tk.ID, GateSpec{Command: "true", Description: "the thing works"})

	_, err := c.Close(tk.ID, CloseParams{})
	if err == nil {
		t.Fatal("expected close to be blocked by pending gate")
	}
	gpe, ok := err.(*GatesPendingError)
	if !ok {
		t.Fatalf("expected *GatesPendingError, got %T", err)
	}
	if len(gpe.Gates) != 1 {
		t.Fatalf("expected 1 pending gate, got %d", len(gpe.Gates))
	}
	// The message guides an LLM operator to the CLI.
	msg := err.Error()
	for _, want := range []string{tk.ID, "tasks verify", "the thing works", "API and MCP cannot"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("guidance missing %q in:\n%s", want, msg)
		}
	}
}

func TestVerifyFlowUnblocksClose(t *testing.T) {
	c := newCore(t)
	tk, _ := c.Create(CreateParams{Title: "x"})
	c.AddGate(tk.ID, GateSpec{Command: "true"})

	// Begin → one challenge with a token + the command to run.
	sess, err := c.BeginVerify(tk.ID)
	must(t, err)
	if len(sess.Challenges) != 1 {
		t.Fatalf("expected 1 challenge, got %d", len(sess.Challenges))
	}
	ch := sess.Challenges[0]
	if ch.Token == "" || ch.Command != "true" || ch.GateID != "g1" {
		t.Fatalf("bad challenge: %+v", ch)
	}

	// A non-zero exit does NOT verify.
	if _, err := c.CompleteVerify(tk.ID, ch.GateID, ch.Token, 1, "boom"); err == nil {
		t.Fatal("expected failure for non-zero exit")
	}
	// Exit 0 with the valid token verifies + records evidence.
	upd, err := c.CompleteVerify(tk.ID, ch.GateID, ch.Token, 0, "PASS\nok")
	must(t, err)
	if upd.Gates[0].Status != model.GateVerified || upd.Gates[0].Evidence == "" {
		t.Fatalf("gate not verified: %+v", upd.Gates[0])
	}

	// Now close succeeds.
	closed, err := c.Close(tk.ID, CloseParams{})
	must(t, err)
	if closed.Status != "closed" {
		t.Fatalf("expected closed, got %q", closed.Status)
	}
}

func TestVerifyEvidenceTruncated(t *testing.T) {
	c := newCore(t)
	tk, _ := c.Create(CreateParams{Title: "x"})
	c.AddGate(tk.ID, GateSpec{Command: "true"})
	sess, _ := c.BeginVerify(tk.ID)
	big := strings.Repeat("A", evidenceMax+500)
	upd, err := c.CompleteVerify(tk.ID, "g1", sess.Challenges[0].Token, 0, big)
	must(t, err)
	ev := upd.Gates[0].Evidence
	if len(ev) > evidenceMax+3 || !strings.HasPrefix(ev, "…") {
		t.Fatalf("evidence not truncated to the tail: len=%d prefix=%q", len(ev), ev[:3])
	}
}

func TestVerifyRejectsBadToken(t *testing.T) {
	c := newCore(t)
	tk, _ := c.Create(CreateParams{Title: "x"})
	c.AddGate(tk.ID, GateSpec{Command: "true"})
	c.BeginVerify(tk.ID)
	if _, err := c.CompleteVerify(tk.ID, "g1", "not-the-token", 0, "x"); err == nil {
		t.Fatal("expected bad-token rejection")
	}
}

func TestBeginVerifyNoGates(t *testing.T) {
	c := newCore(t)
	tk, _ := c.Create(CreateParams{Title: "x"})
	sess, err := c.BeginVerify(tk.ID)
	must(t, err)
	if len(sess.Challenges) != 0 || !strings.Contains(sess.Message, "nothing to verify") {
		t.Fatalf("unexpected empty session: %+v", sess)
	}
	if _, err := c.BeginVerify("proj-nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
