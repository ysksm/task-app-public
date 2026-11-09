package service

import (
	"context"
	"database/sql"
	"strings"

	"github.com/ysksm/task-app/internal/events"
)

// Activity is one change-history entry.
type Activity struct {
	ID        int64    `json:"id"`
	Actor     *UserRef `json:"actor"`
	Source    string   `json:"source"`
	Action    string   `json:"action"`
	Field     string   `json:"field"`
	OldValue  string   `json:"old_value"`
	NewValue  string   `json:"new_value"`
	CreatedAt string   `json:"created_at"`
}

func (s *Service) logActivity(ctx context.Context, tx *sql.Tx, issueID string, actor Actor, action, field, oldV, newV string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO activities (issue_id, actor_id, source, action, field, old_value, new_value, created_at)
		VALUES (?,?,?,?,?,?,?,?)`, issueID, actor.User.ID, string(actor.Source), action, field, oldV, newV, s.timestamp())
	return err
}

// ListActivity returns an issue's history, oldest first.
func (s *Service) ListActivity(ctx context.Context, key string) ([]Activity, error) {
	is, err := s.issueByKey(ctx, s.db, key)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT ac.id, ac.source, ac.action, ac.field, ac.old_value, ac.new_value, ac.created_at,
			u.id, u.kind, u.username, u.display_name, u.avatar_color
		FROM activities ac JOIN users u ON u.id = ac.actor_id WHERE ac.issue_id = ? ORDER BY ac.id`, is.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Activity{}
	for rows.Next() {
		var a Activity
		u := &UserRef{}
		if err := rows.Scan(&a.ID, &a.Source, &a.Action, &a.Field, &a.OldValue, &a.NewValue, &a.CreatedAt,
			&u.ID, &u.Kind, &u.Username, &u.DisplayName, &u.AvatarColor); err != nil {
			return nil, err
		}
		a.Actor = u
		out = append(out, a)
	}
	return out, rows.Err()
}

// Comment is a discussion entry on an issue.
type Comment struct {
	ID        string   `json:"id"`
	Author    *UserRef `json:"author"`
	Body      string   `json:"body"`
	Source    string   `json:"source"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`

	JiraID int64 `json:"-"` // numeric id used by the Jira-compatible API
}

// ListComments returns an issue's comments, oldest first.
func (s *Service) ListComments(ctx context.Context, key string) ([]Comment, error) {
	is, err := s.issueByKey(ctx, s.db, key)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT c.id, c.jira_id, c.body, c.source, c.created_at, c.updated_at,
			u.id, u.kind, u.username, u.display_name, u.avatar_color
		FROM comments c JOIN users u ON u.id = c.author_id WHERE c.issue_id = ? ORDER BY c.created_at, c.id`, is.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		var c Comment
		u := &UserRef{}
		if err := rows.Scan(&c.ID, &c.JiraID, &c.Body, &c.Source, &c.CreatedAt, &c.UpdatedAt, &u.ID, &u.Kind, &u.Username, &u.DisplayName, &u.AvatarColor); err != nil {
			return nil, err
		}
		c.Author = u
		out = append(out, c)
	}
	return out, rows.Err()
}

// AddComment posts a comment as the actor.
func (s *Service) AddComment(ctx context.Context, key, body string) (Comment, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return Comment{}, err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return Comment{}, Errorf(CodeInvalidArgument, "comment body is required")
	}
	var is Issue
	c := Comment{ID: newID(), Author: actor.User.Ref(), Body: body, Source: string(actor.Source)}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		is, err = s.issueByKey(ctx, tx, key)
		if err != nil {
			return err
		}
		var jiraID int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(jira_id), 9999) + 1 FROM comments`).Scan(&jiraID); err != nil {
			return err
		}
		c.JiraID = jiraID
		c.CreatedAt = s.timestamp()
		c.UpdatedAt = c.CreatedAt
		if _, err := tx.ExecContext(ctx, `INSERT INTO comments (id, jira_id, issue_id, author_id, body, source, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?)`, c.ID, jiraID, is.ID, actor.User.ID, c.Body, c.Source, c.CreatedAt, c.UpdatedAt); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, c.CreatedAt, is.ID); err != nil {
			return err
		}
		return s.logActivity(ctx, tx, is.ID, actor, "commented", "", "", "")
	})
	if err != nil {
		return Comment{}, err
	}
	s.bus.Publish(events.Event{Type: "issue.updated", ProjectKey: is.ProjectKey, IssueKey: is.Key, ActorID: actor.User.ID})
	return c, nil
}
