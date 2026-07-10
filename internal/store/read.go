package store

import (
	"database/sql"
	"strings"

	"github.com/tristanMatthias/tasks/internal/model"
)

const taskCols = `id,key,xtype,title,description,status,priority,issue_type,owner,assignee,
	created_by,created_at,updated_at,started_at,closed_at,close_reason,
	acceptance_criteria,design,notes,labels,value`

func scanTask(rows scanner) (model.Task, error) {
	var t model.Task
	var key, xtype, value sql.NullString
	var prio sql.NullInt64
	var labels string
	err := rows.Scan(
		&t.ID, &key, &xtype, &t.Title, &t.Description, &t.Status, &prio, &t.IssueType,
		&t.Owner, &t.Assignee, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.StartedAt,
		&t.ClosedAt, &t.CloseReason, &t.AcceptanceCriteria, &t.Design, &t.Notes,
		&labels, &value,
	)
	if err != nil {
		return t, err
	}
	if key.Valid {
		t.Key = &key.String
	}
	if xtype.Valid {
		t.Type = &xtype.String
	}
	if value.Valid {
		t.Value = &value.String
	}
	if prio.Valid {
		p := int(prio.Int64)
		t.Priority = &p
	}
	t.Labels = unmarshalLabels(labels)
	return t, nil
}

type scanner interface {
	Scan(dest ...any) error
}

