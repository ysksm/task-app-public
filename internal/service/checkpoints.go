package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"

	"github.com/ysksm/task-app/internal/events"
)

// Checkpoint is an agent's progress record with the reasoning behind it.
type Checkpoint struct {
	ID           string         `json:"id"`
	IssueKey     string         `json:"issue_key"`
	Seq          int            `json:"seq"`
	Author       *UserRef       `json:"author"`
	Summary      string         `json:"summary"`
	Decision     string         `json:"decision"`
	Rationale    string         `json:"rationale"`
	Alternatives []string       `json:"alternatives"`
	Confidence   string         `json:"confidence"` // high | medium | low
	Progress     *int           `json:"progress"`
	NextSteps    string         `json:"next_steps"`
	Artifacts    map[string]any `json:"artifacts"`
	Source       string         `json:"source"`
	CreatedAt    string         `json:"created_at"`
}

// CheckpointInput records a checkpoint.
type CheckpointInput struct {
	Summary      string         `json:"summary"`
	Decision     string         `json:"decision"`
	Rationale    string         `json:"rationale"`
	Alternatives []string       `json:"alternatives"`
	Confidence   string         `json:"confidence"`
	Progress     *int           `json:"progress"`
	NextSteps    string         `json:"next_steps"`
	Artifacts    map[string]any `json:"artifacts"` // e.g. {"branch": "...", "commits": [...], "pr": "...", "files": [...]}
}

