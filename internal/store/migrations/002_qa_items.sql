-- +goose Up
-- A QA ticket holds one or more questions (qa_items); each has its own answer
-- format, including multi-select. qa_details keeps ticket-level data only.
-- No qa_details rows existed before this migration, so it is recreated.
DROP INDEX IF EXISTS ix_qa_open;
DROP TABLE qa_details;

CREATE TABLE qa_details (
  issue_id              TEXT PRIMARY KEY REFERENCES issues(id),
  target_issue_id       TEXT REFERENCES issues(id),
  context               TEXT NOT NULL DEFAULT '',
  provisional_decision  TEXT NOT NULL DEFAULT '',
  is_blocking           INTEGER NOT NULL DEFAULT 0,
  answered_at           TEXT,
  closed_at             TEXT
);

CREATE TABLE qa_items (
  id                   TEXT PRIMARY KEY,
  issue_id             TEXT NOT NULL REFERENCES issues(id),
  position             INTEGER NOT NULL,
  question             TEXT NOT NULL,
  answer_type          TEXT NOT NULL CHECK (answer_type IN ('yes_no', 'ok_ng', 'single_choice', 'multi_choice', 'text')),
  options              TEXT NOT NULL DEFAULT '[]',
  allow_other          INTEGER NOT NULL DEFAULT 0,
  required             INTEGER NOT NULL DEFAULT 1,
  selected_option_ids  TEXT,
  answer_text          TEXT,
  answer_note          TEXT,
  answered_by          TEXT REFERENCES users(id),
  answered_at          TEXT,
  UNIQUE (issue_id, position)
);
CREATE INDEX ix_qa_items_issue ON qa_items(issue_id, position);

-- +goose Down
DROP TABLE qa_items;
DROP TABLE qa_details;
CREATE TABLE qa_details (
  issue_id              TEXT PRIMARY KEY REFERENCES issues(id),
  target_issue_id       TEXT REFERENCES issues(id),
  question              TEXT NOT NULL,
  context               TEXT NOT NULL DEFAULT '',
  answer_type           TEXT NOT NULL DEFAULT 'text',
  options               TEXT NOT NULL DEFAULT '[]',
  allow_other           INTEGER NOT NULL DEFAULT 0,
  provisional_decision  TEXT NOT NULL DEFAULT '',
  is_blocking           INTEGER NOT NULL DEFAULT 0,
  answer                TEXT,
  answer_note           TEXT,
  selected_option_id    TEXT,
  answered_by           TEXT REFERENCES users(id),
  answered_at           TEXT
);
