-- +goose Up
-- Web UI sessions, kept in the database so a restart (a deploy) does not log
-- everyone out. Only the SHA-256 of the cookie value is stored. Not part of
-- the JSON export (store/dump.go).
CREATE TABLE sessions (
  id_hash      TEXT PRIMARY KEY,
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  remember     INTEGER NOT NULL DEFAULT 0,
  created_at   TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  expires_at   TEXT NOT NULL
);
CREATE INDEX sessions_user ON sessions(user_id);

-- +goose Down
DROP TABLE sessions;
