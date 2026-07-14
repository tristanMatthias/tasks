package store

import (
	"database/sql"
	"errors"

	"github.com/tristanMatthias/tasks/pkg/model"
)

// ErrGateToken is returned when a verify token is missing, wrong, already used,
// or expired — the one-time-token guard for command-gate verification.
var ErrGateToken = errors.New("invalid or expired verify token")

const gateCols = `issue_id,id,type,description,command,status,verified_at,verified_by,exit_code,evidence,created_at`

func scanGate(rows scanner) (model.Gate, error) {
	var g model.Gate
	var exit sql.NullInt64
	if err := rows.Scan(&g.IssueID, &g.ID, &g.Type, &g.Description, &g.Command,
		&g.Status, &g.VerifiedAt, &g.VerifiedBy, &exit, &g.Evidence, &g.CreatedAt); err != nil {
		return g, err
	}
	if exit.Valid {
		e := int(exit.Int64)
		g.ExitCode = &e
	}
	return g, nil
}

// insertGateTx writes a gate's descriptive + verification columns (leaving the
// one-time token columns at their default). Used by task Insert/Upsert.
func insertGateTx(tx *sql.Tx, g model.Gate) error {
	var exit any
	if g.ExitCode != nil {
		exit = *g.ExitCode
	}
	_, err := tx.Exec(`INSERT OR REPLACE INTO gates
		(issue_id,id,type,description,command,status,verified_at,verified_by,exit_code,evidence,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		g.IssueID, g.ID, g.Type, g.Description, g.Command, g.Status,
		g.VerifiedAt, g.VerifiedBy, exit, g.Evidence, g.CreatedAt)
	return err
}

// AddGate inserts a single gate (its own transaction).
func (s *Store) AddGate(g model.Gate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertGateTx(tx, g); err != nil {
		return err
	}
	return tx.Commit()
}

// RemoveGate deletes a gate by (issue_id, id). ErrNotFound if absent.
func (s *Store) RemoveGate(issueID, gateID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`DELETE FROM gates WHERE issue_id=? AND id=?`, issueID, gateID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetGate returns one gate. ErrNotFound if absent.
func (s *Store) GetGate(issueID, gateID string) (model.Gate, error) {
	row := s.db.QueryRow(`SELECT `+gateCols+` FROM gates WHERE issue_id=? AND id=?`, issueID, gateID)
	g, err := scanGate(row)
	if err == sql.ErrNoRows {
		return g, ErrNotFound
	}
	return g, err
}

// ListGates returns a task's gates in creation order.
func (s *Store) ListGates(issueID string) ([]model.Gate, error) {
	rows, err := s.db.Query(`SELECT `+gateCols+` FROM gates WHERE issue_id=? ORDER BY id`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Gate
	for rows.Next() {
		g, err := scanGate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// SetGateToken arms a gate with a fresh one-time verify token + expiry (RFC3339).
// It replaces any prior token, so each BeginVerify supersedes the last.
func (s *Store) SetGateToken(issueID, gateID, token, expires string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`UPDATE gates SET token=?, token_expires=? WHERE issue_id=? AND id=?`,
		token, expires, issueID, gateID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ConsumeGateVerify atomically marks a gate verified IFF the presented token
// matches, is non-empty, and hasn't expired as of now (all checked in one
// conditional UPDATE, so the token is strictly single-use). Clears the token.
// Returns ErrGateToken when the token doesn't check out.
func (s *Store) ConsumeGateVerify(issueID, gateID, token, now, verifiedAt, verifiedBy string, exitCode int, evidence string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`UPDATE gates
		SET status='verified', verified_at=?, verified_by=?, exit_code=?, evidence=?, token='', token_expires=''
		WHERE issue_id=? AND id=? AND token=? AND token!='' AND token_expires>=?`,
		verifiedAt, verifiedBy, exitCode, evidence, issueID, gateID, token, now)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrGateToken
	}
	return nil
}
