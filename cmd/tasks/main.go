// Command tasks is a drop-in CLI for the task server. Every subcommand is
// generated from the shared api registry (internal/api) — the same single
// source of truth that drives the HTTP and MCP surfaces — so commands and
// flags never drift between them. Symlink it as `bd` for backward compat.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/tristanMatthias/tasks/pkg/api"
	"github.com/tristanMatthias/tasks/pkg/model"
)

func main() {
	if err := Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// Run dispatches a CLI invocation. Returns an error for a failed command or an
// unknown/absent command (so main can set the exit code). Split from main for
// testability.
func Run(args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("no command given")
	}
	cmd, rest := args[0], args[1:]

	switch cmd {
	case "prime", "onboard":
		fmt.Print(primeText)
		return nil
	case "-h", "--help", "help":
		usage()
		return nil
	}

	// Support grouped commands like "keys create": try a two-word op name first,
	// then fall back to the single-word command.
	var op *api.Op
	if len(args) >= 2 {
		if o := api.Ops().Lookup(args[0] + " " + args[1]); o != nil {
			op, rest = o, args[2:]
		}
	}
	if op == nil {
		op = api.Ops().Lookup(cmd)
	}
	if op == nil {
		usage()
		return fmt.Errorf("unknown command: %s", cmd)
	}
	return run(op, rest)
}

func run(op *api.Op, args []string) error {
	// Per-command help: `tasks <cmd> --help` prints the op's flags/args from the
	// same tagged definition that drives every surface.
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Print(op.Usage())
			return nil
		}
	}

	// Global output flags are not part of any op's input.
	jsonOut, silent, args := extractGlobalFlags(args)
	// Tolerate a leading "add" for the dep command (bd compatibility).
	if op.Name == "dep" && len(args) > 0 && args[0] == "add" {
		args = args[1:]
	}

	in, err := op.ParseCLI(args)
	if err != nil {
		return err
	}
	if err := api.Validate(op.Fields(), in); err != nil {
		return err
	}
	// Local ops (e.g. verify) run in-process — they execute commands on this
	// machine instead of proxying to the server. This is the CLI-only path.
	if op.Local {
		return runLocal(op, in)
	}
	path, query, body := api.EncodeRequest(op.Fields(), in, op.Method, op.Path)

	c := newClient()
	data, err := c.request(op.Method, path, query, body)
	if err != nil {
		return err
	}

	if jsonOut {
		printJSONBytes(data)
		return nil
	}
	return printHuman(op, data, silent)
}

// extractGlobalFlags removes --json and --silent from args (they are shared
// across all commands and not part of op inputs).
func extractGlobalFlags(args []string) (jsonOut, silent bool, out []string) {
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "--silent":
			silent = true
		default:
			out = append(out, a)
		}
	}
	return jsonOut, silent, out
}

// ---- output ----

func printHuman(op *api.Op, data []byte, silent bool) error {
	if strings.HasPrefix(op.Name, "keys") {
		return printKeys(op, data, silent)
	}
	if op.List {
		var tasks []model.Task
		if err := json.Unmarshal(data, &tasks); err != nil {
			return err
		}
		if op.Name == "tree" {
			printTree(tasks)
		} else {
			printTasks(tasks)
		}
		return nil
	}
	switch op.Name {
	case "dep":
		fmt.Println("dependency added")
		return nil
	}
	var t model.Task
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	switch op.Name {
	case "show":
		printTaskDetail(&t)
	case "create":
		if silent {
			fmt.Println(t.ID)
		} else {
			fmt.Printf("created %s  %s\n", t.ID, t.Title)
		}
	case "update", "claim":
		fmt.Printf("updated %s  status=%s assignee=%s\n", t.ID, t.Status, t.Assignee)
	case "close":
		fmt.Printf("closed %s\n", t.ID)
	case "comment":
		fmt.Printf("comment added to %s\n", t.ID)
	default:
		printTaskDetail(&t)
	}
	return nil
}

