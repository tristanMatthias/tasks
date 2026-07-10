// Package store persists tasks in SQLite — the single source of truth. It uses
// pure-Go modernc.org/sqlite (no cgo) in WAL mode so many agents can hit it
// concurrently, and provides transaction-atomic mutations (notably Claim).
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a task id does not exist.
var ErrNotFound = errors.New("task not found")

// Store wraps a SQLite database.
type Store struct {
	db *sql.DB
	mu sync.Mutex // serializes write transactions (SQLite allows one writer)
}

// Open opens (creating if needed) the SQLite database at path and applies the
// schema. WAL + busy_timeout make concurrent readers/writers robust.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(0)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // single connection keeps WAL writes serialized & simple
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// Ping verifies the database connection is alive (used by readiness checks).
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Meta returns the stored value for key k, or "" if unset.
func (s *Store) Meta(k string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT v FROM meta WHERE k=?`, k).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// SetMeta stores a key/value pair.
func (s *Store) SetMeta(k, v string) error {
	_, err := s.db.Exec(`INSERT INTO meta(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, k, v)
	return err
}

// Count returns the number of tasks.
func (s *Store) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&n)
	return n, err
}

// Exists reports whether a task id exists.
func (s *Store) Exists(id string) (bool, error) {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM tasks WHERE id=?`, id).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// ---- serialization helpers ----

func marshalLabels(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	b, _ := json.Marshal(labels)
	return string(b)
}

func unmarshalLabels(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	_ = json.Unmarshal([]byte(s), &out)
	return out
}
