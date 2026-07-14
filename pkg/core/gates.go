package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tristanMatthias/tasks/pkg/model"
	"github.com/tristanMatthias/tasks/pkg/store"
)

// verifyTokenTTL bounds how long a one-time verify token is valid after
// BeginVerify. Long enough to run a test suite, short enough to be single-shot.
const verifyTokenTTL = 15 * time.Minute

// evidenceMax caps the amount of command output stored as gate evidence (the
// tail is kept — the end of a test run is usually where the verdict is).
const evidenceMax = 4000

// GateSpec describes a gate to add. Only command gates are implemented; Type is
// carried through so human/server gate types can be added without a schema change.
type GateSpec struct {
	Type        string
	Command     string
	Description string
}

// AddGate attaches a new gate to a task and returns the updated task.
func (c *Core) AddGate(taskID string, spec GateSpec) (*model.Task, error) {
	taskID = c.resolveID(taskID)
	ok, err := c.st.Exists(taskID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	gtype := spec.Type
	if gtype == "" {
		gtype = model.GateCommand
	}
	if gtype != model.GateCommand {
		return nil, fmt.Errorf("unsupported gate type %q (only %q is implemented)", gtype, model.GateCommand)
	}
	if strings.TrimSpace(spec.Command) == "" {
		return nil, fmt.Errorf("a command gate needs a command to run (set --command)")
	}
	gates, err := c.st.ListGates(taskID)
	if err != nil {
		return nil, err
	}
	g := model.Gate{
		IssueID: taskID, ID: nextGateID(gates), Type: gtype,
		Command: spec.Command, Description: spec.Description,
		Status: model.GatePending, CreatedAt: c.now(),
	}
	if err := c.st.AddGate(g); err != nil {
		return nil, wrap("gate add", err)
	}
	c.changed(taskID)
	return c.st.Get(taskID)
}

// RemoveGate detaches a gate from a task and returns the updated task.
func (c *Core) RemoveGate(taskID, gateID string) (*model.Task, error) {
	taskID = c.resolveID(taskID)
	if err := c.st.RemoveGate(taskID, gateID); err != nil {
		return nil, wrap("gate rm", err)
	}
	c.changed(taskID)
	return c.st.Get(taskID)
}

// nextGateID mints the next "gN" id, one past the highest existing suffix (so
// ids stay stable and readable even after removals).
func nextGateID(existing []model.Gate) string {
	max := 0
	for _, g := range existing {
		if strings.HasPrefix(g.ID, "g") {
			if n, err := strconv.Atoi(g.ID[1:]); err == nil && n > max {
				max = n
			}
		}
	}
	return "g" + strconv.Itoa(max+1)
}

// pendingGates returns the command gates that still need verification.
func pendingGates(gates []model.Gate) []model.Gate {
	var out []model.Gate
	for _, g := range gates {
		if g.Type == model.GateCommand && g.Status != model.GateVerified {
			out = append(out, g)
		}
	}
	return out
}

// BeginVerify arms a fresh one-time token on every pending command gate and
// returns them as challenges for the CLI to run. Re-callable: each call
// supersedes the prior tokens.
func (c *Core) BeginVerify(taskID string) (*model.VerifySession, error) {
	taskID = c.resolveID(taskID)
	ok, err := c.st.Exists(taskID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	gates, err := c.st.ListGates(taskID)
	if err != nil {
		return nil, err
	}
	expires := c.nowFunc().UTC().Add(verifyTokenTTL).Format("2006-01-02T15:04:05Z")
	sess := &model.VerifySession{TaskID: taskID}
	for _, g := range pendingGates(gates) {
		tok, err := newToken()
		if err != nil {
			return nil, err
		}
		if err := c.st.SetGateToken(taskID, g.ID, tok, expires); err != nil {
			return nil, err
		}
		sess.Challenges = append(sess.Challenges, model.VerifyChallenge{
			GateID: g.ID, Type: g.Type, Command: g.Command,
			Description: g.Description, Token: tok, Expires: expires,
		})
	}
	if len(sess.Challenges) == 0 {
		sess.Message = fmt.Sprintf("No pending command gates on %s — nothing to verify. You can close it: tasks close %s", taskID, taskID)
	} else {
		sess.Message = fmt.Sprintf("%d gate(s) to verify on %s. The CLI runs each command locally and reports the result.", len(sess.Challenges), taskID)
	}
	return sess, nil
}

// CompleteVerify records the result of running one gate's command. It verifies
// the gate only when exitCode==0 AND the one-time token still checks out
// (matches, unused, unexpired). This is the ONLY path that flips a gate to
// verified, and it is reachable only through the CLI's local-execution flow.
func (c *Core) CompleteVerify(taskID, gateID, token string, exitCode int, output string) (*model.Task, error) {
	taskID = c.resolveID(taskID)
	if exitCode != 0 {
		return nil, fmt.Errorf("gate %s command exited %d — not verified. Fix the failure, then re-run: tasks verify %s", gateID, exitCode, taskID)
	}
	evidence := output
	if len(evidence) > evidenceMax {
		evidence = "…" + evidence[len(evidence)-evidenceMax:]
	}
	now := c.now()
	if err := c.st.ConsumeGateVerify(taskID, gateID, token, now, now, c.actor, exitCode, evidence); err != nil {
		if err == store.ErrGateToken {
			return nil, fmt.Errorf("verify token invalid, used, or expired for gate %s — run `tasks verify %s` to get a fresh one", gateID, taskID)
		}
		return nil, wrap("verify", err)
	}
	c.changed(taskID)
	return c.st.Get(taskID)
}

// GatesPendingError is returned by Close when unverified gates block it. Its
// message is written for an LLM operator: it names the gates and the exact CLI
// command to run, and states plainly that the API/MCP cannot verify.
type GatesPendingError struct {
	TaskID string
	Gates  []model.Gate
}

func (e *GatesPendingError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "cannot close %s: %d acceptance gate(s) not verified.\n", e.TaskID, len(e.Gates))
	for _, g := range e.Gates {
		desc := g.Description
		if desc == "" {
			desc = g.Command
		}
		fmt.Fprintf(&b, "  - [%s] %s\n", g.ID, desc)
	}
	fmt.Fprintf(&b, "Only the CLI can verify these — the API and MCP cannot mark a gate verified. "+
		"On the machine that has the code, run:\n  tasks verify %s\n"+
		"It runs each gate's command locally and records the result; then closing will succeed.", e.TaskID)
	return b.String()
}

// newToken returns a random 128-bit hex token for one-time gate verification.
func newToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
