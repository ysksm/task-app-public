-- +goose Up
-- Jira clients (ORCA in Cloud mode) sign in with email + API token, so every
-- user needs an email. Users created without one get <username>@taskapp.local.
UPDATE users SET email = username || '@taskapp.local' WHERE email = '';

-- +goose Down
UPDATE users SET email = '' WHERE email = username || '@taskapp.local';
