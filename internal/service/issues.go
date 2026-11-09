package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/events"
	"github.com/ysksm/task-app/internal/store"
)

// Issue is the API representation of an issue.
type Issue struct {
	ID          string   `json:"id"`
	Key         string   `json:"key"`
	Number      int      `json:"number"`
	ProjectKey  string   `json:"project_key"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	State       StateRef `json:"state"`
	Priority    int      `json:"priority"`
	Assignee    *UserRef `json:"assignee"`
	Reporter    *UserRef `json:"reporter"`
	Labels      []Label  `json:"labels"`
	Estimate    *float64 `json:"estimate"`
	DueDate     *string  `json:"due_date"`
	ParentKey   *string  `json:"parent_key"`
	SortOrder   float64  `json:"sort_order"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	StartedAt   *string  `json:"started_at"`
	CompletedAt *string  `json:"completed_at"`
	CanceledAt  *string  `json:"canceled_at"`
	ChildCount  int      `json:"child_count"`
	OpenQACount int      `json:"open_qa_count"`     // direct QA children not answered yet
	BlockingQA  int      `json:"blocking_qa_count"` // ...of which marked blocking
	Progress    Progress `json:"progress"`

	// Detail-only fields.
	Ancestors []IssueRef `json:"ancestors,omitempty"`
	Children  []Issue    `json:"children,omitempty"`
	// Stories (requirements layer) this issue helps satisfy: why it exists.
	Stories *[]StoryRef `json:"stories,omitempty"`

	JiraID int64 `json:"-"` // numeric id used by the Jira-compatible API

	projectID string
	parentID  *string
}

// StateRef is the state embedded in an issue.
type StateRef struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	Color        string `json:"color"`
	IsAcceptance bool   `json:"is_acceptance"`
}

// IssueRef is a compact issue reference.
type IssueRef struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

// Progress counts non-QA descendants.
type Progress struct {
	Total int `json:"total"`
	Done  int `json:"done"`
}

const issueSelect = `SELECT i.id, i.jira_id, i.key, i.number, p.key, p.id, i.type, i.title, i.description, i.priority, i.estimate, i.due_date,
	i.sort_order, i.created_at, i.updated_at, i.started_at, i.completed_at, i.canceled_at, i.parent_id,
	st.id, st.name, st.category, st.color, st.is_acceptance,
	par.key,
	a.id, a.kind, a.username, a.display_name, a.avatar_color,
	r.id, r.kind, r.username, r.display_name, r.avatar_color
FROM issues i
JOIN projects p ON p.id = i.project_id
JOIN workflow_states st ON st.id = i.state_id
LEFT JOIN issues par ON par.id = i.parent_id
LEFT JOIN users a ON a.id = i.assignee_id
JOIN users r ON r.id = i.reporter_id`

func scanIssue(row interface{ Scan(...any) error }) (Issue, error) {
	var is Issue
	var estimate sql.NullFloat64
	var due, started, completed, canceled, parentID, parentKey sql.NullString
	var aID, aKind, aUser, aName, aColor sql.NullString
	r := &UserRef{}
	err := row.Scan(&is.ID, &is.JiraID, &is.Key, &is.Number, &is.ProjectKey, &is.projectID, &is.Type, &is.Title, &is.Description, &is.Priority,
		&estimate, &due, &is.SortOrder, &is.CreatedAt, &is.UpdatedAt, &started, &completed, &canceled, &parentID,
		&is.State.ID, &is.State.Name, &is.State.Category, &is.State.Color, &is.State.IsAcceptance,
		&parentKey,
		&aID, &aKind, &aUser, &aName, &aColor,
		&r.ID, &r.Kind, &r.Username, &r.DisplayName, &r.AvatarColor)
	if err != nil {
		return is, err
	}
	if estimate.Valid {
		v := estimate.Float64
		is.Estimate = &v
	}
	is.DueDate, is.StartedAt, is.CompletedAt, is.CanceledAt = strPtr(due), strPtr(started), strPtr(completed), strPtr(canceled)
	is.parentID, is.ParentKey = strPtr(parentID), strPtr(parentKey)
	if aID.Valid {
		is.Assignee = &UserRef{ID: aID.String, Kind: aKind.String, Username: aUser.String, DisplayName: aName.String, AvatarColor: aColor.String}
	}
	is.Reporter = r
	is.Labels = []Label{}
	return is, nil
}

