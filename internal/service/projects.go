package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/events"
)

// Project groups issues.
type Project struct {
	ID              string   `json:"id"`
	Key             string   `json:"key"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Owner           *UserRef `json:"owner"`
	AllowSkipLevels bool     `json:"allow_skip_levels"`
	Tier            string   `json:"tier"` // focus | normal | side
	// RequirementsMode is structured (option A) or quick (option D).
	RequirementsMode string  `json:"requirements_mode"`
	ArchivedAt       *string `json:"archived_at"`
	// DraftOf is the key of the project this one is a draft of (§5.9), or null.
	DraftOf   *string       `json:"draft_of"`
	CreatedAt string        `json:"created_at"`
	UpdatedAt string        `json:"updated_at"`
	Stats     *ProjectStats `json:"stats,omitempty"`

	JiraID int64 `json:"-"` // numeric id used by the Jira-compatible API
}

// ProjectStats summarizes issue counts.
type ProjectStats struct {
	Total  int            `json:"total"`
	Done   int            `json:"done"`
	ByType map[string]int `json:"by_type"`
}

// State is a workflow state.
type State struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	AppliesTo    string `json:"applies_to"` // all | qa
	Color        string `json:"color"`
	Position     int    `json:"position"`
	IsAcceptance bool   `json:"is_acceptance"`
	IsDefault    bool   `json:"is_default"`
}

// Label tags issues within a project.
type Label struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// defaultStates are created with every project (docs/04-er-diagram.md §2).
var defaultStates = []State{
	{Name: "Backlog", Category: "backlog", AppliesTo: "all", Color: "#9AA0B0"},
	{Name: "Todo", Category: "unstarted", AppliesTo: "all", Color: "#6B7285", IsDefault: true},
	{Name: "In Progress", Category: "started", AppliesTo: "all", Color: "#D9A21B"},
	{Name: "In Review", Category: "started", AppliesTo: "all", Color: "#2F7D8C"},
	{Name: "受入待ち", Category: "started", AppliesTo: "all", Color: "#5B58D6", IsAcceptance: true},
	{Name: "Done", Category: "completed", AppliesTo: "all", Color: "#2B7A4B"},
	{Name: "Canceled", Category: "canceled", AppliesTo: "all", Color: "#9AA0B0"},
	{Name: "未回答", Category: "unstarted", AppliesTo: "qa", Color: "#B85A1E", IsDefault: true},
	{Name: "回答済", Category: "started", AppliesTo: "qa", Color: "#2F7D8C"},
	{Name: "クローズ", Category: "completed", AppliesTo: "qa", Color: "#2B7A4B"},
}

// CreateProjectInput describes a new project.
type CreateProjectInput struct {
	Key             string `json:"key"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	AllowSkipLevels *bool  `json:"allow_skip_levels"`
	Tier            string `json:"tier"`
	// RequirementsMode defaults to quick for new projects.
	RequirementsMode string `json:"requirements_mode"`
}

// CreateProject creates a project with the default workflow states.
func (s *Service) CreateProject(ctx context.Context, in CreateProjectInput) (Project, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return Project{}, err
	}
	in.Key = strings.ToUpper(strings.TrimSpace(in.Key))
	in.Name = strings.TrimSpace(in.Name)
	if err := domain.ValidateProjectKey(in.Key); err != nil {
		return Project{}, Errorf(CodeInvalidArgument, "%s", err.Error())
	}
	if in.Name == "" {
		return Project{}, Errorf(CodeInvalidArgument, "name is required")
	}
	if in.Tier == "" {
		in.Tier = domain.TierNormal
	}
	if !domain.ValidTier(in.Tier) {
		return Project{}, Errorf(CodeInvalidArgument, "tier must be focus, normal or side")
	}
	if in.RequirementsMode == "" {
		in.RequirementsMode = domain.ReqModeQuick
	}
	if !domain.ValidRequirementsMode(in.RequirementsMode) {
		return Project{}, Errorf(CodeInvalidArgument, "requirements_mode must be structured or quick")
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := s.insertProject(ctx, tx, actor, in, nil)
		return err
	})
	if err != nil {
		return Project{}, err
	}
	s.bus.Publish(events.Event{Type: "project.created", ProjectKey: in.Key, ActorID: actor.User.ID})
	return s.GetProject(ctx, in.Key)
}

