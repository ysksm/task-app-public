-- +goose Up
-- Humans can log in to the web UI from other devices with a password
-- (01 §4.12). Empty = no password. Format: see service/password.go.
ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN password_hash;