// ---- read ----

// GetIssue returns an issue with ancestors and children.
func (s *Service) GetIssue(ctx context.Context, key string) (Issue, error) {
	is, err := s.issueByKey(ctx, s.db, key)
	if err != nil {
		return Issue{}, err
	}
	list := []Issue{is}
	if err := s.enrich(ctx, list); err != nil {
		return Issue{}, err
	}
	is = list[0]
	if is.Ancestors, err = s.ancestors(ctx, s.db, is.ID); err != nil {
		return Issue{}, err
	}
	children, err := s.queryIssues(ctx, s.db, `WHERE i.parent_id = ? AND i.deleted_at IS NULL ORDER BY i.sort_order, i.number`, is.ID)
	if err != nil {
		return Issue{}, err
	}
	if err := s.enrich(ctx, children); err != nil {
		return Issue{}, err
	}
	is.Children = children
	stories, err := s.storiesOfIssue(ctx, is)
	if err != nil {
		return Issue{}, err
	}
	is.Stories = &stories
	return is, nil
}

func (s *Service) issueByKey(ctx context.Context, q queryer, key string) (Issue, error) {
	is, err := scanIssue(q.QueryRowContext(ctx, issueSelect+` WHERE i.key = ? AND i.deleted_at IS NULL`, strings.ToUpper(key)))
	if errors.Is(err, sql.ErrNoRows) {
		return Issue{}, Errorf(CodeNotFound, "issue %q not found", key)
	}
	return is, err
}

func (s *Service) queryIssues(ctx context.Context, q queryer, where string, args ...any) ([]Issue, error) {
	rows, err := q.QueryContext(ctx, issueSelect+" "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Issue{}
	for rows.Next() {
		is, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, is)
	}
	return out, rows.Err()
}

