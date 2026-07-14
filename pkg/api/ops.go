package api

import (
	"github.com/tristanMatthias/tasks/pkg/core"
	"github.com/tristanMatthias/tasks/pkg/store"
)

// ---- input structs (single definition of each command's parameters) ----
//
// Tags: json=wire name, in=http location (body|query|path), cli=flag/positional,
// desc=help/schema description, req=required.

type ReadyInput struct {
	Limit    int    `json:"limit"    in:"query" cli:"-n,--limit"    desc:"Max results (default 10)"`
	Assignee string `json:"assignee" in:"query" cli:"-a,--assignee" desc:"Filter by assignee"`
	Type     string `json:"type"     in:"query" cli:"-t,--type"     desc:"Filter by issue type"`
	Priority *int   `json:"priority" in:"query" cli:"-p,--priority" desc:"Filter by priority (0-4)"`
	Parent   string `json:"parent"   in:"query" cli:"--parent"      desc:"Only descendants of this id"`
}

type ListInput struct {
	Status   string `json:"status"   in:"query" cli:"-s,--status"   desc:"Filter by status (CSV allowed)"`
	Type     string `json:"type"     in:"query" cli:"-t,--type"     desc:"Filter by issue type (CSV allowed)"`
	Assignee string `json:"assignee" in:"query" cli:"-a,--assignee" desc:"Filter by assignee"`
	Label    string `json:"label"    in:"query" cli:"-l,--label"    desc:"Filter by label (CSV = AND)"`
	Parent   string `json:"parent"   in:"query" cli:"--parent"      desc:"Only direct children of this task id"`
	Limit    int    `json:"limit"    in:"query" cli:"-n,--limit"    desc:"Max results"`
}

type ShowInput struct {
	ID string `json:"id" in:"path" cli:"arg" req:"true" desc:"Task id"`
}

type SearchInput struct {
	Query  string `json:"query"  in:"query" cli:"arg..." req:"true" desc:"Text to fuzzy-search (title/id/labels/description)"`
	Status string `json:"status" in:"query" cli:"-s,--status" desc:"Filter by status (CSV allowed)"`
	Type   string `json:"type"   in:"query" cli:"-t,--type"   desc:"Filter by issue type"`
	Limit  int    `json:"limit"  in:"query" cli:"-n,--limit"  desc:"Max results (default 20)"`
}

type TreeInput struct {
	ID string `json:"id" in:"query" cli:"arg" desc:"Root task id (optional; omit to show the whole forest)"`
}

type CreateInput struct {
	Title              string   `json:"title"               in:"body" cli:"arg..." req:"true" desc:"Task title"`
	Description        string   `json:"description"         in:"body" cli:"-d,--description" desc:"Description"`
	Priority           *int     `json:"priority"            in:"body" cli:"-p,--priority"    desc:"Priority 0-4 (default 2)"`
	IssueType          string   `json:"issue_type"          in:"body" cli:"-t,--type"        desc:"epic|feature|task|bug|chore (default task)"`
	Parent             string   `json:"parent"              in:"body" cli:"--parent"         desc:"Parent id (hierarchical child)"`
	Deps               []string `json:"deps"                in:"body" cli:"--deps"           desc:"Dependencies 'type:id' or 'id'"`
	Labels             []string `json:"labels"              in:"body" cli:"-l,--labels"      desc:"Labels (CSV)"`
	Assignee           string   `json:"assignee"            in:"body" cli:"-a,--assignee"    desc:"Assignee"`
	Design             string   `json:"design"              in:"body" cli:"--design"         desc:"Design notes"`
	AcceptanceCriteria string   `json:"acceptance_criteria" in:"body" cli:"--acceptance"     desc:"Acceptance criteria"`
	Notes              string   `json:"notes"               in:"body" cli:"--notes"          desc:"Notes"`
}

type UpdateInput struct {
	ID           string   `json:"id"           in:"path" cli:"arg" req:"true" desc:"Task id"`
	Claim        bool     `json:"claim"        in:"body" cli:"--claim"        desc:"Atomically claim (assignee=you, in_progress)"`
	Status       *string  `json:"status"       in:"body" cli:"-s,--status"    desc:"New status"`
	Assignee     *string  `json:"assignee"     in:"body" cli:"-a,--assignee"  desc:"New assignee"`
	Title        *string  `json:"title"        in:"body" cli:"--title"        desc:"New title"`
	Description  *string  `json:"description"  in:"body" cli:"-d,--description" desc:"New description"`
	Priority     *int     `json:"priority"     in:"body" cli:"-p,--priority"  desc:"New priority 0-4"`
	IssueType    *string  `json:"issue_type"   in:"body" cli:"-t,--type"      desc:"New issue type"`
	Notes        *string  `json:"notes"        in:"body" cli:"--notes"        desc:"Replace notes"`
	AppendNotes  string   `json:"append_notes" in:"body" cli:"--append-notes" desc:"Append to notes"`
	AddLabels    []string `json:"add_labels"   in:"body" cli:"--add-label"    desc:"Add labels (CSV)"`
	RemoveLabels []string `json:"remove_labels" in:"body" cli:"--remove-label" desc:"Remove labels (CSV)"`
}