// RecordCheckpoint appends a checkpoint to an issue.
func (s *Service) RecordCheckpoint(ctx context.Context, key string, in CheckpointInput) (Checkpoint, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return Checkpoint{}, err
	}
	in.Summary, in.Rationale = strings.TrimSpace(in.Summary), strings.TrimSpace(in.Rationale)
	if in.Summary == "" {
		return Checkpoint{}, Errorf(CodeInvalidArgument, "summary is required")
	}
	if in.Rationale == "" {
		return Checkpoint{}, Errorf(CodeInvalidArgument, "rationale (判断基準) is required")
	}
	if in.Confidence == "" {
		in.Confidence = "medium"
	}
	if in.Confidence != "high" && in.Confidence != "medium" && in.Confidence != "low" {
		return Checkpoint{}, Errorf(CodeInvalidArgument, "confidence must be high, medium or low")
	}
	if in.Progress != nil && (*in.Progress < 0 || *in.Progress > 100) {
		return Checkpoint{}, Errorf(CodeInvalidArgument, "progress must be 0-100")
	}
	if in.Alternatives == nil {
		in.Alternatives = []string{}
	}
	if in.Artifacts == nil {
		in.Artifacts = map[string]any{}
	}
	alts, _ := json.Marshal(in.Alternatives)
	arts, _ := json.Marshal(in.Artifacts)
	cp := Checkpoint{ID: newID(), Author: actor.User.Ref(), Summary: in.Summary, Decision: strings.TrimSpace(in.Decision), Rationale: in.Rationale,
		Alternatives: in.Alternatives, Confidence: in.Confidence, Progress: in.Progress, NextSteps: strings.TrimSpace(in.NextSteps),
		Artifacts: in.Artifacts, Source: string(actor.Source), CreatedAt: s.timestamp()}
	var is Issue
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		is, err = s.issueByKey(ctx, tx, key)
		if err != nil {
			return err
		}
		cp.IssueKey = is.Key
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM checkpoints WHERE issue_id = ?`, is.ID).Scan(&cp.Seq); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO checkpoints (id, issue_id, author_id, seq, summary, decision, rationale, alternatives,
				confidence, progress, next_steps, artifacts, source, created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			cp.ID, is.ID, actor.User.ID, cp.Seq, cp.Summary, cp.Decision, cp.Rationale, string(alts), cp.Confidence, cp.Progress,
			cp.NextSteps, string(arts), cp.Source, cp.CreatedAt); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, cp.CreatedAt, is.ID); err != nil {
			return err
		}
		return s.logActivity(ctx, tx, is.ID, actor, "checkpoint", "", "", cp.Summary)
	})
	if err != nil {
		return Checkpoint{}, err
	}
	s.bus.Publish(events.Event{Type: "issue.updated", ProjectKey: is.ProjectKey, IssueKey: is.Key, ActorID: actor.User.ID})
	return cp, nil
}

const checkpointSelect = `SELECT c.id, i.key, c.seq, c.summary, c.decision, c.rationale, c.alternatives, c.confidence, c.progress,
	c.next_steps, c.artifacts, c.source, c.created_at, u.id, u.kind, u.username, u.display_name, u.avatar_color
	FROM checkpoints c JOIN issues i ON i.id = c.issue_id JOIN users u ON u.id = c.author_id`

func scanCheckpoints(rows *sql.Rows) ([]Checkpoint, error) {
	defer rows.Close()
	out := []Checkpoint{}
	for rows.Next() {
		var c Checkpoint
		var alts, arts string
		var progress sql.NullInt64
		u := &UserRef{}
		if err := rows.Scan(&c.ID, &c.IssueKey, &c.Seq, &c.Summary, &c.Decision, &c.Rationale, &alts, &c.Confidence, &progress,
			&c.NextSteps, &arts, &c.Source, &c.CreatedAt, &u.ID, &u.Kind, &u.Username, &u.DisplayName, &u.AvatarColor); err != nil {
			return nil, err
		}
		if progress.Valid {
			p := int(progress.Int64)
			c.Progress = &p
		}
		_ = json.Unmarshal([]byte(alts), &c.Alternatives)
		_ = json.Unmarshal([]byte(arts), &c.Artifacts)
		if c.Alternatives == nil {
			c.Alternatives = []string{}
		}
		if c.Artifacts == nil {
			c.Artifacts = map[string]any{}
		}
		c.Author = u
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListCheckpoints returns an issue's checkpoints, oldest first.
func (s *Service) ListCheckpoints(ctx context.Context, key string) ([]Checkpoint, error) {
	is, err := s.issueByKey(ctx, s.db, key)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, checkpointSelect+` WHERE c.issue_id = ? ORDER BY c.seq`, is.ID)
	if err != nil {
		return nil, err
	}
	return scanCheckpoints(rows)
}

// ---- agent activity feed ----

// FeedEntry is one item of the agent activity feed.
type FeedEntry struct {
	Kind       string      `json:"kind"` // checkpoint | question | state
	At         string      `json:"at"`
	Actor      *UserRef    `json:"actor"`
	Issue      IssueRef    `json:"issue"`
	ProjectKey string      `json:"project_key"`
	Checkpoint *Checkpoint `json:"checkpoint,omitempty"`
	Question   *FeedQA     `json:"question,omitempty"`
	Change     *FeedChange `json:"change,omitempty"`
}

// FeedQA summarizes a question in the feed.
type FeedQA struct {
	Status     string `json:"status"`
	IsBlocking bool   `json:"is_blocking"`
	ItemCount  int    `json:"item_count"`
	TargetKey  string `json:"target_key"`
}

// FeedChange is a state change.
type FeedChange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// FeedOptions filters AgentActivity.
type FeedOptions struct {
	Agent         string // username
	ProjectKey    string
	LowConfidence bool // only low-confidence checkpoints
	Limit         int
}

// AgentActivity lists recent work by agents: checkpoints, questions and state changes.
func (s *Service) AgentActivity(ctx context.Context, opt FeedOptions) ([]FeedEntry, error) {
	if _, err := requireActor(ctx); err != nil {
		return nil, err
	}
	if opt.Limit <= 0 || opt.Limit > 500 {
		opt.Limit = 100
	}
	where, args := []string{"u.kind = 'agent'", "i.deleted_at IS NULL"}, []any{}
	if opt.Agent != "" {
		where, args = append(where, "u.username = ?"), append(args, opt.Agent)
	}
	if opt.ProjectKey != "" {
		where, args = append(where, "i.project_id = (SELECT id FROM projects WHERE key = ?)"), append(args, strings.ToUpper(opt.ProjectKey))
	}
	cond := strings.Join(where, " AND ")
	out := []FeedEntry{}

	cpWhere := cond
	if opt.LowConfidence {
		cpWhere += " AND c.confidence = 'low'"
	}
	rows, err := s.db.QueryContext(ctx, checkpointSelect+` WHERE `+cpWhere+` ORDER BY c.created_at DESC LIMIT ?`, append(args, opt.Limit)...)
	if err != nil {
		return nil, err
	}
	cps, err := scanCheckpoints(rows)
	if err != nil {
		return nil, err
	}
	refs, err := s.issueRefs(ctx, cps)
	if err != nil {
		return nil, err
	}
	for i := range cps {
		c := cps[i]
		out = append(out, FeedEntry{Kind: "checkpoint", At: c.CreatedAt, Actor: c.Author, Issue: refs[c.IssueKey].ref,
			ProjectKey: refs[c.IssueKey].project, Checkpoint: &c})
	}
	if opt.LowConfidence {
		return out, nil
	}

	rows, err = s.db.QueryContext(ctx, `SELECT i.key, i.type, i.title, p.key, i.created_at, st.category, COALESCE(q.is_blocking, 0),
			(SELECT COUNT(*) FROM qa_items qi WHERE qi.issue_id = i.id), COALESCE(par.key, ''),
			u.id, u.kind, u.username, u.display_name, u.avatar_color
		FROM issues i JOIN projects p ON p.id = i.project_id JOIN users u ON u.id = i.reporter_id
		JOIN workflow_states st ON st.id = i.state_id LEFT JOIN qa_details q ON q.issue_id = i.id LEFT JOIN issues par ON par.id = i.parent_id
		WHERE i.type = 'qa' AND `+cond+` ORDER BY i.created_at DESC LIMIT ?`, append(args, opt.Limit)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var e FeedEntry
		var cat string
		q := &FeedQA{}
		u := &UserRef{}
		if err := rows.Scan(&e.Issue.Key, &e.Issue.Type, &e.Issue.Title, &e.ProjectKey, &e.At, &cat, &q.IsBlocking, &q.ItemCount, &q.TargetKey,
			&u.ID, &u.Kind, &u.Username, &u.DisplayName, &u.AvatarColor); err != nil {
			rows.Close()
			return nil, err
		}
		q.Status = qaStatus(cat)
		e.Kind, e.Actor, e.Question = "question", u, q
		out = append(out, e)
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx, `SELECT i.key, i.type, i.title, p.key, ac.created_at, ac.old_value, ac.new_value,
			u.id, u.kind, u.username, u.display_name, u.avatar_color
		FROM activities ac JOIN issues i ON i.id = ac.issue_id JOIN projects p ON p.id = i.project_id JOIN users u ON u.id = ac.actor_id
		WHERE ac.field = 'state' AND `+cond+` ORDER BY ac.id DESC LIMIT ?`, append(args, opt.Limit)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var e FeedEntry
		ch := &FeedChange{}
		u := &UserRef{}
		if err := rows.Scan(&e.Issue.Key, &e.Issue.Type, &e.Issue.Title, &e.ProjectKey, &e.At, &ch.From, &ch.To,
			&u.ID, &u.Kind, &u.Username, &u.DisplayName, &u.AvatarColor); err != nil {
			rows.Close()
			return nil, err
		}
		e.Kind, e.Actor, e.Change = "state", u, ch
		out = append(out, e)
	}
	rows.Close()

	sort.SliceStable(out, func(i, j int) bool { return out[i].At > out[j].At })
	if len(out) > opt.Limit {
		out = out[:opt.Limit]
	}
	return out, nil
}

type refWithProject struct {
	ref     IssueRef
	project string
}

func (s *Service) issueRefs(ctx context.Context, cps []Checkpoint) (map[string]refWithProject, error) {
	out := map[string]refWithProject{}
	keys := []string{}
	for _, c := range cps {
		if _, ok := out[c.IssueKey]; !ok {
			out[c.IssueKey] = refWithProject{}
			keys = append(keys, c.IssueKey)
		}
	}
	for start := 0; start < len(keys); start += 500 {
		end := min(start+500, len(keys))
		rows, err := s.db.QueryContext(ctx, `SELECT i.key, i.type, i.title, p.key FROM issues i JOIN projects p ON p.id = i.project_id
			WHERE i.key IN (`+placeholders(end-start)+`)`, toAny(keys[start:end])...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var r refWithProject
			if err := rows.Scan(&r.ref.Key, &r.ref.Type, &r.ref.Title, &r.project); err != nil {
				rows.Close()
				return nil, err
			}
			out[r.ref.Key] = r
		}
		rows.Close()
	}
	return out, nil
}