// ancestors returns the chain from the root down to the direct parent.
func (s *Service) ancestors(ctx context.Context, q queryer, issueID string) ([]IssueRef, error) {
	rows, err := q.QueryContext(ctx, `WITH RECURSIVE anc(id, parent_id, depth) AS (
			SELECT id, parent_id, 0 FROM issues WHERE id = ?
			UNION ALL
			SELECT i.id, i.parent_id, anc.depth + 1 FROM anc CROSS JOIN issues i ON i.id = anc.parent_id
		)
		SELECT i.key, i.type, i.title FROM anc JOIN issues i ON i.id = anc.id WHERE anc.depth > 0 ORDER BY anc.depth DESC`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IssueRef{}
	for rows.Next() {
		var r IssueRef
		if err := rows.Scan(&r.Key, &r.Type, &r.Title); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// enrich fills labels, progress, child and open-QA counts.
func (s *Service) enrich(ctx context.Context, list []Issue) error {
	if len(list) == 0 {
		return nil
	}
	idx := map[string]*Issue{}
	ids := make([]string, 0, len(list))
	for i := range list {
		idx[list[i].ID] = &list[i]
		ids = append(ids, list[i].ID)
	}
	// SQLite limits the number of host parameters; process in chunks.
	for start := 0; start < len(ids); start += 500 {
		end := min(start+500, len(ids))
		chunk := toAny(ids[start:end])
		ph := placeholders(end - start)

		rows, err := s.db.QueryContext(ctx, `SELECT il.issue_id, l.id, l.name, l.color FROM issue_labels il
			JOIN labels l ON l.id = il.label_id WHERE il.issue_id IN (`+ph+`) ORDER BY l.name`, chunk...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var issueID string
			var l Label
			if err := rows.Scan(&issueID, &l.ID, &l.Name, &l.Color); err != nil {
				rows.Close()
				return err
			}
			idx[issueID].Labels = append(idx[issueID].Labels, l)
		}
		rows.Close()

		rows, err = s.db.QueryContext(ctx, `SELECT c.parent_id, COUNT(*),
				SUM(CASE WHEN c.type = 'qa' AND st.category IN ('backlog','unstarted') THEN 1 ELSE 0 END),
				SUM(CASE WHEN c.type = 'qa' AND st.category IN ('backlog','unstarted') AND q.is_blocking = 1 THEN 1 ELSE 0 END)
			FROM issues c JOIN workflow_states st ON st.id = c.state_id LEFT JOIN qa_details q ON q.issue_id = c.id
			WHERE c.deleted_at IS NULL AND c.parent_id IN (`+ph+`) GROUP BY c.parent_id`, chunk...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var pid string
			var n, openQA, blocking int
			if err := rows.Scan(&pid, &n, &openQA, &blocking); err != nil {
				rows.Close()
				return err
			}
			idx[pid].ChildCount, idx[pid].OpenQACount, idx[pid].BlockingQA = n, openQA, blocking
		}
		rows.Close()

		rows, err = s.db.QueryContext(ctx, `WITH RECURSIVE sub(root, id) AS (
				SELECT c.parent_id, c.id FROM issues c WHERE c.deleted_at IS NULL AND c.parent_id IN (`+ph+`)
				UNION ALL
				SELECT sub.root, c.id FROM sub CROSS JOIN issues c ON c.parent_id = sub.id WHERE c.deleted_at IS NULL
			)
			SELECT sub.root, COUNT(*), SUM(CASE WHEN st.category IN ('completed','canceled') THEN 1 ELSE 0 END)
			FROM sub JOIN issues i ON i.id = sub.id JOIN workflow_states st ON st.id = i.state_id
			WHERE i.type <> 'qa' GROUP BY sub.root`, chunk...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var root string
			var p Progress
			if err := rows.Scan(&root, &p.Total, &p.Done); err != nil {
				rows.Close()
				return err
			}
			idx[root].Progress = p
		}
		rows.Close()
	}
	return nil
}

// ListOptions controls ListIssues.
type ListOptions struct {
	Filter           domain.Filter
	Sort             string // updated (default) | created | priority | manual | key | due_date, or one of updated_asc | created_asc | key_desc
	Limit            int
	Offset           int
	IncludeAncestors bool // also return ancestors of matches (tree view)
	IncludeDeleted   bool
}

// ListResult is a page of issues.
type ListResult struct {
	Issues    []Issue `json:"issues"`
	Total     int     `json:"total"`
	Ancestors []Issue `json:"ancestors,omitempty"` // ancestors of matches that did not match themselves
}

var sortClauses = map[string]string{
	"updated":  "i.updated_at DESC, i.id",
	"created":  "i.created_at DESC, i.id",
	"priority": "CASE i.priority WHEN 0 THEN 5 ELSE i.priority END, i.updated_at DESC",
	"manual":   "i.sort_order, i.number",
	"key":      "p.key, i.number",
	"due_date": "i.due_date IS NULL, i.due_date, i.priority",

	"updated_asc": "i.updated_at, i.id",
	"created_asc": "i.created_at, i.id",
	"key_desc":    "p.key DESC, i.number DESC",
}

// ListIssues searches issues across projects.
func (s *Service) ListIssues(ctx context.Context, opt ListOptions) (ListResult, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return ListResult{}, err
	}
	if err := opt.Filter.Validate(); err != nil {
		return ListResult{}, Errorf(CodeInvalidArgument, "%s", err.Error())
	}
	cond, args, err := store.FilterSQL(opt.Filter, actor.User.ID)
	if err != nil {
		return ListResult{}, Errorf(CodeInvalidArgument, "%s", err.Error())
	}
	where := "WHERE (" + cond + ")"
	if !opt.IncludeDeleted {
		where += " AND i.deleted_at IS NULL"
	}
	where += " AND p.archived_at IS NULL"
	order, ok := sortClauses[opt.Sort]
	if opt.Sort == "" {
		order, ok = sortClauses["updated"], true
	}
	if !ok {
		return ListResult{}, Errorf(CodeInvalidArgument, "unknown sort %q", opt.Sort)
	}
	if opt.Limit <= 0 || opt.Limit > 5000 {
		opt.Limit = 500
	}

	var res ListResult
	countSQL := `SELECT COUNT(*) FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN workflow_states st ON st.id = i.state_id LEFT JOIN users a ON a.id = i.assignee_id ` + where
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&res.Total); err != nil {
		return ListResult{}, err
	}
	pageArgs := append(append([]any{}, args...), opt.Limit, opt.Offset)
	res.Issues, err = s.queryIssues(ctx, s.db, where+" ORDER BY "+order+" LIMIT ? OFFSET ?", pageArgs...)
	if err != nil {
		return ListResult{}, err
	}
	if err := s.enrich(ctx, res.Issues); err != nil {
		return ListResult{}, err
	}
	if opt.IncludeAncestors {
		if res.Ancestors, err = s.missingAncestors(ctx, res.Issues); err != nil {
			return ListResult{}, err
		}
	}
	return res, nil
}

// missingAncestors returns ancestors of the given issues that are not in the list.
func (s *Service) missingAncestors(ctx context.Context, list []Issue) ([]Issue, error) {
	have := map[string]bool{}
	for _, is := range list {
		have[is.ID] = true
	}
	need := map[string]bool{}
	for _, is := range list {
		if is.parentID != nil && !have[*is.parentID] {
			need[*is.parentID] = true
		}
	}
	out := []Issue{}
	for len(need) > 0 {
		ids := make([]string, 0, len(need))
		for id := range need {
			ids = append(ids, id)
			have[id] = true
		}
		need = map[string]bool{}
		for start := 0; start < len(ids); start += 500 {
			end := min(start+500, len(ids))
			found, err := s.queryIssues(ctx, s.db, `WHERE i.id IN (`+placeholders(end-start)+`)`, toAny(ids[start:end])...)
			if err != nil {
				return nil, err
			}
			for _, is := range found {
				out = append(out, is)
				if is.parentID != nil && !have[*is.parentID] {
					need[*is.parentID] = true
				}
			}
		}
	}
	return out, s.enrich(ctx, out)
}

// ---- write ----

// CreateIssueInput describes a new issue.
type CreateIssueInput struct {
	ProjectKey  string   `json:"project_key"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	ParentKey   *string  `json:"parent_key"`
	StateID     *int64   `json:"state_id"`
	Priority    *int     `json:"priority"`
	Assignee    *string  `json:"assignee"` // username or "me"
	Labels      []string `json:"labels"`   // label names (created if missing)
	Estimate    *float64 `json:"estimate"`
	DueDate     *string  `json:"due_date"`
}

// CreateIssue creates an issue, enforcing the hierarchy rules.
func (s *Service) CreateIssue(ctx context.Context, in CreateIssueInput) (Issue, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return Issue{}, err
	}
	var created createdIssue
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		created, err = s.createIssueTx(ctx, tx, actor, in)
		return err
	})
	if err != nil {
		return Issue{}, err
	}
	s.bus.Publish(events.Event{Type: "issue.created", ProjectKey: created.projectKey, IssueKey: created.key, ActorID: actor.User.ID})
	return s.GetIssue(ctx, created.key)
}

type createdIssue struct {
	id, key, projectKey, projectID string
}

// createIssueTx validates and inserts an issue inside tx.
func (s *Service) createIssueTx(ctx context.Context, tx *sql.Tx, actor Actor, in CreateIssueInput) (createdIssue, error) {
	typ := domain.IssueType(in.Type)
	if !typ.Valid() {
		return createdIssue{}, Errorf(CodeInvalidArgument, "unknown issue type %q", in.Type)
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return createdIssue{}, Errorf(CodeInvalidArgument, "title is required")
	}
	if in.Priority != nil && (*in.Priority < 0 || *in.Priority > 4) {
		return createdIssue{}, Errorf(CodeInvalidArgument, "priority must be 0-4")
	}
	if err := validateDate(in.DueDate); err != nil {
		return createdIssue{}, err
	}
	var parent *Issue
	if in.ParentKey != nil && *in.ParentKey != "" {
		p, err := s.issueByKey(ctx, tx, *in.ParentKey)
		if err != nil {
			return createdIssue{}, Errorf(CodeInvalidArgument, "parent %q not found", *in.ParentKey)
		}
		parent = &p
		if in.ProjectKey == "" {
			in.ProjectKey = p.ProjectKey
		}
	}
	if in.ProjectKey == "" {
		return createdIssue{}, Errorf(CodeInvalidArgument, "project_key or parent_key is required")
	}
	var projectID string
	var allowSkip bool
	err := tx.QueryRowContext(ctx, `SELECT id, allow_skip_levels FROM projects WHERE key = ? AND archived_at IS NULL`,
		strings.ToUpper(in.ProjectKey)).Scan(&projectID, &allowSkip)
	if errors.Is(err, sql.ErrNoRows) {
		return createdIssue{}, Errorf(CodeNotFound, "project %q not found", in.ProjectKey)
	}
	if err != nil {
		return createdIssue{}, err
	}
	in.ProjectKey = strings.ToUpper(in.ProjectKey)
	var parentType *domain.IssueType
	var parentID any
	if parent != nil {
		if parent.projectID != projectID {
			return createdIssue{}, Errorf(CodeHierarchyViolation, "parent %s belongs to another project", parent.Key)
		}
		pt := domain.IssueType(parent.Type)
		parentType, parentID = &pt, parent.ID
	}
	if err := domain.CanBeChild(parentType, typ, allowSkip); err != nil {
		return createdIssue{}, err
	}
	stateID, err := s.resolveState(ctx, tx, projectID, typ, in.StateID)
	if err != nil {
		return createdIssue{}, err
	}
	var assigneeID any
	if in.Assignee != nil && *in.Assignee != "" {
		id, err := s.resolveUserID(ctx, tx, actor, *in.Assignee)
		if err != nil {
			return createdIssue{}, Errorf(CodeInvalidArgument, "assignee %q not found", *in.Assignee)
		}
		assigneeID = id
	}
	var number int
	if err := tx.QueryRowContext(ctx, `UPDATE projects SET next_issue_number = next_issue_number + 1 WHERE id = ? RETURNING next_issue_number - 1`,
		projectID).Scan(&number); err != nil {
		return createdIssue{}, err
	}
	var jiraID int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(jira_id), 9999) + 1 FROM issues`).Scan(&jiraID); err != nil {
		return createdIssue{}, err
	}
	var sortOrder float64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1024 FROM issues WHERE project_id = ? AND parent_id IS ?`,
		projectID, parentID).Scan(&sortOrder); err != nil {
		return createdIssue{}, err
	}
	priority := 0
	if in.Priority != nil {
		priority = *in.Priority
	}
	now := s.timestamp()
	key := domain.IssueKey(in.ProjectKey, number)
	id := newID()
	var startedAt, completedAt, canceledAt any
	cat, err := stateCategory(ctx, tx, stateID)
	if err != nil {
		return createdIssue{}, err
	}
	switch cat {
	case domain.CategoryStarted:
		startedAt = now
	case domain.CategoryCompleted:
		startedAt, completedAt = now, now
	case domain.CategoryCanceled:
		canceledAt = now
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO issues (id, jira_id, project_id, number, key, type, parent_id, title, description, state_id,
			priority, assignee_id, reporter_id, estimate, due_date, sort_order, started_at, completed_at, canceled_at, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, jiraID, projectID, number, key, string(typ), parentID, in.Title, in.Description, stateID,
		priority, assigneeID, actor.User.ID, in.Estimate, nullString(in.DueDate), sortOrder, startedAt, completedAt, canceledAt, now, now)
	if err != nil {
		return createdIssue{}, err
	}
	if err := s.setLabels(ctx, tx, projectID, id, in.Labels); err != nil {
		return createdIssue{}, err
	}
	if err := s.logActivity(ctx, tx, id, actor, "created", "", "", ""); err != nil {
		return createdIssue{}, err
	}
	return createdIssue{id: id, key: key, projectKey: in.ProjectKey, projectID: projectID}, nil
}

// resolveState validates a requested state or picks the default one for the type.
func (s *Service) resolveState(ctx context.Context, q queryer, projectID string, typ domain.IssueType, requested *int64) (int64, error) {
	appliesTo := "all"
	if typ == domain.TypeQA {
		appliesTo = "qa"
	}
	if requested != nil {
		var ok bool
		err := q.QueryRowContext(ctx, `SELECT 1 FROM workflow_states WHERE id = ? AND project_id = ? AND applies_to = ?`,
			*requested, projectID, appliesTo).Scan(&ok)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, Errorf(CodeInvalidArgument, "state %d is not valid for a %s in this project", *requested, typ)
		}
		return *requested, err
	}
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM workflow_states WHERE project_id = ? AND applies_to = ?
		ORDER BY is_default DESC, position LIMIT 1`, projectID, appliesTo).Scan(&id)
	return id, err
}

