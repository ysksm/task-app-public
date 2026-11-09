-- +goose Up
-- Option E (docs/09-requirements-loose.md §6): each project picks how its
-- requirements are written. structured = option A (place first, Given/When/Then,
-- gaps as warnings); quick = option D (one line into 未整理, one-line criteria).
-- Same tables either way; only the screens and the gap colouring differ.
-- Existing projects keep the M1 behaviour; CreateProject defaults new ones to quick.
ALTER TABLE projects ADD COLUMN requirements_mode TEXT NOT NULL DEFAULT 'structured' CHECK (requirements_mode IN ('structured', 'quick'));

-- +goose Down
ALTER TABLE projects DROP COLUMN requirements_mode;
