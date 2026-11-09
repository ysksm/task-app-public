-- +goose Up
-- An encrypted copy of each API token so the settings screen can show it again
-- (token_hash stays the only thing authentication looks at). Empty = issued
-- before this column or without a key: cannot be shown, only reissued.
-- Format: see service/tokenkey.go.
ALTER TABLE api_tokens ADD COLUMN token_enc TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE api_tokens DROP COLUMN token_enc;
