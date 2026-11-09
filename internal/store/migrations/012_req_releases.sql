-- +goose Up
-- Release slices (docs/08-devcycle.md §5.8): the story map's horizontal rows as
-- things of their own, with an outcome (goal), an order and a status. Stories
-- still name their release by text (req_stories.release = req_releases.name);
-- renaming a slice renames it on its stories. Keys are TAPP-RL1, never reused.
CREATE TABLE req_releases (
  id           TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL REFERENCES projects(id),
  num          INTEGER NOT NULL,
  name         TEXT NOT NULL,
  goal         TEXT NOT NULL DEFAULT '',       -- the outcome: what this slice lets someone do
  status       TEXT NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'building', 'shipped')),
  metric       TEXT NOT NULL DEFAULT '',       -- optional: how we will know it worked
  position     INTEGER NOT NULL,
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL,
  UNIQUE (project_id, num),
  UNIQUE (project_id, name)
);

-- One slice per release name already written on stories, in name order.
INSERT INTO req_releases (id, project_id, num, name, position, created_at, updated_at)
SELECT 'rl' || lower(hex(randomblob(12))), project_id,
       ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY release),
       release,
       ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY release),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM (SELECT DISTINCT project_id, release FROM req_stories WHERE release <> '');

INSERT INTO req_counters (project_id, kind, next)
SELECT project_id, 'release', MAX(num) FROM req_releases GROUP BY project_id;

-- +goose Down
DELETE FROM req_counters WHERE kind = 'release';
DROP TABLE req_releases;
