-- +goose Up
-- Schema follows docs/04-er-diagram.md. Plain SQL only so that another
-- implementation (e.g. Rust) can apply the same migrations.

CREATE TABLE users (
  id            TEXT PRIMARY KEY,
  kind          TEXT NOT NULL CHECK (kind IN ('human', 'agent')),
  username      TEXT NOT NULL UNIQUE,
  display_name  TEXT NOT NULL,
  email         TEXT NOT NULL DEFAULT '',
  agent_model   TEXT NOT NULL DEFAULT '',
  avatar_color  TEXT NOT NULL DEFAULT '',
  is_owner      INTEGER NOT NULL DEFAULT 0,
  active        INTEGER NOT NULL DEFAULT 1,
  created_at    TEXT NOT NULL
);

CREATE TABLE api_tokens (
  id            TEXT PRIMARY KEY,
  user_id       TEXT NOT NULL REFERENCES users(id),
  name          TEXT NOT NULL,
  token_hash    TEXT NOT NULL UNIQUE,
  token_prefix  TEXT NOT NULL,
  last_used_at  TEXT,
  expires_at    TEXT,
  revoked_at    TEXT,
  created_at    TEXT NOT NULL
);

CREATE TABLE projects (
  id                      TEXT PRIMARY KEY,
  jira_id                 INTEGER NOT NULL UNIQUE,
  key                     TEXT NOT NULL UNIQUE,
  name                    TEXT NOT NULL,
  description             TEXT NOT NULL DEFAULT '',
  owner_id                TEXT NOT NULL REFERENCES users(id),
  next_issue_number       INTEGER NOT NULL DEFAULT 1,
  allow_skip_levels       INTEGER NOT NULL DEFAULT 1,
  default_qa_assignee_id  TEXT REFERENCES users(id),
  archived_at             TEXT,
  created_at              TEXT NOT NULL,
  updated_at              TEXT NOT NULL
);

