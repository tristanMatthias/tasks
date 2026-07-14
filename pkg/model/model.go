// Package model defines the task data types and the exact JSONL serialization
// shape that the beads UI and `bd --json` consume. Field names and semantics
// mirror beads so the existing UI and the migrated data round-trip faithfully.
package model

import (
	"bufio"
	"encoding/json"
	"io"
	"sort"
)

// Valid enum values (mirrors beads).
var (
	Statuses = []string{"open", "in_progress", "closed", "deferred"}
	Types    = []string{"epic", "feature", "task", "bug", "chore"}
	// DepTypes seen in the imported data.
	DepTypes = []string{"parent-child", "parent", "blocks", "related", "relates-to", "discovered-from", "supersedes"}
)

// Task is a single issue/task. JSON tags reproduce the beads issue object so the
// UI (which reads .beads/issues.jsonl directly) and `bd --json` output match.
type Task struct {
	ID    string  `json:"id"`
	Key   *string `json:"key,omitempty"`   // slug for _type=="memory" records
	Type  *string `json:"_type,omitempty"` // nil for normal tasks; "memory" for remember entries
	Title string  `json:"title"`

	Description        string   `json:"description,omitempty"`
	Status             string   `json:"status"`
	Priority           *int     `json:"priority"` // always present; null for a few legacy records
	IssueType          string   `json:"issue_type"`
	Owner              string   `json:"owner,omitempty"`
	Assignee           string   `json:"assignee,omitempty"`
	CreatedBy          string   `json:"created_by,omitempty"`
	CreatedAt          string   `json:"created_at,omitempty"`
	UpdatedAt          string   `json:"updated_at,omitempty"`
	StartedAt          string   `json:"started_at,omitempty"`
	ClosedAt           string   `json:"closed_at,omitempty"`
	CloseReason        string   `json:"close_reason,omitempty"`
	AcceptanceCriteria string   `json:"acceptance_criteria,omitempty"`
	Design             string   `json:"design,omitempty"`
	Notes              string   `json:"notes,omitempty"`
	Labels             []string `json:"labels,omitempty"`
	Value              *string  `json:"value,omitempty"` // body for _type=="memory" records

	Dependencies []Dependency `json:"dependencies,omitempty"`
	Comments     []Comment    `json:"comments,omitempty"`
	Gates        []Gate       `json:"gates,omitempty"`

	// Computed counts (cosmetic; the UI recomputes its own).
	DependencyCount int `json:"dependency_count"`
	DependentCount  int `json:"dependent_count"`
	CommentCount    int `json:"comment_count"`
	GateCount       int `json:"gate_count"`
}

// Gate status + type values.
const (
	GatePending  = "pending"
	GateVerified = "verified"

	// GateCommand is verified by the CLI running Command locally and reporting
	// exit 0. Human-approval and server-executed gate types are intended future
	// additions — the Type field keeps the model open to them.
	GateCommand = "command"
)

// Gate is a verifiable acceptance criterion attached to a task. A task cannot be
// closed while any of its gates are still pending. A command gate is verified
// out-of-band by the CLI (never the API/MCP): the CLI runs Command locally and,
// on exit 0, records the exit code + captured output as tamper-evident
// Evidence. See core.BeginVerify / core.CompleteVerify.
type Gate struct {
	ID          string `json:"id"`
	IssueID     string `json:"issue_id,omitempty"`
	Type        string `json:"type"`                  // GateCommand (future: human, etc.)
	Description string `json:"description,omitempty"` // what passing this proves
	Command     string `json:"command,omitempty"`     // for GateCommand: the shell command
	Status      string `json:"status"`                // GatePending | GateVerified
	VerifiedAt  string `json:"verified_at,omitempty"`
	VerifiedBy  string `json:"verified_by,omitempty"`
	ExitCode    *int   `json:"exit_code,omitempty"` // command exit status recorded at verify
	Evidence    string `json:"evidence,omitempty"`  // captured output excerpt
	CreatedAt   string `json:"created_at,omitempty"`
}

// Dependency is an edge from IssueID -> DependsOnID of a given Type.
type Dependency struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
	CreatedAt   string `json:"created_at,omitempty"`
	CreatedBy   string `json:"created_by,omitempty"`
	Metadata    string `json:"metadata,omitempty"` // JSON encoded as a string, e.g. "{}"
}

// Comment is a threaded note on a task.
type Comment struct {
	ID        string `json:"id"`
	IssueID   string `json:"issue_id"`
	Author    string `json:"author,omitempty"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at,omitempty"`
}

// VerifyChallenge is one pending command gate handed to the CLI by BeginVerify:
// a one-time token (single-use, short-lived) plus the command to run. The CLI
// runs Command locally and posts the exit code + output back with Token. This
// wire type is shared verbatim by core, the HTTP surface and the CLI.
type VerifyChallenge struct {
	GateID      string `json:"gate_id"`
	Type        string `json:"type"`
	Command     string `json:"command,omitempty"`
	Description string `json:"description,omitempty"`
	Token       string `json:"token"`
	Expires     string `json:"expires"` // RFC3339
}

// VerifySession is the response to "begin verification": the challenges to run
// and an LLM-friendly message describing the next step.
type VerifySession struct {
	TaskID     string            `json:"task_id"`
	Challenges []VerifyChallenge `json:"challenges"`
	Message    string            `json:"message"`
}

// VerifyResult is the CLI's report of running one gate's command: the one-time
// token it was issued, the process exit code, and captured output (evidence).
type VerifyResult struct {
	Token    string `json:"token"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
}

// MarshalLabels encodes labels as the JSON text used in the store's labels
// column (empty string when there are none).
func MarshalLabels(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	b, _ := json.Marshal(labels)
	return string(b)
}

// PriorityOr returns the priority value or the given default when unset.
func (t *Task) PriorityOr(def int) int {
	if t.Priority == nil {
		return def
	}
	return *t.Priority
}

// TypeString returns t.Type or "" when nil.
func (t *Task) TypeString() string {
	if t.Type == nil {
		return ""
	}
	return *t.Type
}

// ReadJSONL parses a beads-style issues.jsonl stream into tasks, skipping blank
// lines. It does not fail the whole stream on a single bad line.
func ReadJSONL(r io.Reader) ([]Task, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024) // records can be large
	var out []Task
	for sc.Scan() {
		line := sc.Bytes()
		if len(trimSpace(line)) == 0 {
			continue
		}
		var t Task
		if err := json.Unmarshal(line, &t); err != nil {
			return out, err
		}
		out = append(out, t)
	}
	return out, sc.Err()
}

// WriteJSONL writes tasks as newline-delimited JSON in a stable id order.
func WriteJSONL(w io.Writer, tasks []Task) error {
	sorted := append([]Task(nil), tasks...)
	sort.Slice(sorted, func(i, j int) bool { return NaturalLess(sorted[i].ID, sorted[j].ID) })
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	for i := range sorted {
		if err := enc.Encode(&sorted[i]); err != nil {
			return err
		}
	}
	return bw.Flush()
}

func trimSpace(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\t' || b[i] == '\r' || b[i] == '\n') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\t' || b[j-1] == '\r' || b[j-1] == '\n') {
		j--
	}
	return b[i:j]
}
