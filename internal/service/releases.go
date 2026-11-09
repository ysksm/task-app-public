package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Release slices (docs/08-devcycle.md §5.8): the rows of the story map. A story
// names its slice by text (Story.Release = Release.Name); a name written on a
// story that has no slice yet creates one (planned), so writing "R2" on a story
// still works in quick mode and from MCP.

var kindRelease = reqKind{"req_releases", "RL", "release"}

// Release statuses.
const (
	ReleasePlanned  = "planned"  // 計画中
	ReleaseBuilding = "building" // 作っている
	ReleaseShipped  = "shipped"  // 出した
)

// Release is a horizontal slice of the story map.
type Release struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Goal      string `json:"goal"`
	Status    string `json:"status"`
	Metric    string `json:"metric"`
	Position  int    `json:"position"`
	Stories   int    `json:"stories"` // active stories in the slice
	Done      int    `json:"done"`    // of which done (all linked issues completed)
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	id string
}

// ReleaseInput creates a slice.
type ReleaseInput struct {
	Name   string `json:"name"`
	Goal   string `json:"goal"`
	Status string `json:"status"`
	Metric string `json:"metric"`
}

// ReleasePatch changes a slice. Renaming renames it on its stories.
type ReleasePatch struct {
	Name     *string `json:"name"`
	Goal     *string `json:"goal"`
	Status   *string `json:"status"`
	Metric   *string `json:"metric"`
	Position *int    `json:"position"`
}

func validReleaseStatus(v string) (string, error) {
	if v == "" {
		return ReleasePlanned, nil
	}
	if v != ReleasePlanned && v != ReleaseBuilding && v != ReleaseShipped {
		return "", Errorf(CodeInvalidArgument, "status must be planned, building or shipped")
	}
	return v, nil
}

// loadReleases returns the slices in map order (position, then number).
func (s *Service) loadReleases(ctx context.Context, q queryer, p reqProject) ([]Release, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, num, name, goal, status, metric, position, created_at, updated_at
		FROM req_releases WHERE project_id = ? ORDER BY position, num`, p.id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Release{}
	for rows.Next() {
		var r Release
		var n int
		if err := rows.Scan(&r.id, &n, &r.Name, &r.Goal, &r.Status, &r.Metric, &r.Position, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Key = fmtKey(p.key, "RL", n)
		out = append(out, r)
	}
	return out, rows.Err()
}

// withCounts fills Stories and Done from the loaded stories.
func withCounts(rels []Release, stories []Story) []Release {
	idx := map[string]int{}
	for i, r := range rels {
		idx[r.Name] = i
	}
	for _, st := range stories {
		if i, ok := idx[st.Release]; ok {
			rels[i].Stories++
			if st.Status == "done" {
				rels[i].Done++
			}
		}
	}
	return rels
}

// ensureRelease creates a planned slice for a name written on a story.
func (s *Service) ensureRelease(ctx context.Context, tx *sql.Tx, p reqProject, actor Actor, name string) error {
	if name == "" {
		return nil
	}
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM req_releases WHERE project_id = ? AND name = ?`, p.id, name).Scan(&id)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = s.insertRelease(ctx, tx, p, actor, ReleaseInput{Name: name, Status: ReleasePlanned})
	return err
}

func (s *Service) insertRelease(ctx context.Context, tx *sql.Tx, p reqProject, actor Actor, in ReleaseInput) (string, error) {
	n, err := nextReqNum(ctx, tx, p.id, "release")
	if err != nil {
		return "", err
	}
	var pos int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) + 1 FROM req_releases WHERE project_id = ?`, p.id).Scan(&pos); err != nil {
		return "", err
	}
	now := s.timestamp()
	if _, err := tx.ExecContext(ctx, `INSERT INTO req_releases (id, project_id, num, name, goal, status, metric, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		newID(), p.id, n, in.Name, strings.TrimSpace(in.Goal), in.Status, strings.TrimSpace(in.Metric), pos, now, now); err != nil {
		if isUniqueViolation(err) {
			return "", Errorf(CodeConflict, "release %q already exists", in.Name)
		}
		return "", err
	}
	key := fmtKey(p.key, "RL", n)
	return key, s.reqChange(ctx, tx, p, actor, key, nil, "created", map[string]any{"name": in.Name})
}

// ListReleases returns a project's slices in map order with their progress.
func (s *Service) ListReleases(ctx context.Context, projectKey string) ([]Release, error) {
	p, _, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	m, err := s.loadStoryMap(ctx, p)
	if err != nil {
		return nil, err
	}
	return m.ReleaseSlices, nil
}

func (s *Service) getRelease(ctx context.Context, p reqProject, key string) (Release, error) {
	m, err := s.loadStoryMap(ctx, p)
	if err != nil {
		return Release{}, err
	}
	for _, r := range m.ReleaseSlices {
		if r.Key == key {
			return r, nil
		}
	}
	return Release{}, Errorf(CodeNotFound, "release %s not found", key)
}