// printKeys renders the API-key commands. "keys create" prints the one-time
// secret prominently; "keys list" tabulates; "keys revoke" confirms.
func printKeys(op *api.Op, data []byte, silent bool) error {
	switch op.Name {
	case "keys create":
		var k model.APIKey
		if err := json.Unmarshal(data, &k); err != nil {
			return err
		}
		if silent {
			fmt.Println(k.Secret)
			return nil
		}
		fmt.Printf("created key %s  %s\n", k.ID, k.Label)
		fmt.Printf("\n  %s\n\n", k.Secret)
		fmt.Println("Store it now — the secret is shown only once.")
		fmt.Println("Use it as: TASKS_TOKEN=<secret> tasks ready")
		return nil
	case "keys revoke":
		var k model.APIKey
		if err := json.Unmarshal(data, &k); err != nil {
			return err
		}
		fmt.Printf("revoked key %s  %s\n", k.ID, k.Label)
		return nil
	default: // keys list
		var keys []model.APIKey
		if err := json.Unmarshal(data, &keys); err != nil {
			return err
		}
		if len(keys) == 0 {
			fmt.Println("(no keys)")
			return nil
		}
		idW, lbW := 0, 0
		for i := range keys {
			idW = max(idW, len(keys[i].ID))
			lbW = max(lbW, len(keys[i].Label))
		}
		for i := range keys {
			k := &keys[i]
			state := "active"
			if k.Revoked() {
				state = "revoked"
			}
			last := "never"
			if k.LastUsedAt != "" {
				last = dateOnly(k.LastUsedAt)
			}
			fmt.Printf("%-*s  %-*s  %-7s  created %s  last-used %s\n",
				idW, k.ID, lbW, orDash(k.Label), state, dateOnly(k.CreatedAt), last)
		}
		return nil
	}
}

func printTasks(tasks []model.Task) {
	if len(tasks) == 0 {
		fmt.Println("(no tasks)")
		return
	}
	// Size columns to the widest value so rows stay aligned regardless of id length.
	idW, stW, tyW := 0, 0, 0
	for i := range tasks {
		idW = max(idW, len(tasks[i].ID))
		stW = max(stW, len(tasks[i].Status))
		tyW = max(tyW, len(tasks[i].IssueType))
	}
	for i := range tasks {
		t := &tasks[i]
		fmt.Printf("%-*s  P%s  %-*s  %-*s  %s\n", idW, t.ID, prioStr(t), stW, t.Status, tyW, t.IssueType, t.Title)
	}
}

