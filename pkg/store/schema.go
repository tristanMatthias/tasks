package store

const schemaSQL = `
CREATE TABLE IF NOT EXISTS tasks (
	id                  TEXT PRIMARY KEY,
	key                 TEXT,
	xtype               TEXT,               -- beads "_type" (e.g. "memory")
	title               TEXT NOT NULL DEFAULT '',
	description         TEXT NOT NULL DEFAULT '',
	status              TEXT NOT NULL DEFAULT 'open',
	priority            INTEGER,            -- nullable to match legacy records
	issue_type          TEXT NOT NULL DEFAULT 'task',
	owner               TEXT NOT NULL DEFAULT '',
	assignee            TEXT NOT NULL DEFAULT '',
	created_by          TEXT NOT NULL DEFAULT '',
	created_at          TEXT NOT NULL DEFAULT '',
	updated_at          TEXT NOT NULL DEFAULT '',
	started_at          TEXT NOT NULL DEFAULT '',
	closed_at           TEXT NOT NULL DEFAULT '',
	close_reason        TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	design              TEXT NOT NULL DEFAULT '',
	notes               TEXT NOT NULL DEFAULT '',
	labels              TEXT NOT NULL DEFAULT '',  -- JSON array, or '' when none
	value               TEXT
);

CREATE TABLE IF NOT EXISTS dependencies (
	issue_id      TEXT NOT NULL,
	depends_on_id TEXT NOT NULL,
	type          TEXT NOT NULL,
	created_at    TEXT NOT NULL DEFAULT '',
	created_by    TEXT NOT NULL DEFAULT '',
	metadata      TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (issue_id, depends_on_id, type)
);

CREATE TABLE IF NOT EXISTS comments (
	id         TEXT PRIMARY KEY,
	issue_id   TEXT NOT NULL,
	author     TEXT NOT NULL DEFAULT '',
	text       TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS gates (
	issue_id      TEXT NOT NULL,
	id            TEXT NOT NULL,
	type          TEXT NOT NULL DEFAULT 'command',
	description   TEXT NOT NULL DEFAULT '',
	command       TEXT NOT NULL DEFAULT '',
	status        TEXT NOT NULL DEFAULT 'pending',
	verified_at   TEXT NOT NULL DEFAULT '',
	verified_by   TEXT NOT NULL DEFAULT '',
	exit_code     INTEGER,
	evidence      TEXT NOT NULL DEFAULT '',
	created_at    TEXT NOT NULL DEFAULT '',
	token         TEXT NOT NULL DEFAULT '',  -- one-time verify token (cleared on use)
	token_expires TEXT NOT NULL DEFAULT '',  -- RFC3339 expiry of the pending token
	PRIMARY KEY (issue_id, id)
);

CREATE TABLE IF NOT EXISTS meta (
	k TEXT PRIMARY KEY,
	v TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS api_keys (
	id           TEXT PRIMARY KEY,
	hash         TEXT NOT NULL UNIQUE,        -- sha256 hex of the raw secret
	label        TEXT NOT NULL DEFAULT '',
	created_by   TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL DEFAULT '',
	last_used_at TEXT NOT NULL DEFAULT '',
	revoked_at   TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_deps_issue      ON dependencies(issue_id);
CREATE INDEX IF NOT EXISTS idx_deps_dependson  ON dependencies(depends_on_id);
CREATE INDEX IF NOT EXISTS idx_comments_issue  ON comments(issue_id);
CREATE INDEX IF NOT EXISTS idx_gates_issue     ON gates(issue_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status    ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_type      ON tasks(issue_type);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash   ON api_keys(hash);
`