type ClaimInput struct {
	ID string `json:"id" in:"path" cli:"arg" req:"true" desc:"Task id"`
}

type CloseInput struct {
	ID     string `json:"id"     in:"path" cli:"arg" req:"true" desc:"Task id"`
	Reason string `json:"reason" in:"body" cli:"-r,--reason"    desc:"Reason for closing"`
}

type DepInput struct {
	Blocked string `json:"blocked" in:"body" cli:"arg" req:"true" desc:"Task that is blocked (depends on the blocker)"`
	Blocker string `json:"blocker" in:"body" cli:"arg" req:"true" desc:"Task that blocks"`
	Type    string `json:"type"    in:"body" cli:"--type"         desc:"Dependency type (default blocks)"`
}

type CommentInput struct {
	ID   string `json:"id"   in:"path" cli:"arg"    req:"true" desc:"Task id"`
	Text string `json:"text" in:"body" cli:"arg..." req:"true" desc:"Comment text"`
}

type GateAddInput struct {
	ID          string `json:"id"          in:"path" cli:"arg"             req:"true" desc:"Task id"`
	Command     string `json:"command"     in:"body" cli:"-c,--command"    req:"true" desc:"Shell command the CLI runs to verify this gate (exit 0 = pass)"`
	Description string `json:"description" in:"body" cli:"-d,--description" desc:"What passing this gate proves (shown to the operator)"`
	Type        string `json:"type"        in:"body" cli:"-t,--type"       desc:"Gate type (default: command)"`
}

type GateRemoveInput struct {
	ID   string `json:"id"   in:"path" cli:"arg" req:"true" desc:"Task id"`
	Gate string `json:"gate" in:"path" cli:"arg" req:"true" desc:"Gate id (e.g. g1)"`
}

// VerifyInput drives the CLI-only `verify` op. It has no server handler: the CLI
// runs each gate's command locally, then posts results to the dedicated
// begin/complete endpoints. See cmd/tasks.
type VerifyInput struct {
	ID   string `json:"id"   in:"path"  cli:"arg"        req:"true" desc:"Task id"`
	Gate string `json:"gate" in:"query" cli:"arg"        desc:"Only verify this gate id (default: all pending)"`
	Yes  bool   `json:"yes"  in:"query" cli:"-y,--yes"   desc:"Run gate commands without confirmation"`
}

type KeysListInput struct{}

type KeysCreateInput struct {
	Label string `json:"label" in:"body" cli:"arg..." desc:"Human label for the key (e.g. 'ci', 'claude-web')"`
}

type KeysRevokeInput struct {
	ID string `json:"id" in:"path" cli:"arg" req:"true" desc:"Key id to revoke"`
}

// ---- the registry: THE single source of truth for every surface ----