func (p reqProject) releaseID(ctx context.Context, q queryer, raw string) (string, string, error) {
	n, ok := ParseReqNum(p.key, "RL", raw)
	if !ok {
		return "", "", Errorf(CodeNotFound, "release %q not found", raw)
	}
	var id string
	err := q.QueryRowContext(ctx, `SELECT id FROM req_releases WHERE project_id = ? AND num = ?`, p.id, n).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", Errorf(CodeNotFound, "release %q not found", raw)
	}
	return id, fmtKey(p.key, "RL", n), err
}

// CreateRelease adds a slice at the bottom of the map (above 未定).
func (s *Service) CreateRelease(ctx context.Context, projectKey string, in ReleaseInput) (Release, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Release{}, err
	}
	if in.Name, err = required("name", &in.Name); err != nil {
		return Release{}, err
	}
	if in.Status, err = validReleaseStatus(in.Status); err != nil {
		return Release{}, err
	}
	var key string
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		key, err = s.insertRelease(ctx, tx, p, actor, in)
		return err
	})
	if err != nil {
		return Release{}, err
	}
	s.reqChanged(p, actor)
	return s.getRelease(ctx, p, key)
}

// UpdateRelease changes a slice; a new name is written onto its stories too.
func (s *Service) UpdateRelease(ctx context.Context, projectKey, key string, in ReleasePatch) (Release, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Release{}, err
	}
	id, full, err := p.releaseID(ctx, s.db, key)
	if err != nil {
		return Release{}, err
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var ps patchSet
		if in.Name != nil {
			v, err := required("name", in.Name)
			if err != nil {
				return err
			}
			var old string
			if err := tx.QueryRowContext(ctx, `SELECT name FROM req_releases WHERE id = ?`, id).Scan(&old); err != nil {
				return err
			}
			if v != old {
				var clash int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM req_releases WHERE project_id = ? AND name = ?`, p.id, v).Scan(&clash); err != nil {
					return err
				}
				if clash > 0 {
					return Errorf(CodeConflict, "release %q already exists", v)
				}
				if _, err := tx.ExecContext(ctx, `UPDATE req_stories SET release = ?, updated_at = ? WHERE project_id = ? AND release = ?`, v, s.timestamp(), p.id, old); err != nil {
					return err
				}
				ps.add("name", "name", v)
			}
		}
		if in.Goal != nil {
			ps.add("goal", "goal", strings.TrimSpace(*in.Goal))
		}
		if in.Status != nil {
			if *in.Status == "" {
				return Errorf(CodeInvalidArgument, "status must be planned, building or shipped")
			}
			v, err := validReleaseStatus(*in.Status)
			if err != nil {
				return err
			}
			ps.add("status", "status", v)
		}
		if in.Metric != nil {
			ps.add("metric", "metric", strings.TrimSpace(*in.Metric))
		}
		if in.Position != nil {
			ps.add("position", "position", *in.Position)
		}
		if err := ps.exec(ctx, tx, "req_releases", id, s.timestamp()); err != nil {
			return err
		}
		if ps.detail == nil {
			return nil
		}
		return s.reqChange(ctx, tx, p, actor, full, nil, "updated", ps.detail)
	})
	if err != nil {
		return Release{}, err
	}
	s.reqChanged(p, actor)
	return s.getRelease(ctx, p, full)
}

// DeleteRelease removes an empty slice. Stories must be moved out first (409).
func (s *Service) DeleteRelease(ctx context.Context, projectKey, key string) error {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return err
	}
	id, full, err := p.releaseID(ctx, s.db, key)
	if err != nil {
		return err
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM req_stories s JOIN req_releases r ON r.project_id = s.project_id AND r.name = s.release
			WHERE r.id = ? AND s.archived_at IS NULL`, id).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return Errorf(CodeConflict, "release %s still has %d stories; move them first", full, n)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM req_releases WHERE id = ?`, id); err != nil {
			return err
		}
		return s.reqChange(ctx, tx, p, actor, full, nil, "deleted", nil)
	})
	if err != nil {
		return err
	}
	s.reqChanged(p, actor)
	return nil
}

// UpsertRelease is the MCP form: find the slice by name (create it if missing) and apply the patch.
func (s *Service) UpsertRelease(ctx context.Context, projectKey, name string, in ReleasePatch) (Release, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Release{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Release{}, Errorf(CodeInvalidArgument, "name is required")
	}
	var num int
	err = s.db.QueryRowContext(ctx, `SELECT num FROM req_releases WHERE project_id = ? AND name = ?`, p.id, name).Scan(&num)
	if errors.Is(err, sql.ErrNoRows) {
		if err := s.withTx(ctx, func(tx *sql.Tx) error { return s.ensureRelease(ctx, tx, p, actor, name) }); err != nil {
			return Release{}, err
		}
		err = s.db.QueryRowContext(ctx, `SELECT num FROM req_releases WHERE project_id = ? AND name = ?`, p.id, name).Scan(&num)
	}
	if err != nil {
		return Release{}, err
	}
	return s.UpdateRelease(ctx, projectKey, fmtKey(p.key, "RL", num), in)
}