// insertProject writes a validated project and its default workflow states.
func (s *Service) insertProject(ctx context.Context, tx *sql.Tx, actor Actor, in CreateProjectInput, draftOf *string) (string, error) {
	allowSkip := true
	if in.AllowSkipLevels != nil {
		allowSkip = *in.AllowSkipLevels
	}
	now := s.timestamp()
	id := newID()
	var jiraID int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(jira_id), 9999) + 1 FROM projects`).Scan(&jiraID); err != nil {
		return "", err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO projects (id, jira_id, key, name, description, owner_id, allow_skip_levels, tier, requirements_mode, draft_of, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, id, jiraID, in.Key, in.Name, in.Description, actor.User.ID, allowSkip, in.Tier, in.RequirementsMode, nullString(draftOf), now, now)
	if isUniqueViolation(err) {
		return "", Errorf(CodeConflict, "project key %q already exists", in.Key)
	}
	if err != nil {
		return "", err
	}
	for i, st := range defaultStates {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_states (project_id, name, category, applies_to, color, position, is_acceptance, is_default)
			VALUES (?,?,?,?,?,?,?,?)`, id, st.Name, st.Category, st.AppliesTo, st.Color, i, st.IsAcceptance, st.IsDefault); err != nil {
			return "", err
		}
	}
	return id, nil
}

const projectSelect = `SELECT p.id, p.jira_id, p.key, p.name, p.description, p.allow_skip_levels, p.tier, p.requirements_mode, p.archived_at, p.created_at, p.updated_at,
	(SELECT t.key FROM projects t WHERE t.id = p.draft_of),
	` + "u.id, u.kind, u.username, u.display_name, u.avatar_color" + `
	FROM projects p JOIN users u ON u.id = p.owner_id`

func scanProject(row interface{ Scan(...any) error }) (Project, error) {
	var p Project
	var archived, draftOf sql.NullString
	owner := &UserRef{}
	err := row.Scan(&p.ID, &p.JiraID, &p.Key, &p.Name, &p.Description, &p.AllowSkipLevels, &p.Tier, &p.RequirementsMode, &archived, &p.CreatedAt, &p.UpdatedAt,
		&draftOf, &owner.ID, &owner.Kind, &owner.Username, &owner.DisplayName, &owner.AvatarColor)
	p.ArchivedAt = strPtr(archived)
	p.DraftOf = strPtr(draftOf)
	p.Owner = owner
	return p, err
}

// GetProject returns a project with stats.
func (s *Service) GetProject(ctx context.Context, key string) (Project, error) {
	p, err := scanProject(s.db.QueryRowContext(ctx, projectSelect+` WHERE p.key = ?`, strings.ToUpper(key)))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, Errorf(CodeNotFound, "project %q not found", key)
	}
	if err != nil {
		return Project{}, err
	}
	stats, err := s.projectStats(ctx, []string{p.ID})
	if err != nil {
		return Project{}, err
	}
	p.Stats = stats[p.ID]
	return p, nil
}

