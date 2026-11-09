-- +goose Up
-- Option D (docs/09-requirements-loose.md): a story can be just a title plus free text.
ALTER TABLE req_stories ADD COLUMN body TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE req_stories DROP COLUMN body;
