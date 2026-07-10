package store

import (
	"database/sql"

	"github.com/tristanMatthias/tasks/pkg/model"
)

// InsertKey persists a new API key (its hash + metadata). The raw secret is
// never stored — only k.Hash.
func (s *Store) InsertKey(k model.APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO api_keys(id,hash,label,created_by,created_at,last_used_at,revoked_at)
		 VALUES(?,?,?,?,?,?,?)`,
		k.ID, k.Hash, k.Label, k.CreatedBy, k.CreatedAt, k.LastUsedAt, k.RevokedAt)
	return err
}

// ListKeys returns all keys (active and revoked), newest first. The hash is not
// selected — callers never need it and it must not leak.
func (s *Store) ListKeys() ([]model.APIKey, error) {
	rows, err := s.db.Query(
		`SELECT id,label,created_by,created_at,last_used_at,revoked_at
		 FROM api_keys ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.APIKey
	for rows.Next() {
		var k model.APIKey
		if err := rows.Scan(&k.ID, &k.Label, &k.CreatedBy, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// KeyByHash looks up a key by its secret hash. Returns ErrNotFound if absent.
func (s *Store) KeyByHash(hash string) (*model.APIKey, error) {
	return s.keyWhere("hash", hash)
}

// KeyByID looks up a key by its public id. Returns ErrNotFound if absent.
func (s *Store) KeyByID(id string) (*model.APIKey, error) {
	return s.keyWhere("id", id)
}

func (s *Store) keyWhere(col, val string) (*model.APIKey, error) {
	var k model.APIKey
	err := s.db.QueryRow(
		`SELECT id,hash,label,created_by,created_at,last_used_at,revoked_at
		 FROM api_keys WHERE `+col+`=?`, val).
		Scan(&k.ID, &k.Hash, &k.Label, &k.CreatedBy, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// TouchKey records the last-used timestamp for a key (best-effort audit).
func (s *Store) TouchKey(id, now string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE api_keys SET last_used_at=? WHERE id=?`, now, id)
	return err
}

// RevokeKey marks a key revoked. It returns false (no error) if the id doesn't
// exist or was already revoked, so revocation is idempotent and observable.
func (s *Store) RevokeKey(id, now string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`UPDATE api_keys SET revoked_at=? WHERE id=? AND revoked_at=''`, now, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
