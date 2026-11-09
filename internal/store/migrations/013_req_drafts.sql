-- +goose Up
-- Drafts of a story map (docs/08-devcycle.md §5.9): a project whose draft_of
-- names the project it is a sandbox for. Brainstorm there freely, then import
-- the result into the target in one go after reviewing a plan.
-- Drafts are hidden from project lists; they are archived like any project.
ALTER TABLE projects ADD COLUMN draft_of TEXT REFERENCES projects(id);

-- Which element of the draft stands for which element of the target. Written
-- when a draft is copied from the target and after every import. The hashes are
-- the element's content (fields and references) on each side at that moment,
-- so a plan can tell "changed in the draft" from "changed in the target too".
-- kind: persona | activity | step | job | release | story | criterion
CREATE TABLE req_draft_links (
  draft_project_id  TEXT NOT NULL REFERENCES projects(id),
  kind              TEXT NOT NULL,
  draft_id          TEXT NOT NULL,
  target_id         TEXT NOT NULL,
  draft_hash        TEXT NOT NULL,
  target_hash       TEXT NOT NULL,
  PRIMARY KEY (draft_project_id, kind, draft_id),
  UNIQUE (draft_project_id, kind, target_id)
);

-- +goose Down
DROP TABLE req_draft_links;
ALTER TABLE projects DROP COLUMN draft_of;
