// Package core implements the task-tracking business logic — the subset of the
// beads `bd` command surface that the agents actually use: ready, show, create,
// update (incl. --claim), close, list, dep, comment/note. It sits on top of the
// store and owns id minting, timestamps and ready/blocker semantics.
package core

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sahilm/fuzzy"
	"github.com/tristanMatthias/tasks/pkg/model"
	"github.com/tristanMatthias/tasks/pkg/store"
)

// Core is the domain service.
type Core struct {
	st          *store.Store
	prefix      string
	actor       string
	keySelector string // opaque routing prefix embedded in minted API tokens (empty in single-tenant)
	nowFunc     func() time.Time
	onChange    func(ids []string) // fired after every successful mutation, any surface
}

// SetOnChange registers a hook invoked after each successful mutation (create,
// update, claim, close, dep, comment). It receives the ids of the task(s)
// affected, so a listener can push a targeted live update. Used to drive the UI
// mtime signal, the debounced JSONL exporter and WebSocket broadcasts uniformly
// across HTTP, MCP and CLI callers.
func (c *Core) SetOnChange(fn func(ids []string)) { c.onChange = fn }

// changed fires the onChange hook if set, naming the affected task id(s).
func (c *Core) changed(ids ...string) {
	if c.onChange != nil {
		c.onChange(ids)
	}
}

// Options configure a Core.
type Options struct {
	Prefix string // issue id prefix; derived from data if empty
	Actor  string // default actor for audit fields; defaults to "agent"
	// KeySelector, when set, is embedded into minted API tokens as
	// "tasks_<selector>_<secret>" so a multi-core host can route a bare token to
	// the right Core before verifying it. It is opaque to the engine and is NEVER
	// part of the hashed secret. Empty yields plain "tasks_<secret>" tokens.
	KeySelector string
}

// New builds a Core over st. If prefix/actor are empty they are derived/defaulted.
func New(st *store.Store, opts Options) (*Core, error) {
	c := &Core{st: st, prefix: opts.Prefix, actor: opts.Actor, keySelector: opts.KeySelector, nowFunc: time.Now}
	if c.actor == "" {
		c.actor = "agent"
	}
	if c.prefix == "" {
		p, err := st.Meta("prefix")
		if err != nil {
			return nil, err
		}
		c.prefix = p
	}
	if c.prefix == "" {
		// derive from existing ids
		tasks, err := st.List(store.Filter{Limit: 50})
		if err != nil {
			return nil, err
		}
		ids := make([]string, len(tasks))
		for i := range tasks {
			ids[i] = tasks[i].ID
		}
		c.prefix = DerivePrefix(ids)
	}
	if c.prefix != "" {
		_ = st.SetMeta("prefix", c.prefix)
	}
	return c, nil
}

// Prefix returns the configured issue prefix.
func (c *Core) Prefix() string { return c.prefix }

// Actor returns the default actor.
func (c *Core) Actor() string { return c.actor }

func (c *Core) now() string { return c.nowFunc().UTC().Format("2006-01-02T15:04:05Z") }

// resolveID expands a short id (without the project prefix) to the full id when
// the bare form doesn't exist but "<prefix>-<id>" does — so callers can use
// "w7t0" or "w7t0.1" instead of "tasks-w7t0". A literal match always wins.
func (c *Core) resolveID(id string) string {
	if id == "" || c.prefix == "" {
		return id
	}
	if ok, _ := c.st.Exists(id); ok {
		return id
	}
	if !strings.HasPrefix(id, c.prefix+"-") {
		full := c.prefix + "-" + id
		if ok, _ := c.st.Exists(full); ok {
			return full
		}
	}
	return id
}

// ResolveID exposes short-id expansion for callers that need it (e.g. display).
func (c *Core) ResolveID(id string) string { return c.resolveID(id) }

// Store exposes the underlying store for read-only helpers (used by http/mcp).
func (c *Core) Store() *store.Store { return c.st }

// Show returns a fully-hydrated task (accepts a short id).
func (c *Core) Show(id string) (*model.Task, error) { return c.st.Get(c.resolveID(id)) }

// ---- ready ----

// ReadyOptions filters ready work.
type ReadyOptions struct {
	Limit    int
	Assignee string
	Priority *int
	Type     string
	Parent   string
	Labels   []string
}