// Ops returns the operation registry. Order controls CLI help ordering.
func Ops() Registry {
	return Registry{
		{
			Name: "ready", Summary: "List claimable work (open, unblocked), priority-ordered. Default limit 10.",
			Method: "GET", Path: "/api/v1/ready", List: true, Proto: &ReadyInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				p := in.(*ReadyInput)
				return c.Ready(core.ReadyOptions{Limit: p.Limit, Assignee: p.Assignee, Type: p.Type, Priority: p.Priority, Parent: p.Parent})
			},
		},
		{
			Name: "list", Aliases: []string{"ls"}, Summary: "List tasks filtered by status/type/assignee/label.",
			Method: "GET", Path: "/api/v1/tasks", List: true, Proto: &ListInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				p := in.(*ListInput)
				f := store.Filter{Assignee: p.Assignee, Parent: p.Parent, Limit: p.Limit}
				if p.Status != "" {
					f.Statuses = splitCSV(p.Status)
				}
				if p.Type != "" {
					f.Types = splitCSV(p.Type)
				}
				if p.Label != "" {
					f.Labels = splitCSV(p.Label)
				}
				return c.List(f)
			},
		},
		{
			Name: "show", Aliases: []string{"view"}, Summary: "Show a task's full details (deps, comments).",
			Method: "GET", Path: "/api/v1/tasks/{id}", Proto: &ShowInput{},
			Handle: func(c *core.Core, in any) (any, error) { return c.Show(in.(*ShowInput).ID) },
		},
		{
			Name: "search", Aliases: []string{"find"}, Summary: "Fuzzy-search tasks by text, best matches first.",
			Method: "GET", Path: "/api/v1/search", List: true, Proto: &SearchInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				p := in.(*SearchInput)
				o := core.SearchOptions{Type: p.Type, Limit: p.Limit}
				if o.Limit == 0 {
					o.Limit = 20
				}
				if p.Status != "" {
					o.Statuses = splitCSV(p.Status)
				}
				return c.Search(p.Query, o)
			},
		},
		{
			Name: "tree", Summary: "Show a task's subtree, or the whole forest if no id is given.",
			Method: "GET", Path: "/api/v1/tree", List: true, Proto: &TreeInput{},
			Handle: func(c *core.Core, in any) (any, error) { return c.Tree(in.(*TreeInput).ID) },
		},
		{
			Name: "create", Aliases: []string{"new"}, Summary: "Create a task; returns it with a minted id.",
			Method: "POST", Path: "/api/v1/tasks", Proto: &CreateInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				p := in.(*CreateInput)
				return c.Create(core.CreateParams{
					Title: p.Title, Description: p.Description, Priority: p.Priority, IssueType: p.IssueType,
					Parent: p.Parent, Deps: p.Deps, Labels: p.Labels, Assignee: p.Assignee,
					Design: p.Design, AcceptanceCriteria: p.AcceptanceCriteria, Notes: p.Notes,
				})
			},
		},
		{
			Name: "update", Summary: "Update task fields and/or atomically claim it.",
			Method: "PATCH", Path: "/api/v1/tasks/{id}", Proto: &UpdateInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				p := in.(*UpdateInput)
				return c.Update(p.ID, core.UpdateParams{
					Claim: p.Claim, Status: p.Status, Assignee: p.Assignee, Title: p.Title,
					Description: p.Description, Priority: p.Priority, IssueType: p.IssueType,
					Notes: p.Notes, AppendNotes: p.AppendNotes, AddLabels: p.AddLabels, RemoveLabels: p.RemoveLabels,
				})
			},
		},
		{
			Name: "claim", Summary: "Atomically claim a task (assignee=you, in_progress).",
			Method: "POST", Path: "/api/v1/tasks/{id}/claim", Proto: &ClaimInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				return c.Update(in.(*ClaimInput).ID, core.UpdateParams{Claim: true})
			},
		},
		{
			Name: "close", Aliases: []string{"done"},
			Summary: "Close a task. Blocked while it has unverified acceptance gates — the CLI must verify them first (tasks verify <id>); the API/MCP cannot.",
			Method:  "POST", Path: "/api/v1/tasks/{id}/close", Proto: &CloseInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				p := in.(*CloseInput)
				return c.Close(p.ID, core.CloseParams{Reason: p.Reason})
			},
		},
		{
			Name: "dep", Summary: "Add a dependency: 'blocked' depends on 'blocker'. Type defaults to blocks.",
			Method: "POST", Path: "/api/v1/deps", Proto: &DepInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				p := in.(*DepInput)
				if err := c.AddDep(p.Blocked, p.Blocker, p.Type, ""); err != nil {
					return nil, err
				}
				return map[string]any{"ok": true, "blocked": p.Blocked, "blocker": p.Blocker}, nil
			},
		},
		{
			Name: "comment", Summary: "Add a comment to a task.",
			Method: "POST", Path: "/api/v1/tasks/{id}/comments", Proto: &CommentInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				p := in.(*CommentInput)
				return c.Comment(p.ID, p.Text, "")
			},
		},
		{
			Name: "gate add",
			Summary: "Add an acceptance gate to a task: a command the CLI must run (exit 0) before the task can be closed.",
			Method:  "POST", Path: "/api/v1/tasks/{id}/gates", Proto: &GateAddInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				p := in.(*GateAddInput)
				return c.AddGate(p.ID, core.GateSpec{Type: p.Type, Command: p.Command, Description: p.Description})
			},
		},
		{
			Name: "gate rm", Aliases: []string{"gate remove"},
			Summary: "Remove an acceptance gate from a task.",
			// HideMCP: the working agent must not drop its own gates via MCP tools
			// to bypass verification. Still available on the CLI + HTTP.
			HideMCP: true,
			Method:  "DELETE", Path: "/api/v1/tasks/{id}/gates/{gate}", Proto: &GateRemoveInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				p := in.(*GateRemoveInput)
				return c.RemoveGate(p.ID, p.Gate)
			},
		},
		{
			Name: "verify",
			Summary: "Verify a task's command gates by running them locally (CLI only). Runs each pending gate's command; on exit 0 it records the result so the task can be closed. The API/MCP cannot mark gates verified.",
			// Local: no server route. The CLI executes commands on the host and
			// calls the dedicated begin/complete endpoints. Never an MCP tool.
			Local: true, Proto: &VerifyInput{},
		},
		{
			Name: "keys list", Aliases: []string{"keys"}, Summary: "List API keys (for bots/agents) — active and revoked.",
			Method: "GET", Path: "/api/v1/keys", List: true, Proto: &KeysListInput{},
			Handle: func(c *core.Core, in any) (any, error) { return c.ListKeys() },
		},
		{
			Name: "keys create", Summary: "Mint an API key for a bot/agent. The secret is shown ONCE.",
			Method: "POST", Path: "/api/v1/keys", Proto: &KeysCreateInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				return c.CreateKey(in.(*KeysCreateInput).Label, "")
			},
		},
		{
			Name: "keys revoke", Summary: "Revoke an API key by id (immediately stops working).",
			Method: "POST", Path: "/api/v1/keys/{id}/revoke", Proto: &KeysRevokeInput{},
			Handle: func(c *core.Core, in any) (any, error) {
				return c.RevokeKey(in.(*KeysRevokeInput).ID)
			},
		},
	}
}
