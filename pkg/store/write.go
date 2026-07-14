package store

import (
	"database/sql"
	"fmt"

	"github.com/tristanMatthias/tasks/pkg/model"
)

// Upsert inserts or replaces a task and its dependencies/comments. Used by the
// importer; it fully replaces the task's dep/comment rows to stay idempotent.
func (s *Store) Upsert(t *model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := upsertTaskTx(tx, t); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM dependencies WHERE issue_id=?`, t.ID); err != nil {
		return err
	}
	for _, d := range t.Dependencies {
		if err := insertDepTx(tx, d); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM comments WHERE issue_id=?`, t.ID); err != nil {
		return err
	}
	for _, c := range t.Comments {
		if err := insertCommentTx(tx, c); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM gates WHERE issue_id=?`, t.ID); err != nil {
		return err
	}
	for _, g := range t.Gates {
		if err := insertGateTx(tx, g); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Insert creates a new task (no upsert). Errors if the id already exists.
func (s *Store) Insert(t *model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertTaskTx(tx, t); err != nil {
		return err
	}
	for _, d := range t.Dependencies {
		if err := insertDepTx(tx, d); err != nil {
			return err
		}
	}
	for _, c := range t.Comments {
		if err := insertCommentTx(tx, c); err != nil {
			return err
		}
	}
	for _, g := range t.Gates {
		if err := insertGateTx(tx, g); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func upsertTaskTx(tx *sql.Tx, t *model.Task) error {
	_, err := tx.Exec(upsertSQL, taskArgs(t)...)
	return err
}

func insertTaskTx(tx *sql.Tx, t *model.Task) error {
	_, err := tx.Exec(insertSQL, taskArgs(t)...)
	return err
}

func taskArgs(t *model.Task) []any {
	var key, xtype, value any
	if t.Key != nil {
		key = *t.Key
	}
	if t.Type != nil {
		xtype = *t.Type
	}
	if t.Value != nil {
		value = *t.Value
	}
	var prio any
	if t.Priority != nil {
		prio = *t.Priority
	}
	return []any{
		t.ID, key, xtype, t.Title, t.Description, t.Status, prio, t.IssueType,
		t.Owner, t.Assignee, t.CreatedBy, t.CreatedAt, t.UpdatedAt, t.StartedAt,
		t.ClosedAt, t.CloseReason, t.AcceptanceCriteria, t.Design, t.Notes,
		marshalLabels(t.Labels), value,
	}
}

const taskInsertCols = `id,key,xtype,title,description,status,priority,issue_type,owner,assignee,
	created_by,created_at,updated_at,started_at,closed_at,close_reason,
	acceptance_criteria,design,notes,labels,value`

var insertSQL = `INSERT INTO tasks (` + taskInsertCols + `) VALUES (` + placeholders(21) + `)`

var upsertSQL = `INSERT INTO tasks (` + taskInsertCols + `) VALUES (` + placeholders(21) + `)
ON CONFLICT(id) DO UPDATE SET
	key=excluded.key, xtype=excluded.xtype, title=excluded.title,
	description=excluded.description, status=excluded.status, priority=excluded.priority,
	issue_type=excluded.issue_type, owner=excluded.owner, assignee=excluded.assignee,
	created_by=excluded.created_by, created_at=excluded.created_at, updated_at=excluded.updated_at,
	started_at=excluded.started_at, closed_at=excluded.closed_at, close_reason=excluded.close_reason,
	acceptance_criteria=excluded.acceptance_criteria, design=excluded.design, notes=excluded.notes,
	labels=excluded.labels, value=excluded.value`

func insertDepTx(tx *sql.Tx, d model.Dependency) error {
	_, err := tx.Exec(`INSERT OR IGNORE INTO dependencies (issue_id,depends_on_id,type,created_at,created_by,metadata)
		VALUES (?,?,?,?,?,?)`, d.IssueID, d.DependsOnID, d.Type, d.CreatedAt, d.CreatedBy, d.Metadata)
	return err
}

func insertCommentTx(tx *sql.Tx, c model.Comment) error {
	_, err := tx.Exec(`INSERT OR REPLACE INTO comments (id,issue_id,author,text,created_at)
		VALUES (?,?,?,?,?)`, c.ID, c.IssueID, c.Author, c.Text, c.CreatedAt)
	return err
}

// Patch applies a set of column=value updates to a task and bumps updated_at.
// cols must be a whitelist of known column names.
func (s *Store) Patch(id string, set map[string]any, updatedAt string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(set) == 0 {
		return nil
	}
	q := "UPDATE tasks SET "
	var args []any
	first := true
	for col, v := range set {
		if !allowedCol[col] {
			return fmt.Errorf("column not updatable: %s", col)
		}
		if !first {
			q += ", "
		}
		q += col + "=?"
		args = append(args, v)
		first = false
	}
	q += ", updated_at=? WHERE id=?"
	args = append(args, updatedAt, id)
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete permanently removes a task along with its dependencies (in either
// direction) and comments. Returns ErrNotFound if the task doesn't exist.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM dependencies WHERE issue_id=? OR depends_on_id=?`, id, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM comments WHERE issue_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM gates WHERE issue_id=?`, id); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM tasks WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

var allowedCol = map[string]bool{
	"title": true, "description": true, "status": true, "priority": true,
	"issue_type": true, "owner": true, "assignee": true, "started_at": true,
	"closed_at": true, "close_reason": true, "acceptance_criteria": true,
	"design": true, "notes": true, "labels": true,
}

// Claim atomically claims a task for actor: sets assignee=actor and
// status=in_progress, but only if it is currently unassigned or already claimed
// by actor. Returns (claimed=false) when another actor already holds it.
// This is the multi-agent race guard.
func (s *Store) Claim(id, actor, startedAt, updatedAt string) (claimed bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`UPDATE tasks
		SET assignee=?, status='in_progress',
		    started_at=CASE WHEN started_at='' THEN ? ELSE started_at END,
		    updated_at=?
		WHERE id=? AND status!='closed' AND (assignee='' OR assignee=?)`,
		actor, startedAt, updatedAt, id, actor)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return true, nil
	}
	// Distinguish "not found" from "held by someone else".
	exists, err := s.existsTx(id)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, ErrNotFound
	}
	return false, nil
}

func (s *Store) existsTx(id string) (bool, error) {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM tasks WHERE id=?`, id).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// AddDependency inserts a dependency edge (idempotent).
func (s *Store) AddDependency(d model.Dependency) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertDepTx(tx, d); err != nil {
		return err
	}
	return tx.Commit()
}

// AddComment inserts a comment.
func (s *Store) AddComment(c model.Comment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertCommentTx(tx, c); err != nil {
		return err
	}
	return tx.Commit()
}