CREATE TABLE workflow_states (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id     TEXT NOT NULL REFERENCES projects(id),
  name           TEXT NOT NULL,
  category       TEXT NOT NULL CHECK (category IN ('backlog', 'unstarted', 'started', 'completed', 'canceled')),
  applies_to     TEXT NOT NULL DEFAULT 'all' CHECK (applies_to IN ('all', 'qa')),
  color          TEXT NOT NULL DEFAULT '',
  position       INTEGER NOT NULL,
  is_acceptance  INTEGER NOT NULL DEFAULT 0,
  is_default     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX ix_states_project ON workflow_states(project_id, applies_to, position);

CREATE TABLE labels (
  id          TEXT PRIMARY KEY,
  project_id  TEXT NOT NULL REFERENCES projects(id),
  name        TEXT NOT NULL,
  color       TEXT NOT NULL DEFAULT '',
  UNIQUE (project_id, name)
);

CREATE TABLE issues (
  id            TEXT PRIMARY KEY,
  jira_id       INTEGER NOT NULL UNIQUE,
  project_id    TEXT NOT NULL REFERENCES projects(id),
  number        INTEGER NOT NULL,
  key           TEXT NOT NULL UNIQUE,
  type          TEXT NOT NULL CHECK (type IN ('epic', 'story', 'feature', 'task', 'qa')),
  parent_id     TEXT REFERENCES issues(id),
  title         TEXT NOT NULL,
  description   TEXT NOT NULL DEFAULT '',
  state_id      INTEGER NOT NULL REFERENCES workflow_states(id),
  priority      INTEGER NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 4),
  assignee_id   TEXT REFERENCES users(id),
  reporter_id   TEXT NOT NULL REFERENCES users(id),
  estimate      REAL,
  due_date      TEXT,
  sort_order    REAL NOT NULL DEFAULT 0,
  started_at    TEXT,
  completed_at  TEXT,
  canceled_at   TEXT,
  deleted_at    TEXT,
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL
);
CREATE UNIQUE INDEX ux_issues_project_number ON issues(project_id, number);
CREATE INDEX ix_issues_parent        ON issues(parent_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_issues_project_state ON issues(project_id, state_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_issues_assignee      ON issues(assignee_id, state_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_issues_updated       ON issues(updated_at);

CREATE TABLE issue_labels (
  issue_id  TEXT NOT NULL REFERENCES issues(id),
  label_id  TEXT NOT NULL REFERENCES labels(id),
  PRIMARY KEY (issue_id, label_id)
);

CREATE TABLE issue_links (
  id             TEXT PRIMARY KEY,
  from_issue_id  TEXT NOT NULL REFERENCES issues(id),
  to_issue_id    TEXT NOT NULL REFERENCES issues(id),
  link_type      TEXT NOT NULL CHECK (link_type IN ('blocks', 'relates', 'duplicates')),
  created_by     TEXT NOT NULL REFERENCES users(id),
  created_at     TEXT NOT NULL
);

CREATE TABLE comments (
  id          TEXT PRIMARY KEY,
  jira_id     INTEGER NOT NULL UNIQUE,
  issue_id    TEXT NOT NULL REFERENCES issues(id),
  author_id   TEXT NOT NULL REFERENCES users(id),
  body        TEXT NOT NULL,
  source      TEXT NOT NULL CHECK (source IN ('ui', 'api', 'jira', 'mcp', 'system')),
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);
CREATE INDEX ix_comments_issue ON comments(issue_id, created_at);

CREATE TABLE checkpoints (
  id            TEXT PRIMARY KEY,
  issue_id      TEXT NOT NULL REFERENCES issues(id),
  author_id     TEXT NOT NULL REFERENCES users(id),
  seq           INTEGER NOT NULL,
  summary       TEXT NOT NULL,
  decision      TEXT NOT NULL DEFAULT '',
  rationale     TEXT NOT NULL,
  alternatives  TEXT NOT NULL DEFAULT '[]',
  confidence    TEXT NOT NULL CHECK (confidence IN ('high', 'medium', 'low')),
  progress      INTEGER CHECK (progress BETWEEN 0 AND 100),
  next_steps    TEXT NOT NULL DEFAULT '',
  artifacts     TEXT NOT NULL DEFAULT '{}',
  source        TEXT NOT NULL CHECK (source IN ('ui', 'api', 'jira', 'mcp', 'system')),
  created_at    TEXT NOT NULL,
  UNIQUE (issue_id, seq)
);
CREATE INDEX ix_checkpoints_created ON checkpoints(created_at);

CREATE TABLE qa_details (
  issue_id              TEXT PRIMARY KEY REFERENCES issues(id),
  target_issue_id       TEXT REFERENCES issues(id),
  question              TEXT NOT NULL,
  context               TEXT NOT NULL DEFAULT '',
  answer_type           TEXT NOT NULL DEFAULT 'text' CHECK (answer_type IN ('yes_no', 'ok_ng', 'choice', 'text')),
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
CREATE INDEX ix_qa_open ON qa_details(answered_at, is_blocking);

CREATE TABLE acceptance_criteria (
  id           TEXT PRIMARY KEY,
  issue_id     TEXT NOT NULL REFERENCES issues(id),
  position     INTEGER NOT NULL,
  description  TEXT NOT NULL,
  created_at   TEXT NOT NULL
);

CREATE TABLE acceptance_reviews (
  id           TEXT PRIMARY KEY,
  issue_id     TEXT NOT NULL REFERENCES issues(id),
  round        INTEGER NOT NULL,
  reviewer_id  TEXT NOT NULL REFERENCES users(id),
  verdict      TEXT NOT NULL CHECK (verdict IN ('accepted', 'rejected')),
  summary      TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL
);

CREATE TABLE acceptance_review_items (
  review_id     TEXT NOT NULL REFERENCES acceptance_reviews(id),
  criterion_id  TEXT NOT NULL REFERENCES acceptance_criteria(id),
  result        TEXT NOT NULL CHECK (result IN ('ok', 'ng', 'na')),
  note          TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (review_id, criterion_id)
);

CREATE TABLE saved_views (
  id          TEXT PRIMARY KEY,
  project_id  TEXT REFERENCES projects(id),
  owner_id    TEXT NOT NULL REFERENCES users(id),
  name        TEXT NOT NULL,
  filter      TEXT NOT NULL DEFAULT '{}',
  display     TEXT NOT NULL DEFAULT '{}',
  position    INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);

CREATE TABLE activities (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  issue_id    TEXT NOT NULL REFERENCES issues(id),
  actor_id    TEXT NOT NULL REFERENCES users(id),
  source      TEXT NOT NULL CHECK (source IN ('ui', 'api', 'jira', 'mcp', 'system')),
  action      TEXT NOT NULL,
  field       TEXT NOT NULL DEFAULT '',
  old_value   TEXT NOT NULL DEFAULT '',
  new_value   TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL
);
CREATE INDEX ix_activities_issue ON activities(issue_id, created_at);

CREATE TABLE attachments (
  id            TEXT PRIMARY KEY,
  jira_id       INTEGER NOT NULL UNIQUE,
  issue_id      TEXT NOT NULL REFERENCES issues(id),
  uploader_id   TEXT NOT NULL REFERENCES users(id),
  filename      TEXT NOT NULL,
  mime_type     TEXT NOT NULL,
  size          INTEGER NOT NULL,
  storage_path  TEXT NOT NULL,
  created_at    TEXT NOT NULL
);

-- +goose Down
DROP TABLE attachments;
DROP TABLE activities;
DROP TABLE saved_views;
DROP TABLE acceptance_review_items;
DROP TABLE acceptance_reviews;
DROP TABLE acceptance_criteria;
DROP TABLE qa_details;
DROP TABLE checkpoints;
DROP TABLE comments;
DROP TABLE issue_links;
DROP TABLE issue_labels;
DROP TABLE issues;
DROP TABLE labels;
DROP TABLE workflow_states;
DROP TABLE projects;
DROP TABLE api_tokens;
DROP TABLE users;
