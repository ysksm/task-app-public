package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/ysksm/task-app/internal/domain"
)

// Saved views (VIEW-05): a named filter plus display settings, owned by a
// user, either for one project or across projects.

// SavedView is a saved filter.
type SavedView struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	ProjectKey *string         `json:"project_key"` // null = across projects
	Filter     domain.Filter   `json:"filter"`
	Display    json.RawMessage `json:"display"` // UI settings, e.g. {"view":"map","group":"state"}
	Position   int             `json:"position"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
}

// SavedViewInput creates a view.
type SavedViewInput struct {
	Name       string          `json:"name"`
	ProjectKey *string         `json:"project_key"`
	Filter     domain.Filter   `json:"filter"`
	Display    json.RawMessage `json:"display"`
}

// SavedViewPatch changes a view.
type SavedViewPatch struct {
	Name     *string          `json:"name"`
	Filter   *domain.Filter   `json:"filter"`
	Display  *json.RawMessage `json:"display"`
	Position *int             `json:"position"`
}

const viewSelect = `SELECT v.id, v.name, p.key, v.filter, v.display, v.position, v.created_at, v.updated_at
	FROM saved_views v LEFT JOIN projects p ON p.id = v.project_id`

func scanView(row interface{ Scan(...any) error }) (SavedView, error) {
	var v SavedView
	var pkey sql.NullString
	var filter, display string
	if err := row.Scan(&v.ID, &v.Name, &pkey, &filter, &display, &v.Position, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return v, err
	}
	v.ProjectKey = strPtr(pkey)
	_ = json.Unmarshal([]byte(filter), &v.Filter)
	v.Display = json.RawMessage(display)
	return v, nil
}

func validDisplay(d json.RawMessage) (string, error) {
	if len(d) == 0 || string(d) == "null" {
		return "{}", nil
	}
	var obj map[string]any
	if err := json.Unmarshal(d, &obj); err != nil {
		return "", Errorf(CodeInvalidArgument, "display must be a JSON object")
	}
	return string(d), nil
}

// ListViews returns the caller's views in sidebar order.
func (s *Service) ListViews(ctx context.Context) ([]SavedView, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, viewSelect+` WHERE v.owner_id = ? ORDER BY v.position, v.created_at`, actor.User.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavedView{}
	for rows.Next() {
		v, err := scanView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetView returns one of the caller's views.
func (s *Service) GetView(ctx context.Context, id string) (SavedView, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return SavedView{}, err
	}
	v, err := scanView(s.db.QueryRowContext(ctx, viewSelect+` WHERE v.id = ? AND v.owner_id = ?`, id, actor.User.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return SavedView{}, Errorf(CodeNotFound, "view %s not found", id)
	}
	return v, err
}

// CreateView saves a view for the caller.
func (s *Service) CreateView(ctx context.Context, in SavedViewInput) (SavedView, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return SavedView{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return SavedView{}, Errorf(CodeInvalidArgument, "name is required")
	}
	if err := in.Filter.Validate(); err != nil {
		return SavedView{}, Errorf(CodeInvalidArgument, "%s", err.Error())
	}
	display, err := validDisplay(in.Display)
	if err != nil {
		return SavedView{}, err
	}
	var projectID any
	if in.ProjectKey != nil && *in.ProjectKey != "" {
		p, err := s.GetProject(ctx, *in.ProjectKey)
		if err != nil {
			return SavedView{}, err
		}
		projectID = p.ID
	}
	filter, _ := json.Marshal(in.Filter)
	id, now := newID(), s.timestamp()
	_, err = s.db.ExecContext(ctx, `INSERT INTO saved_views (id, project_id, owner_id, name, filter, display, position, created_at, updated_at)
		VALUES (?,?,?,?,?,?,(SELECT COALESCE(MAX(position), 0) + 1 FROM saved_views WHERE owner_id = ?),?,?)`,
		id, projectID, actor.User.ID, in.Name, string(filter), display, actor.User.ID, now, now)
	if err != nil {
		return SavedView{}, err
	}
	return s.GetView(ctx, id)
}

// UpdateView changes one of the caller's views.
func (s *Service) UpdateView(ctx context.Context, id string, in SavedViewPatch) (SavedView, error) {
	v, err := s.GetView(ctx, id)
	if err != nil {
		return SavedView{}, err
	}
	sets, args := []string{}, []any{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return SavedView{}, Errorf(CodeInvalidArgument, "name is required")
		}
		sets, args = append(sets, "name = ?"), append(args, name)
	}
	if in.Filter != nil {
		if err := in.Filter.Validate(); err != nil {
			return SavedView{}, Errorf(CodeInvalidArgument, "%s", err.Error())
		}
		b, _ := json.Marshal(in.Filter)
		sets, args = append(sets, "filter = ?"), append(args, string(b))
	}
	if in.Display != nil {
		d, err := validDisplay(*in.Display)
		if err != nil {
			return SavedView{}, err
		}
		sets, args = append(sets, "display = ?"), append(args, d)
	}
	if in.Position != nil {
		sets, args = append(sets, "position = ?"), append(args, *in.Position)
	}
	if len(sets) > 0 {
		sets, args = append(sets, "updated_at = ?"), append(args, s.timestamp())
		if _, err := s.db.ExecContext(ctx, `UPDATE saved_views SET `+strings.Join(sets, ", ")+` WHERE id = ?`, append(args, v.ID)...); err != nil {
			return SavedView{}, err
		}
	}
	return s.GetView(ctx, id)
}

// DeleteView removes one of the caller's views.
func (s *Service) DeleteView(ctx context.Context, id string) error {
	v, err := s.GetView(ctx, id)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM saved_views WHERE id = ?`, v.ID)
	return err
}