// Ready returns claimable work: status=open tasks with no still-open blocker,
// sorted by priority then natural id. Default limit 10.
//
// Semantics note: a "blocker" is a `blocks` dependency on a task that is not yet
// closed. Parent-child links are treated purely as containment, NOT as blockers.
// beads' own `bd ready` additionally (and inconsistently) hides a child when its
// parent is open in some cases but not others; we deliberately do not replicate
// that quirk, so genuinely-unblocked leaf work is never hidden. The raw
// dependency data is imported untouched — only this readiness view differs.
func (c *Core) Ready(o ReadyOptions) ([]model.Task, error) {
	f := store.Filter{
		Statuses:        []string{"open"},
		Assignee:        o.Assignee,
		Priority:        o.Priority,
		Parent:          c.resolveID(o.Parent),
		Labels:          o.Labels,
		OrderByPriority: true,
	}
	if o.Type != "" {
		f.Types = []string{o.Type}
	}
	candidates, err := c.st.List(f)
	if err != nil {
		return nil, err
	}
	// Build a status lookup for blocker resolution.
	statusByID, err := c.statusIndex()
	if err != nil {
		return nil, err
	}
	limit := o.Limit
	if limit <= 0 {
		limit = 10
	}
	var out []model.Task
	for _, t := range candidates {
		if c.hasOpenBlocker(t, statusByID) {
			continue
		}
		out = append(out, t)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (c *Core) hasOpenBlocker(t model.Task, statusByID map[string]string) bool {
	for _, d := range t.Dependencies {
		if d.Type != "blocks" {
			continue
		}
		st, ok := statusByID[d.DependsOnID]
		if !ok {
			continue // unknown blocker; treat as satisfied
		}
		if st != "closed" {
			return true
		}
	}
	return false
}

// statusIndex maps every task id to its status (for blocker checks).
func (c *Core) statusIndex() (map[string]string, error) {
	all, err := c.st.All()
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(all))
	for _, t := range all {
		m[t.ID] = t.Status
	}
	return m, nil
}

// ---- list ----

// List returns tasks for `tasks list` (a short --parent id is expanded).
func (c *Core) List(f store.Filter) ([]model.Task, error) {
	f.Parent = c.resolveID(f.Parent)
	return c.st.List(f)
}

// All returns every task (for UI/export).
func (c *Core) All() ([]model.Task, error) { return c.st.All() }

// ---- search ----

// SearchOptions filters and bounds a fuzzy search.
type SearchOptions struct {
	Statuses []string
	Type     string
	Limit    int
}

// Search fuzzy-matches tasks against query (fzf/fuse.js-style ranked subsequence
// matching over id + title + description + labels), returning best matches first.
// An empty query returns the (filtered) tasks unranked.
func (c *Core) Search(query string, o SearchOptions) ([]model.Task, error) {
	f := store.Filter{Statuses: o.Statuses}
	if o.Type != "" {
		f.Types = []string{o.Type}
	}
	candidates, err := c.st.List(f)
	if err != nil {
		return nil, err
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return limitTasks(candidates, o.Limit), nil
	}
	matches := fuzzy.FindFrom(q, taskSource(candidates))
	out := make([]model.Task, 0, len(matches))
	for _, m := range matches {
		out = append(out, candidates[m.Index])
	}
	return limitTasks(out, o.Limit), nil
}

// taskSource adapts tasks to a fuzzy.Source. The haystack leads with the title
// (and short id + labels) so title hits rank above description hits — matching
// fuse.js-style field weighting via match position.
type taskSource []model.Task

func (s taskSource) Len() int { return len(s) }
func (s taskSource) String(i int) string {
	t := s[i]
	hay := t.Title + " " + shortID(t.ID)
	if len(t.Labels) > 0 {
		hay += " " + strings.Join(t.Labels, " ")
	}
	if t.Description != "" {
		d := t.Description
		if len(d) > 400 {
			d = d[:400]
		}
		hay += " " + d
	}
	return hay
}

func shortID(id string) string {
	if i := strings.LastIndexByte(id, '-'); i >= 0 {
		return id[i+1:]
	}
	return id
}

func limitTasks(ts []model.Task, limit int) []model.Task {
	if limit > 0 && len(ts) > limit {
		return ts[:limit]
	}
	return ts
}

// ---- tree ----

// Tree returns a task and its transitive child subtree (parent-child). With an
// empty id it returns every task (the whole forest), which the CLI renders as
// all root subtrees. Returns ErrNotFound if a given root doesn't exist.
func (c *Core) Tree(id string) ([]model.Task, error) {
	if strings.TrimSpace(id) == "" {
		return c.st.All()
	}
	id = c.resolveID(id)
	ok, err := c.st.Exists(id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return c.st.Subtree(id)
}

// ---- errors ----

// ErrNotFound is re-exported for callers.
var ErrNotFound = store.ErrNotFound

func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return err
	}
	return fmt.Errorf("%s: %w", op, err)
}

// intPtr is a small helper.
func intPtr(v int) *int { return &v }

// splitCSV splits a comma list, trimming spaces and dropping empties.
func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
