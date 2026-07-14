package core

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tristanMatthias/tasks/pkg/model"
	"github.com/tristanMatthias/tasks/pkg/store"
)

// CreateParams describes a new task (subset of `bd create`).
type CreateParams struct {
	Title              string
	Description        string
	Priority           *int   // default 2
	IssueType          string // default "task"
	Status             string // default "open"
	Assignee           string
	Parent             string   // parent id -> parent-child dep + hierarchical id
	Deps               []string // "type:id" or "id" (default type "blocks")
	Labels             []string
	Design             string
	AcceptanceCriteria string
	Notes              string
	ID                 string // explicit id (optional)
	Actor              string // overrides default actor
}

// Create mints an id, applies defaults and dependencies, and inserts the task.
func (c *Core) Create(p CreateParams) (*model.Task, error) {
	actor := c.actorOr(p.Actor)
	now := c.now()

	// Expand a short parent id ("w7t0" -> "tasks-w7t0") before it drives the
	// child id or the parent-child dependency.
	if p.Parent != "" {
		p.Parent = c.resolveID(p.Parent)
	}

	id := p.ID
	if id == "" {
		var err error
		if p.Parent != "" {
			id, err = c.childID(p.Parent)
		} else {
			id, err = c.freshID()
		}
		if err != nil {
			return nil, err
		}
	}

	prio := p.Priority
	if prio == nil {
		prio = intPtr(2)
	}
	itype := p.IssueType
	if itype == "" {
		itype = "task"
	}
	status := p.Status
	if status == "" {
		status = "open"
	}

	t := &model.Task{
		ID:                 id,
		Title:              p.Title,
		Description:        p.Description,
		Status:             status,
		Priority:           prio,
		IssueType:          itype,
		Owner:              actor,
		Assignee:           p.Assignee,
		CreatedBy:          actor,
		CreatedAt:          now,
		UpdatedAt:          now,
		AcceptanceCriteria: p.AcceptanceCriteria,
		Design:             p.Design,
		Notes:              p.Notes,
		Labels:             p.Labels,
	}

	// Parent-child dependency (p.Parent already resolved above).
	if p.Parent != "" {
		ok, err := c.st.Exists(p.Parent)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("parent not found: %s", p.Parent)
		}
		t.Dependencies = append(t.Dependencies, model.Dependency{
			IssueID: id, DependsOnID: p.Parent, Type: "parent-child",
			CreatedAt: now, CreatedBy: actor, Metadata: "{}",
		})
	}
	// Other dependencies.
	for _, spec := range p.Deps {
		dtype, depID := parseDepSpec(spec)
		t.Dependencies = append(t.Dependencies, model.Dependency{
			IssueID: id, DependsOnID: depID, Type: dtype,
			CreatedAt: now, CreatedBy: actor, Metadata: "{}",
		})
	}

	if err := c.st.Insert(t); err != nil {
		return nil, wrap("create", err)
	}
	c.changed(id)
	return c.st.Get(id)
}

// freshID returns an unused prefix-<suffix> id.
func (c *Core) freshID() (string, error) {
	if c.prefix == "" {
		return "", fmt.Errorf("no issue prefix configured")
	}
	for i := 0; i < 100; i++ {
		id := c.prefix + "-" + randSuffix(4)
		ok, err := c.st.Exists(id)
		if err != nil {
			return "", err
		}
		if !ok {
			return id, nil
		}
	}
	return "", fmt.Errorf("could not mint a unique id")
}

// childID returns the next "<parent>.N" id.
func (c *Core) childID(parent string) (string, error) {
	children, err := c.st.List(store.Filter{Parent: parent})
	if err != nil {
		return "", err
	}
	ids := make([]string, len(children))
	for i := range children {
		ids[i] = children[i].ID
	}
	return fmt.Sprintf("%s.%d", parent, nextChildIndex(parent, ids)), nil
}

func parseDepSpec(spec string) (dtype, id string) {
	spec = strings.TrimSpace(spec)
	if i := strings.IndexByte(spec, ':'); i >= 0 {
		return spec[:i], spec[i+1:]
	}
	return "blocks", spec
}

// UpdateParams describes a task mutation (subset of `bd update`).
type UpdateParams struct {
	Title              *string
	Description        *string
	Status             *string
	Priority           *int
	IssueType          *string
	Assignee           *string
	Design             *string
	AcceptanceCriteria *string
	Notes              *string
	AppendNotes        string
	SetLabels          *[]string
	AddLabels          []string
	RemoveLabels       []string
	Claim              bool
	Actor              string
}

