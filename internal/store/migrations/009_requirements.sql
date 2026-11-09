-- +goose Up
-- Requirements layer (docs/08-devcycle.md §5): personas, jobs, story map
-- (activities and steps), stories and their acceptance criteria. Unlike
-- issues these are living specifications: they are archived, never closed,
-- and their keys (TAPP-ST12) are never reused.

CREATE TABLE req_personas (
  id           TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL REFERENCES projects(id),
  num          INTEGER NOT NULL,
  name         TEXT NOT NULL,
  kind         TEXT NOT NULL DEFAULT 'human' CHECK (kind IN ('human', 'agent')),
  description  TEXT NOT NULL DEFAULT '',
  position     INTEGER NOT NULL,
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL,
  archived_at  TEXT,
  UNIQUE (project_id, num)
);

CREATE TABLE req_jobs (
  id           TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL REFERENCES projects(id),
  num          INTEGER NOT NULL,
  persona_id   TEXT REFERENCES req_personas(id),
  situation    TEXT NOT NULL DEFAULT '',
  motivation   TEXT NOT NULL,
  outcome      TEXT NOT NULL DEFAULT '',
  position     INTEGER NOT NULL,
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL,
  archived_at  TEXT,
  UNIQUE (project_id, num)
);

CREATE TABLE req_activities (
  id           TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL REFERENCES projects(id),
  num          INTEGER NOT NULL,
  persona_id   TEXT REFERENCES req_personas(id),
  name         TEXT NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  position     INTEGER NOT NULL,
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL,
  archived_at  TEXT,
  UNIQUE (project_id, num)
);

CREATE TABLE req_steps (
  id           TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL REFERENCES projects(id),
  activity_id  TEXT NOT NULL REFERENCES req_activities(id),
  num          INTEGER NOT NULL,            -- within the activity: TAPP-A1-2
  name         TEXT NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  position     INTEGER NOT NULL,
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL,
  archived_at  TEXT,
  UNIQUE (activity_id, num)
);

CREATE TABLE req_job_activities (
  job_id       TEXT NOT NULL REFERENCES req_jobs(id),
  activity_id  TEXT NOT NULL REFERENCES req_activities(id),
  PRIMARY KEY (job_id, activity_id)
);

CREATE TABLE req_stories (
  id           TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL REFERENCES projects(id),
  num          INTEGER NOT NULL,
  title        TEXT NOT NULL,
  persona_id   TEXT REFERENCES req_personas(id),
  i_want       TEXT NOT NULL DEFAULT '',
  so_that      TEXT NOT NULL DEFAULT '',
  release      TEXT NOT NULL DEFAULT '',    -- e.g. R1; the map's horizontal slices
  position     INTEGER NOT NULL,
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL,
  archived_at  TEXT,
  UNIQUE (project_id, num)
);

CREATE TABLE req_story_steps (
  story_id     TEXT NOT NULL REFERENCES req_stories(id),
  step_id      TEXT NOT NULL REFERENCES req_steps(id),
  PRIMARY KEY (story_id, step_id)
);

CREATE TABLE req_criteria (
  id           TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL REFERENCES projects(id),
  story_id     TEXT NOT NULL REFERENCES req_stories(id),
  num          INTEGER NOT NULL,
  given_text   TEXT NOT NULL DEFAULT '',
  when_text    TEXT NOT NULL DEFAULT '',
  then_text    TEXT NOT NULL,
  position     INTEGER NOT NULL,
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL,
  archived_at  TEXT,
  UNIQUE (project_id, num)
);

CREATE TABLE req_story_issues (
  story_id     TEXT NOT NULL REFERENCES req_stories(id),
  issue_id     TEXT NOT NULL REFERENCES issues(id),
  created_at   TEXT NOT NULL,
  PRIMARY KEY (story_id, issue_id)
);
CREATE INDEX ix_req_story_issues_issue ON req_story_issues(issue_id);

-- Next number per project and kind (persona, job, activity, story, criterion).
CREATE TABLE req_counters (
  project_id   TEXT NOT NULL REFERENCES projects(id),
  kind         TEXT NOT NULL,
  next         INTEGER NOT NULL,
  PRIMARY KEY (project_id, kind)
);

-- Change history, standing in for versions in M1.
CREATE TABLE req_changes (
  id           TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL REFERENCES projects(id),
  entity_key   TEXT NOT NULL,               -- TAPP-ST12, TAPP-AC42, ...
  story_id     TEXT,                        -- set for a story and its criteria and links
  actor_id     TEXT NOT NULL REFERENCES users(id),
  action       TEXT NOT NULL,               -- created | updated | archived | linked | unlinked
  detail       TEXT NOT NULL DEFAULT '{}',  -- JSON: changed fields or the linked issue
  created_at   TEXT NOT NULL
);
CREATE INDEX ix_req_changes_story ON req_changes(story_id, created_at);

-- +goose Down
DROP INDEX ix_req_changes_story;
DROP TABLE req_changes;
DROP TABLE req_counters;
DROP INDEX ix_req_story_issues_issue;
DROP TABLE req_story_issues;
DROP TABLE req_criteria;
DROP TABLE req_story_steps;
DROP TABLE req_stories;
DROP TABLE req_job_activities;
DROP TABLE req_steps;
DROP TABLE req_activities;
DROP TABLE req_jobs;
DROP TABLE req_personas;