// ListProjects returns projects (archived ones only when requested). Drafts
// (§5.9) are never listed here; see ListDrafts.
func (s *Service) ListProjects(ctx context.Context, includeArchived bool) ([]Project, error) {
	q := projectSelect + ` WHERE p.draft_of IS NULL`
	if !includeArchived {
		q += ` AND p.archived_at IS NULL`
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY p.key`)
	if err != nil {
		return nil, err
	}
	out := []Project{}
	ids := []string{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, p)
		ids = append(ids, p.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	stats, err := s.projectStats(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Stats = stats[out[i].ID]
	}
	return out, nil
}

func (s *Service) projectStats(ctx context.Context, ids []string) (map[string]*ProjectStats, error) {
	out := map[string]*ProjectStats{}
	for _, id := range ids {
		out[id] = &ProjectStats{ByType: map[string]int{}}
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT i.project_id, i.type, COUNT(*),
			SUM(CASE WHEN st.category IN ('completed','canceled') THEN 1 ELSE 0 END)
		FROM issues i JOIN workflow_states st ON st.id = i.state_id
		WHERE i.deleted_at IS NULL AND i.project_id IN (`+placeholders(len(ids))+`)
		GROUP BY i.project_id, i.type`, toAny(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pid, typ string
		var n, done int
		if err := rows.Scan(&pid, &typ, &n, &done); err != nil {
			return nil, err
		}
		st := out[pid]
		st.ByType[typ] = n
		if typ != string(domain.TypeQA) {
			st.Total += n
			st.Done += done
		}
	}
	return out, rows.Err()
}

// UpdateProjectInput holds optional changes.
type UpdateProjectInput struct {
	Name             *string `json:"name"`
	Description      *string `json:"description"`
	AllowSkipLevels  *bool   `json:"allow_skip_levels"`
	Tier             *string `json:"tier"`
	RequirementsMode *string `json:"requirements_mode"`
	Archived         *bool   `json:"archived"`
}

// UpdateProject changes project settings. The key cannot change (it is part
// of every issue key and of the Jira API).
func (s *Service) UpdateProject(ctx context.Context, key string, in UpdateProjectInput) (Project, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return Project{}, err
	}
	p, err := s.GetProject(ctx, key)
	if err != nil {
		return Project{}, err
	}
	sets, args := []string{}, []any{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return Project{}, Errorf(CodeInvalidArgument, "name is required")
		}
		sets, args = append(sets, "name = ?"), append(args, name)
	}
	if in.Description != nil {
		sets, args = append(sets, "description = ?"), append(args, *in.Description)
	}
	if in.AllowSkipLevels != nil {
		sets, args = append(sets, "allow_skip_levels = ?"), append(args, *in.AllowSkipLevels)
	}
	if in.Tier != nil {
		if !domain.ValidTier(*in.Tier) {
			return Project{}, Errorf(CodeInvalidArgument, "tier must be focus, normal or side")
		}
		sets, args = append(sets, "tier = ?"), append(args, *in.Tier)
	}
	if in.RequirementsMode != nil {
		if !domain.ValidRequirementsMode(*in.RequirementsMode) {
			return Project{}, Errorf(CodeInvalidArgument, "requirements_mode must be structured or quick")
		}
		sets, args = append(sets, "requirements_mode = ?"), append(args, *in.RequirementsMode)
	}
	if in.Archived != nil {
		if *in.Archived {
			sets, args = append(sets, "archived_at = ?"), append(args, s.timestamp())
		} else {
			sets = append(sets, "archived_at = NULL")
		}
	}
	if len(sets) > 0 {
		sets, args = append(sets, "updated_at = ?"), append(args, s.timestamp())
		args = append(args, p.ID)
		if _, err := s.db.ExecContext(ctx, `UPDATE projects SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...); err != nil {
			return Project{}, err
		}
		s.bus.Publish(events.Event{Type: "project.updated", ProjectKey: p.Key, ActorID: actor.User.ID})
	}
	return s.GetProject(ctx, key)
}

// ListStates returns the project's workflow states in order.
func (s *Service) ListStates(ctx context.Context, projectKey string) ([]State, error) {
	p, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	return s.statesForProject(ctx, s.db, p.ID)
}

func (s *Service) statesForProject(ctx context.Context, q queryer, projectID string) ([]State, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, name, category, applies_to, color, position, is_acceptance, is_default
		FROM workflow_states WHERE project_id = ? ORDER BY applies_to, position`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []State{}
	for rows.Next() {
		var st State
		if err := rows.Scan(&st.ID, &st.Name, &st.Category, &st.AppliesTo, &st.Color, &st.Position, &st.IsAcceptance, &st.IsDefault); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// ListLabels returns the project's labels.
func (s *Service) ListLabels(ctx context.Context, projectKey string) ([]Label, error) {
	p, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, color FROM labels WHERE project_id = ? ORDER BY name`, p.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Label{}
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.Name, &l.Color); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// CreateLabel adds a label to a project.
func (s *Service) CreateLabel(ctx context.Context, projectKey, name, color string) (Label, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return Label{}, err
	}
	p, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return Label{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Label{}, Errorf(CodeInvalidArgument, "label name is required")
	}
	if color == "" {
		color = "#6B7285"
	}
	l := Label{ID: newID(), Name: name, Color: color}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO labels (id, project_id, name, color) VALUES (?,?,?,?)`, l.ID, p.ID, l.Name, l.Color); err != nil {
		if isUniqueViolation(err) {
			return Label{}, Errorf(CodeConflict, "label %q already exists", name)
		}
		return Label{}, err
	}
	s.bus.Publish(events.Event{Type: "project.updated", ProjectKey: p.Key, ActorID: actor.User.ID})
	return l, nil
}

func placeholders(n int) string {
	if n == 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