// Get returns a single fully-populated task (deps, comments, counts).
func (s *Store) Get(id string) (*model.Task, error) {
	row := s.db.QueryRow(`SELECT `+taskCols+` FROM tasks WHERE id=?`, id)
	t, err := scanTask(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := s.hydrate([]*model.Task{&t}); err != nil {
		return nil, err
	}
	return &t, nil
}

// All returns every task, id-ordered naturally, fully hydrated. Used by the UI
// endpoint and the JSONL exporter.
func (s *Store) All() ([]model.Task, error) {
	return s.query(`SELECT `+taskCols+` FROM tasks`, nil)
}

// Subtree returns rootID and all of its transitive children (via parent-child
// dependencies), hydrated and natural-id sorted. Empty if rootID doesn't exist.
func (s *Store) Subtree(rootID string) ([]model.Task, error) {
	const q = `WITH RECURSIVE sub(id) AS (
		SELECT id FROM tasks WHERE id = ?
		UNION
		SELECT d.issue_id FROM dependencies d
		  JOIN sub ON d.depends_on_id = sub.id
		 WHERE d.type IN ('parent-child','parent')
	)
	SELECT ` + taskCols + ` FROM tasks WHERE id IN (SELECT id FROM sub)`
	return s.query(q, []any{rootID})
}

// Filter describes the supported list filters (a subset of `bd list`).
type Filter struct {
	Statuses        []string // OR; empty = any
	Types           []string // OR; empty = any
	Assignee        string   // exact; "" = ignore
	Priority        *int     // exact; nil = ignore
	Parent          string   // only children of this id (parent-child dep)
	Labels          []string // AND: must have all
	Limit           int      // 0 = no limit
	OrderByPriority bool     // priority then natural id (default: natural id)
}

// List returns tasks matching a filter, hydrated.
func (s *Store) List(f Filter) ([]model.Task, error) {
	var where []string
	var args []any
	if len(f.Statuses) > 0 {
		where = append(where, "status IN ("+placeholders(len(f.Statuses))+")")
		for _, v := range f.Statuses {
			args = append(args, v)
		}
	}
	if len(f.Types) > 0 {
		where = append(where, "issue_type IN ("+placeholders(len(f.Types))+")")
		for _, v := range f.Types {
			args = append(args, v)
		}
	}
	if f.Assignee != "" {
		where = append(where, "assignee=?")
		args = append(args, f.Assignee)
	}
	if f.Priority != nil {
		where = append(where, "priority=?")
		args = append(args, *f.Priority)
	}
	if f.Parent != "" {
		where = append(where, `id IN (SELECT issue_id FROM dependencies WHERE depends_on_id=? AND type IN ('parent-child','parent'))`)
		args = append(args, f.Parent)
	}
	q := `SELECT ` + taskCols + ` FROM tasks`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	tasks, err := s.query(q, args)
	if err != nil {
		return nil, err
	}
	// Label AND-filter applied in Go (labels stored as JSON text).
	if len(f.Labels) > 0 {
		tasks = filterLabels(tasks, f.Labels)
	}
	if f.OrderByPriority {
		sortByPriority(tasks)
	}
	if f.Limit > 0 && len(tasks) > f.Limit {
		tasks = tasks[:f.Limit]
	}
	return tasks, nil
}

// query runs a task SELECT, sorts naturally by id, and hydrates deps/comments.
func (s *Store) query(q string, args []any) ([]model.Task, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []model.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortNatural(tasks)
	ptrs := make([]*model.Task, len(tasks))
	for i := range tasks {
		ptrs[i] = &tasks[i]
	}
	if err := s.hydrate(ptrs); err != nil {
		return nil, err
	}
	return tasks, nil
}

// hydrate fills dependencies, comments and the computed counts for the given
// tasks using batched lookups (no N+1). dependent_count is computed against the
// entire dependency table (a task may be depended on by any other task).
func (s *Store) hydrate(tasks []*model.Task) error {
	if len(tasks) == 0 {
		return nil
	}
	byID := make(map[string]*model.Task, len(tasks))
	ids := make([]string, 0, len(tasks))
	for _, t := range tasks {
		t.Dependencies = nil
		t.Comments = nil
		byID[t.ID] = t
		ids = append(ids, t.ID)
	}

	// Dependencies where issue_id in set.
	if err := s.eachIn(`SELECT issue_id,depends_on_id,type,created_at,created_by,metadata FROM dependencies WHERE issue_id IN (`, ids, func(rows *sql.Rows) error {
		for rows.Next() {
			var d model.Dependency
			if err := rows.Scan(&d.IssueID, &d.DependsOnID, &d.Type, &d.CreatedAt, &d.CreatedBy, &d.Metadata); err != nil {
				return err
			}
			if t := byID[d.IssueID]; t != nil {
				t.Dependencies = append(t.Dependencies, d)
			}
		}
		return rows.Err()
	}); err != nil {
		return err
	}

	// Comments where issue_id in set (ordered).
	if err := s.eachIn(`SELECT id,issue_id,author,text,created_at FROM comments WHERE issue_id IN (`, ids, func(rows *sql.Rows) error {
		for rows.Next() {
			var c model.Comment
			if err := rows.Scan(&c.ID, &c.IssueID, &c.Author, &c.Text, &c.CreatedAt); err != nil {
				return err
			}
			if t := byID[c.IssueID]; t != nil {
				t.Comments = append(t.Comments, c)
			}
		}
		return rows.Err()
	}); err != nil {
		return err
	}

	// dependent_count: how many deps point AT each id.
	depCounts := map[string]int{}
	if err := s.eachIn(`SELECT depends_on_id,COUNT(*) FROM dependencies WHERE depends_on_id IN (`, ids, func(rows *sql.Rows) error {
		for rows.Next() {
			var id string
			var n int
			if err := rows.Scan(&id, &n); err != nil {
				return err
			}
			depCounts[id] = n
		}
		return rows.Err()
	}, ` GROUP BY depends_on_id`); err != nil {
		return err
	}

	for _, t := range tasks {
		t.DependencyCount = len(t.Dependencies)
		t.CommentCount = len(t.Comments)
		t.DependentCount = depCounts[t.ID]
	}
	return nil
}

// eachIn runs `prefix (?,?,...) suffix...` over ids and invokes fn with the rows.
func (s *Store) eachIn(prefix string, ids []string, fn func(*sql.Rows) error, suffix ...string) error {
	if len(ids) == 0 {
		return nil
	}
	q := prefix + placeholders(len(ids)) + ")"
	for _, sfx := range suffix {
		q += sfx
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	return fn(rows)
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}