func stateCategory(ctx context.Context, q queryer, stateID int64) (domain.StateCategory, error) {
	var c string
	err := q.QueryRowContext(ctx, `SELECT category FROM workflow_states WHERE id = ?`, stateID).Scan(&c)
	return domain.StateCategory(c), err
}

// setLabels replaces an issue's labels, creating unknown label names.
func (s *Service) setLabels(ctx context.Context, tx *sql.Tx, projectID, issueID string, names []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM issue_labels WHERE issue_id = ?`, issueID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		var labelID string
		err := tx.QueryRowContext(ctx, `SELECT id FROM labels WHERE project_id = ? AND name = ?`, projectID, name).Scan(&labelID)
		if errors.Is(err, sql.ErrNoRows) {
			labelID = newID()
			_, err = tx.ExecContext(ctx, `INSERT INTO labels (id, project_id, name, color) VALUES (?,?,?,?)`, labelID, projectID, name, "#6B7285")
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO issue_labels (issue_id, label_id) VALUES (?,?)`, issueID, labelID); err != nil {
			return err
		}
	}
	return nil
}

// UpdateIssueInput holds PATCH fields. Absent fields are left unchanged; null
// clears nullable fields.
type UpdateIssueInput struct {
	Title       Opt[string]   `json:"title"`
	Description Opt[string]   `json:"description"`
	Type        Opt[string]   `json:"type"`
	StateID     Opt[int64]    `json:"state_id"`
	Priority    Opt[int]      `json:"priority"`
	Assignee    Opt[string]   `json:"assignee"`
	Labels      Opt[[]string] `json:"labels"`
	Estimate    Opt[float64]  `json:"estimate"`
	DueDate     Opt[string]   `json:"due_date"`
	ParentKey   Opt[string]   `json:"parent_key"`
	SortOrder   Opt[float64]  `json:"sort_order"`
}