// Update applies field changes and/or an atomic claim, returning the new task.
func (c *Core) Update(id string, p UpdateParams) (*model.Task, error) {
	id = c.resolveID(id)
	actor := c.actorOr(p.Actor)
	now := c.now()

	if p.Claim {
		claimed, err := c.st.Claim(id, actor, now, now)
		if err != nil {
			return nil, wrap("claim", err)
		}
		if !claimed {
			// Held by another actor (not found already surfaced as error).
			return nil, fmt.Errorf("issue %s is already claimed by someone else", id)
		}
	}

	set := map[string]any{}
	if p.Title != nil {
		set["title"] = *p.Title
	}
	if p.Description != nil {
		set["description"] = *p.Description
	}
	if p.Status != nil {
		set["status"] = *p.Status
		if *p.Status == "closed" {
			set["closed_at"] = now
		}
	}
	if p.Priority != nil {
		set["priority"] = *p.Priority
	}
	if p.IssueType != nil {
		set["issue_type"] = *p.IssueType
	}
	if p.Assignee != nil {
		set["assignee"] = *p.Assignee
	}
	if p.Design != nil {
		set["design"] = *p.Design
	}
	if p.AcceptanceCriteria != nil {
		set["acceptance_criteria"] = *p.AcceptanceCriteria
	}
	if p.Notes != nil {
		set["notes"] = *p.Notes
	}

	// Label edits require the current set.
	if p.SetLabels != nil || len(p.AddLabels) > 0 || len(p.RemoveLabels) > 0 {
		cur, err := c.st.Get(id)
		if err != nil {
			return nil, err
		}
		labels := cur.Labels
		if p.SetLabels != nil {
			labels = *p.SetLabels
		}
		labels = addLabels(labels, p.AddLabels)
		labels = removeLabels(labels, p.RemoveLabels)
		set["labels"] = model.MarshalLabels(labels)
	}

	if p.AppendNotes != "" {
		cur, err := c.st.Get(id)
		if err != nil {
			return nil, err
		}
		notes := cur.Notes
		if notes != "" {
			notes += "\n"
		}
		set["notes"] = notes + p.AppendNotes
	}

	if len(set) > 0 {
		if err := c.st.Patch(id, set, now); err != nil {
			return nil, wrap("update", err)
		}
	}
	if p.Claim || len(set) > 0 {
		c.changed(id)
	}
	return c.st.Get(id)
}

// CloseParams describes closing a task.
type CloseParams struct {
	Reason string
	Actor  string
}

// Close marks a task closed with closed_at=now and an optional reason.
func (c *Core) Close(id string, p CloseParams) (*model.Task, error) {
	id = c.resolveID(id)
	// Acceptance gates block close: a task can't be closed while any command
	// gate is still pending. The error names them + the CLI command to verify.
	gates, err := c.st.ListGates(id)
	if err != nil {
		return nil, err
	}
	if pending := pendingGates(gates); len(pending) > 0 {
		return nil, &GatesPendingError{TaskID: id, Gates: pending}
	}
	now := c.now()
	set := map[string]any{"status": "closed", "closed_at": now}
	if p.Reason != "" {
		set["close_reason"] = p.Reason
	}
	if err := c.st.Patch(id, set, now); err != nil {
		return nil, wrap("close", err)
	}
	c.changed(id)
	return c.st.Get(id)
}

// AddDep creates a dependency: `dep add <blocked> <blocker>` means blocked
// depends on blocker (blocker blocks blocked). dtype defaults to "blocks".
func (c *Core) AddDep(blocked, blocker, dtype, actor string) error {
	blocked, blocker = c.resolveID(blocked), c.resolveID(blocker)
	if dtype == "" {
		dtype = "blocks"
	}
	for _, id := range []string{blocked, blocker} {
		ok, err := c.st.Exists(id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("issue not found: %s", id)
		}
	}
	if err := c.st.AddDependency(model.Dependency{
		IssueID: blocked, DependsOnID: blocker, Type: dtype,
		CreatedAt: c.now(), CreatedBy: c.actorOr(actor), Metadata: "{}",
	}); err != nil {
		return err
	}
	c.changed(blocked, blocker)
	return nil
}

// Delete permanently removes a task (and its dependencies + comments).
func (c *Core) Delete(id string) error {
	id = c.resolveID(id)
	if err := c.st.Delete(id); err != nil {
		return err
	}
	c.changed(id)
	return nil
}

// Comment appends a comment to a task.
func (c *Core) Comment(id, text, author string) (*model.Task, error) {
	id = c.resolveID(id)
	ok, err := c.st.Exists(id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	cm := model.Comment{
		ID: uuid.NewString(), IssueID: id, Author: c.actorOr(author),
		Text: text, CreatedAt: c.now(),
	}
	if err := c.st.AddComment(cm); err != nil {
		return nil, wrap("comment", err)
	}
	c.changed(id)
	return c.st.Get(id)
}

// Note appends text to a task's notes field (convenience wrapper).
func (c *Core) Note(id, text, actor string) (*model.Task, error) {
	return c.Update(id, UpdateParams{AppendNotes: text, Actor: actor})
}

func (c *Core) actorOr(a string) string {
	if a != "" {
		return a
	}
	return c.actor
}

func addLabels(cur, add []string) []string {
	have := map[string]bool{}
	for _, l := range cur {
		have[l] = true
	}
	for _, l := range add {
		if !have[l] {
			cur = append(cur, l)
			have[l] = true
		}
	}
	return cur
}

func removeLabels(cur, rm []string) []string {
	if len(rm) == 0 {
		return cur
	}
	drop := map[string]bool{}
	for _, l := range rm {
		drop[l] = true
	}
	out := cur[:0]
	for _, l := range cur {
		if !drop[l] {
			out = append(out, l)
		}
	}
	return out
}