// printTree renders a root task and its child subtree with box-drawing
// connectors, building the hierarchy from the parent-child deps in the set.
func printTree(tasks []model.Task) {
	if len(tasks) == 0 {
		fmt.Println("(no tasks)")
		return
	}
	byID := make(map[string]*model.Task, len(tasks))
	for i := range tasks {
		byID[tasks[i].ID] = &tasks[i]
	}
	children := map[string][]string{}
	hasParent := map[string]bool{}
	for i := range tasks {
		for _, d := range tasks[i].Dependencies {
			if (d.Type == "parent-child" || d.Type == "parent") && byID[d.DependsOnID] != nil {
				children[d.DependsOnID] = append(children[d.DependsOnID], tasks[i].ID)
				hasParent[tasks[i].ID] = true
			}
		}
	}
	nat := func(ids []string) {
		sort.SliceStable(ids, func(a, b int) bool { return model.NaturalLess(ids[a], ids[b]) })
	}
	for k := range children {
		nat(children[k])
	}
	var roots []string
	for i := range tasks {
		if !hasParent[tasks[i].ID] {
			roots = append(roots, tasks[i].ID)
		}
	}
	nat(roots)

	var walk func(id, prefix string, isLast, isRoot bool)
	walk = func(id, prefix string, isLast, isRoot bool) {
		t := byID[id]
		if t == nil {
			return
		}
		connector, childPrefix := "", prefix
		if !isRoot {
			if isLast {
				connector, childPrefix = "└─ ", prefix+"   "
			} else {
				connector, childPrefix = "├─ ", prefix+"│  "
			}
		}
		fmt.Printf("%s%s%s %s  P%s  %-8s %s\n",
			prefix, connector, statusGlyph(t.Status), t.ID, prioStr(t), t.IssueType, truncate(t.Title, 80))
		kids := children[id]
		for i, k := range kids {
			walk(k, childPrefix, i == len(kids)-1, false)
		}
	}
	for _, r := range roots {
		walk(r, "", true, true)
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func printTaskDetail(t *model.Task) {
	// Header: <glyph> <id> [TYPE] · <title>   [P# · STATUS]
	fmt.Printf("%s %s [%s] · %s   [P%s · %s]\n",
		statusGlyph(t.Status), t.ID, strings.ToUpper(t.IssueType), t.Title,
		prioStr(t), strings.ToUpper(t.Status))

	// Metadata lines.
	line1 := "Owner: " + orDash(t.Owner) + " · Type: " + t.IssueType
	if t.Assignee != "" {
		line1 += " · Assignee: " + t.Assignee
	}
	fmt.Println(line1)
	var times []string
	if t.CreatedAt != "" {
		times = append(times, "Created: "+dateOnly(t.CreatedAt))
	}
	if t.StartedAt != "" {
		times = append(times, "Started: "+dateOnly(t.StartedAt))
	}
	if t.UpdatedAt != "" {
		times = append(times, "Updated: "+dateOnly(t.UpdatedAt))
	}
	if t.ClosedAt != "" {
		times = append(times, "Closed: "+dateOnly(t.ClosedAt))
	}
	if len(times) > 0 {
		fmt.Println(strings.Join(times, " · "))
	}
	if len(t.Labels) > 0 {
		fmt.Println("Labels: " + strings.Join(t.Labels, ", "))
	}
	if t.CloseReason != "" {
		fmt.Println("Close reason: " + t.CloseReason)
	}

	section("DESCRIPTION", t.Description)
	section("ACCEPTANCE CRITERIA", t.AcceptanceCriteria)
	section("DESIGN", t.Design)
	section("NOTES", t.Notes)

	// Dependencies broken out by kind.
	var parent, blockers, related []string
	for _, d := range t.Dependencies {
		switch d.Type {
		case "parent-child", "parent":
			parent = append(parent, d.DependsOnID)
		case "blocks":
			blockers = append(blockers, d.DependsOnID)
		default:
			related = append(related, d.Type+" → "+d.DependsOnID)
		}
	}
	if len(parent)+len(blockers)+len(related) > 0 || t.DependentCount > 0 {
		fmt.Println("\nDEPENDENCIES")
		for _, p := range parent {
			fmt.Printf("  parent      %s\n", p)
		}
		for _, b := range blockers {
			fmt.Printf("  blocked by  %s\n", b)
		}
		for _, r := range related {
			fmt.Printf("  %s\n", r)
		}
		if t.DependentCount > 0 {
			fmt.Printf("  (%d task(s) depend on this)\n", t.DependentCount)
		}
	}

	if len(t.Comments) > 0 {
		fmt.Printf("\nCOMMENTS (%d)\n", len(t.Comments))
		for _, c := range t.Comments {
			author := c.Author
			if author == "" {
				author = "(unknown)"
			}
			fmt.Printf("  %s  %s\n", dateTime(c.CreatedAt), author)
			for _, ln := range strings.Split(strings.TrimRight(c.Text, "\n"), "\n") {
				fmt.Printf("    %s\n", ln)
			}
		}
	}
}

// section prints an uppercase-titled block if body is non-empty.
func section(title, body string) {
	if strings.TrimSpace(body) == "" {
		return
	}
	fmt.Printf("\n%s\n%s\n", title, body)
}

func statusGlyph(status string) string {
	switch status {
	case "closed":
		return "✓"
	case "in_progress":
		return "◐"
	case "deferred":
		return "◦"
	default: // open
		return "○"
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// dateOnly returns the YYYY-MM-DD portion of an RFC3339 timestamp.
func dateOnly(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

// dateTime returns "YYYY-MM-DD HH:MM" from an RFC3339 timestamp.
func dateTime(ts string) string {
	if len(ts) >= 16 && ts[10] == 'T' {
		return ts[:10] + " " + ts[11:16]
	}
	return dateOnly(ts)
}

func printJSONBytes(data []byte) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		os.Stdout.Write(data)
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func prioStr(t *model.Task) string {
	if t.Priority == nil {
		return "?"
	}
	return strconv.Itoa(*t.Priority)
}

func usage() {
	var b strings.Builder
	b.WriteString("tasks — task tracker CLI (talks to tasksd via TASKS_URL/TASKS_TOKEN)\n\nCommands:\n")
	for _, op := range api.Ops() {
		name := op.Name
		if len(op.Aliases) > 0 {
			name += "/" + strings.Join(op.Aliases, "/")
		}
		fmt.Fprintf(&b, "  %-10s %s\n", name, op.Summary)
	}
	b.WriteString("  prime      Print workflow context\n")
	b.WriteString("\nRun 'tasks <command> --help' for a command's flags.\n")
	b.WriteString("Global: --json (raw JSON output), --silent (create: print id only)\n")
	b.WriteString("Env: TASKS_URL (default http://127.0.0.1:7842), TASKS_TOKEN\n")
	fmt.Print(b.String())
}