// UpdateIssue applies changes, re-validating the hierarchy when the type or
// parent changes, and logs one activity entry per changed field.
func (s *Service) UpdateIssue(ctx context.Context, key string, in UpdateIssueInput) (Issue, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return Issue{}, err
	}
	var cur Issue
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		cur, err = s.issueByKey(ctx, tx, key)
		if err != nil {
			return err
		}
		sets, args := []string{}, []any{}
		set := func(col string, v any) { sets, args = append(sets, col+" = ?"), append(args, v) }
		changes := [][3]string{} // field, old, new

		if in.Title.Set {
			t := strings.TrimSpace(in.Title.Value)
			if in.Title.Null || t == "" {
				return Errorf(CodeInvalidArgument, "title is required")
			}
			if t != cur.Title {
				set("title", t)
				changes = append(changes, [3]string{"title", cur.Title, t})
			}
		}
		if in.Description.Set && in.Description.Value != cur.Description {
			set("description", in.Description.Value)
			changes = append(changes, [3]string{"description", "", ""})
		}
		if in.Priority.Set {
			p := in.Priority.Value
			if in.Priority.Null {
				p = 0
			}
			if p < 0 || p > 4 {
				return Errorf(CodeInvalidArgument, "priority must be 0-4")
			}
			if p != cur.Priority {
				set("priority", p)
				changes = append(changes, [3]string{"priority", strconv.Itoa(cur.Priority), strconv.Itoa(p)})
			}
		}
		if in.Assignee.Set {
			oldName := ""
			if cur.Assignee != nil {
				oldName = cur.Assignee.Username
			}
			if in.Assignee.Null || in.Assignee.Value == "" {
				if cur.Assignee != nil {
					set("assignee_id", nil)
					changes = append(changes, [3]string{"assignee", oldName, ""})
				}
			} else {
				id, err := s.resolveUserID(ctx, tx, actor, in.Assignee.Value)
				if err != nil {
					return Errorf(CodeInvalidArgument, "assignee %q not found", in.Assignee.Value)
				}
				if cur.Assignee == nil || cur.Assignee.ID != id {
					var newName string
					tx.QueryRowContext(ctx, `SELECT username FROM users WHERE id = ?`, id).Scan(&newName)
					set("assignee_id", id)
					changes = append(changes, [3]string{"assignee", oldName, newName})
				}
			}
		}
		if in.Estimate.Set {
			if in.Estimate.Null {
				set("estimate", nil)
			} else {
				set("estimate", in.Estimate.Value)
			}
			changes = append(changes, [3]string{"estimate", fmtFloat(cur.Estimate), fmtOptFloat(in.Estimate)})
		}
		if in.DueDate.Set {
			var v *string
			if !in.DueDate.Null && in.DueDate.Value != "" {
				v = &in.DueDate.Value
			}
			if err := validateDate(v); err != nil {
				return err
			}
			set("due_date", nullString(v))
			changes = append(changes, [3]string{"due_date", deref(cur.DueDate), deref(v)})
		}
		if in.SortOrder.Set && !in.SortOrder.Null {
			set("sort_order", in.SortOrder.Value)
		}

		// Type and parent are validated together against parent and children.
		newType := domain.IssueType(cur.Type)
		if in.Type.Set && !in.Type.Null && in.Type.Value != cur.Type {
			newType = domain.IssueType(in.Type.Value)
			if !newType.Valid() {
				return Errorf(CodeInvalidArgument, "unknown issue type %q", in.Type.Value)
			}
			if (newType == domain.TypeQA) != (cur.Type == string(domain.TypeQA)) {
				return Errorf(CodeInvalidArgument, "cannot convert between QA and other issue types")
			}
			set("type", string(newType))
			changes = append(changes, [3]string{"type", cur.Type, string(newType)})
		}
		parentID, parentKey := cur.parentID, deref(cur.ParentKey)
		if in.ParentKey.Set {
			newKey := ""
			if !in.ParentKey.Null {
				newKey = strings.ToUpper(in.ParentKey.Value)
			}
			if newKey != parentKey {
				if newKey == "" {
					parentID = nil
				} else {
					p, err := s.issueByKey(ctx, tx, newKey)
					if err != nil {
						return Errorf(CodeInvalidArgument, "parent %q not found", newKey)
					}
					if p.projectID != cur.projectID {
						return Errorf(CodeHierarchyViolation, "parent %s belongs to another project", p.Key)
					}
					if err := s.checkNoCycle(ctx, tx, cur.ID, p.ID); err != nil {
						return err
					}
					parentID = &p.ID
				}
				set("parent_id", nullString(parentID))
				changes = append(changes, [3]string{"parent", parentKey, newKey})
			}
		}
		if newType != domain.IssueType(cur.Type) || in.ParentKey.Set {
			if err := s.checkPlacement(ctx, tx, cur, newType, parentID); err != nil {
				return err
			}
		}

		if in.StateID.Set && !in.StateID.Null && in.StateID.Value != cur.State.ID {
			stateID, err := s.resolveState(ctx, tx, cur.projectID, newType, &in.StateID.Value)
			if err != nil {
				return err
			}
			var newName string
			tx.QueryRowContext(ctx, `SELECT name FROM workflow_states WHERE id = ?`, stateID).Scan(&newName)
			set("state_id", stateID)
			changes = append(changes, [3]string{"state", cur.State.Name, newName})
			cat, err := stateCategory(ctx, tx, stateID)
			if err != nil {
				return err
			}
			now := s.timestamp()
			switch cat {
			case domain.CategoryBacklog, domain.CategoryUnstarted:
				set("completed_at", nil)
				set("canceled_at", nil)
			case domain.CategoryStarted:
				if cur.StartedAt == nil {
					set("started_at", now)
				}
				set("completed_at", nil)
				set("canceled_at", nil)
			case domain.CategoryCompleted:
				if cur.StartedAt == nil {
					set("started_at", now)
				}
				set("completed_at", now)
				set("canceled_at", nil)
			case domain.CategoryCanceled:
				set("canceled_at", now)
				set("completed_at", nil)
			}
		}

		if in.Labels.Set {
			names := in.Labels.Value
			if in.Labels.Null {
				names = nil
			}
			if err := s.setLabels(ctx, tx, cur.projectID, cur.ID, names); err != nil {
				return err
			}
			changes = append(changes, [3]string{"labels", "", strings.Join(names, ",")})
		}

		if len(sets) == 0 && len(changes) == 0 {
			return nil
		}
		set("updated_at", s.timestamp())
		args = append(args, cur.ID)
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...); err != nil {
			return err
		}
		for _, c := range changes {
			if err := s.logActivity(ctx, tx, cur.ID, actor, "updated", c[0], c[1], c[2]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Issue{}, err
	}
	s.bus.Publish(events.Event{Type: "issue.updated", ProjectKey: cur.ProjectKey, IssueKey: cur.Key, ActorID: actor.User.ID})
	return s.GetIssue(ctx, cur.Key)
}

