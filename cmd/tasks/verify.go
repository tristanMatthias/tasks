package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tristanMatthias/tasks/pkg/api"
	"github.com/tristanMatthias/tasks/pkg/model"
)

// commandTimeout bounds how long a single gate command may run.
const commandTimeout = 10 * time.Minute

// runLocal executes a Local op (one with no server route) in-process. The gate
// feature's `verify` is the only such op: it runs commands on THIS machine —
// the thing the API/MCP structurally cannot do — which is what makes the CLI
// the sole surface able to verify a gate.
func runLocal(op *api.Op, in any) error {
	switch op.Name {
	case "verify":
		return runVerify(newClient(), in.(*api.VerifyInput), os.Stdin)
	}
	return fmt.Errorf("no local handler for %q", op.Name)
}

// runVerify drives the gate handshake: ask the server for the pending gates +
// one-time tokens, run each command locally (with confirmation), and report the
// result back. Guidance at every step is written for an LLM operator.
func runVerify(c *client, in *api.VerifyInput, stdin io.Reader) error {
	sess, err := c.beginVerify(in.ID)
	if err != nil {
		return err
	}
	challenges := sess.Challenges
	if in.Gate != "" {
		challenges = filterGate(challenges, in.Gate)
		if len(challenges) == 0 {
			return fmt.Errorf("no pending command gate %q on %s (already verified, or unknown)", in.Gate, in.ID)
		}
	}
	if len(challenges) == 0 {
		fmt.Println(sess.Message)
		return nil
	}

	in.ID = sess.TaskID // use the resolved full id from here on
	reader := bufio.NewReader(stdin)
	var verified, failed, skipped int
	for _, ch := range challenges {
		fmt.Printf("\nGate %s", ch.GateID)
		if ch.Description != "" {
			fmt.Printf(" — %s", ch.Description)
		}
		fmt.Printf("\n  $ %s\n", ch.Command)

		if !in.Yes && !confirm(reader, "  Run this command locally?") {
			fmt.Println("  skipped (not confirmed).")
			skipped++
			continue
		}

		exit, output := runCommand(ch.Command)
		if _, err := c.completeVerify(in.ID, ch.GateID, model.VerifyResult{
			Token: ch.Token, ExitCode: exit, Output: output,
		}); err != nil {
			fmt.Printf("  ✗ gate %s not verified: %s\n", ch.GateID, err)
			failed++
			continue
		}
		fmt.Printf("  ✓ gate %s verified\n", ch.GateID)
		verified++
	}

	fmt.Printf("\n%d verified, %d failed, %d skipped.\n", verified, failed, skipped)
	// Re-query to report the true remaining count (verified gates won't reappear).
	after, err := c.beginVerify(in.ID)
	if err == nil && len(after.Challenges) == 0 {
		fmt.Printf("All acceptance gates verified — you can close it now:\n  tasks close %s\n", in.ID)
	} else if err == nil {
		fmt.Printf("%d gate(s) still pending. Re-run when ready:\n  tasks verify %s\n", len(after.Challenges), in.ID)
	}
	if failed > 0 {
		return fmt.Errorf("%d gate(s) failed verification", failed)
	}
	return nil
}

func filterGate(chs []model.VerifyChallenge, id string) []model.VerifyChallenge {
	for _, ch := range chs {
		if ch.GateID == id {
			return []model.VerifyChallenge{ch}
		}
	}
	return nil
}

// confirm prompts on stdout and reads a yes/no from reader. Anything other than
// y/yes is treated as no — commands come from the board, so the default is safe.
func confirm(reader *bufio.Reader, prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	line, _ := reader.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

// runCommand runs command via `sh -c` in the current working directory,
// streaming its output to the terminal while capturing it (as gate evidence).
// Returns the exit code (or -1 if it couldn't start / timed out).
func runCommand(command string) (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &buf)
	err := cmd.Run()
	if err == nil {
		return 0, buf.String()
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), buf.String()
	}
	buf.WriteString("\n" + err.Error())
	return -1, buf.String()
}

// ---- client calls for the dedicated verify endpoints ----

func (c *client) beginVerify(id string) (*model.VerifySession, error) {
	data, err := c.request("POST", "/api/v1/tasks/"+url.PathEscape(id)+"/gates/verify/begin", nil, nil)
	if err != nil {
		return nil, err
	}
	var s model.VerifySession
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *client) completeVerify(id, gate string, res model.VerifyResult) (*model.Task, error) {
	body := map[string]any{"token": res.Token, "exit_code": res.ExitCode, "output": res.Output}
	data, err := c.request("POST",
		"/api/v1/tasks/"+url.PathEscape(id)+"/gates/"+url.PathEscape(gate)+"/verify/complete", nil, body)
	if err != nil {
		return nil, err
	}
	var t model.Task
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
