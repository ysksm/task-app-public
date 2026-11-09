-- +goose Up
-- How much attention a project gets: focus (注力) / normal (通常) / side (その他:
-- low priority, things started casually). The all-projects board and the
-- projects page group by it.
ALTER TABLE projects ADD COLUMN tier TEXT NOT NULL DEFAULT 'normal' CHECK (tier IN ('focus', 'normal', 'side'));

-- +goose Down
ALTER TABLE projects DROP COLUMN tier;