// checkPlacement validates the issue (with a possibly new type and parent)
// against its parent and its existing children.
func (s *Service) checkPlacement(ctx context.Context, q queryer, cur Issue, typ domain.IssueType, parentID *string) error {
	var allowSkip bool
	if err := q.QueryRowContext(ctx, `SELECT allow_skip_levels FROM projects WHERE id = ?`, cur.projectID).Scan(&allowSkip); err != nil {
		return err
	}
	if parentID != nil {
		var pt string
		if err := q.QueryRowContext(ctx, `SELECT type FROM issues WHERE id = ?`, *parentID).Scan(&pt); err != nil {
			return err
		}
		ptt := domain.IssueType(pt)
		if err := domain.CanBeChild(&ptt, typ, allowSkip); err != nil {
			return err
		}
	} else if err := domain.CanBeChild(nil, typ, allowSkip); err != nil {
		return err
	}
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT type FROM issues WHERE parent_id = ? AND deleted_at IS NULL`, cur.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ct string
		if err := rows.Scan(&ct); err != nil {
			return err
		}
		if err := domain.CanBeChild(&typ, domain.IssueType(ct), allowSkip); err != nil {
			return Errorf(CodeHierarchyViolation, "existing children do not fit: %s", err.Error())
		}
	}
	return rows.Err()
}

// checkNoCycle fails if newParentID is the issue itself or one of its descendants.
func (s *Service) checkNoCycle(ctx context.Context, q queryer, issueID, newParentID string) error {
	var found bool
	err := q.QueryRowContext(ctx, `WITH RECURSIVE anc(id) AS (
			SELECT ?
			UNION ALL
			SELECT i.parent_id FROM anc CROSS JOIN issues i ON i.id = anc.id WHERE i.parent_id IS NOT NULL
		) SELECT EXISTS (SELECT 1 FROM anc WHERE id = ?)`, newParentID, issueID).Scan(&found)
	if err != nil {
		return err
	}
	if found {
		return Errorf(CodeHierarchyViolation, "an issue cannot be moved under itself or its descendants")
	}
	return nil
}

// DeleteIssue soft-deletes the issue and its descendants.
func (s *Service) DeleteIssue(ctx context.Context, key string) (int, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return 0, err
	}
	var cur Issue
	var n int64
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		cur, err = s.issueByKey(ctx, tx, key)
		if err != nil {
			return err
		}
		now := s.timestamp()
		res, err := tx.ExecContext(ctx, `WITH RECURSIVE sub(id) AS (
				SELECT ? UNION ALL SELECT i.id FROM sub CROSS JOIN issues i ON i.parent_id = sub.id WHERE i.deleted_at IS NULL
			) UPDATE issues SET deleted_at = ?, updated_at = ? WHERE id IN (SELECT id FROM sub)`, cur.ID, now, now)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return s.logActivity(ctx, tx, cur.ID, actor, "deleted", "", "", "")
	})
	if err != nil {
		return 0, err
	}
	s.bus.Publish(events.Event{Type: "issue.deleted", ProjectKey: cur.ProjectKey, IssueKey: cur.Key, ActorID: actor.User.ID})
	return int(n), nil
}

func validateDate(v *string) error {
	if v == nil || *v == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", *v); err != nil {
		return Errorf(CodeInvalidArgument, "due_date must be YYYY-MM-DD: %q", *v)
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func fmtFloat(f *float64) string {
	if f == nil {
		return ""
	}
	return strconv.FormatFloat(*f, 'f', -1, 64)
}

func fmtOptFloat(o Opt[float64]) string {
	if o.Null {
		return ""
	}
	return fmt.Sprint(o.Value)
}

// IssueKeyByJiraID resolves the numeric id used by the Jira-compatible API.
func (s *Service) IssueKeyByJiraID(ctx context.Context, jiraID int64) (string, error) {
	var key string
	err := s.db.QueryRowContext(ctx, `SELECT key FROM issues WHERE jira_id = ? AND deleted_at IS NULL`, jiraID).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", Errorf(CodeNotFound, "issue %d not found", jiraID)
	}
	return key, err
}
