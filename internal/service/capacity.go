package service

import (
	"context"
	"database/sql"
	"errors"
)

// Counts are the sizes the settings page compares with the scale taskapp was
// checked at (01 NFR-03, the capacity section).
type Counts struct {
	Issues         int    `json:"issues"`
	OpenIssues     int    `json:"open_issues"`
	Projects       int    `json:"projects"`
	LargestProject string `json:"largest_project"`
	LargestIssues  int    `json:"largest_project_issues"`
	Users          int    `json:"users"`
	Activities     int    `json:"activities"`
}

// Counts reads the counts for the capacity section.
func (s *Service) Counts(ctx context.Context) (Counts, error) {
	var c Counts
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM issues WHERE deleted_at IS NULL),
		(SELECT COUNT(*) FROM issues i JOIN workflow_states w ON w.id = i.state_id WHERE i.deleted_at IS NULL AND w.category NOT IN ('completed', 'canceled')),
		(SELECT COUNT(*) FROM projects),
		(SELECT COUNT(*) FROM users WHERE active = 1),
		(SELECT COUNT(*) FROM activities)`).Scan(&c.Issues, &c.OpenIssues, &c.Projects, &c.Users, &c.Activities)
	if err != nil {
		return Counts{}, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(p.key, ''), COUNT(i.id) FROM projects p LEFT JOIN issues i ON i.project_id = p.id AND i.deleted_at IS NULL
		GROUP BY p.id ORDER BY COUNT(i.id) DESC, p.key LIMIT 1`).Scan(&c.LargestProject, &c.LargestIssues)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Counts{}, err
	}
	return c, nil
}
