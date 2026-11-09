package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/events"
)

// Acceptance inspection (docs/01-requirements.md §4.8).

// AcceptanceCriterion is one item of an issue's acceptance checklist.
type AcceptanceCriterion struct {
	ID          string `json:"id"`
	Position    int    `json:"position"`
	Description string `json:"description"`
}

// AcceptanceReviewItem is the result for one criterion in a review.
type AcceptanceReviewItem struct {
	CriterionID string `json:"criterion_id"`
	Description string `json:"description"` // the criterion text at review time
	Result      string `json:"result"`      // ok | ng | na
	Note        string `json:"note"`
}

// AcceptanceReview is one round of inspection.
type AcceptanceReview struct {
	ID        string                 `json:"id"`
	Round     int                    `json:"round"`
	Reviewer  *UserRef               `json:"reviewer"`
	Verdict   string                 `json:"verdict"` // accepted | rejected
	Summary   string                 `json:"summary"`
	Items     []AcceptanceReviewItem `json:"items"`
	CreatedAt string                 `json:"created_at"`

	issueID string
}

// AcceptanceReadiness lists what should be finished before accepting.
type AcceptanceReadiness struct {
	OpenChildren int      `json:"open_children"` // unfinished non-QA descendants
	OpenQA       int      `json:"open_qa"`       // unanswered QA among descendants
	Warnings     []string `json:"warnings"`
	// Items lists those descendants (at most 100) so a reviewer can check them.
	Items []ReadinessItem `json:"items"`
}

// ReadinessItem is an unfinished descendant or an unanswered QA.
type ReadinessItem struct {
	Key       string   `json:"key"`
	Type      string   `json:"type"`
	Title     string   `json:"title"`
	State     StateRef `json:"state"`
	ParentKey *string  `json:"parent_key"`
	Assignee  *UserRef `json:"assignee"`
}

// Acceptance is the acceptance state of one issue.
type Acceptance struct {
	IssueKey     string                `json:"issue_key"`
	InAcceptance bool                  `json:"in_acceptance"` // the issue is in the acceptance state
	Criteria     []AcceptanceCriterion `json:"criteria"`
	Reviews      []AcceptanceReview    `json:"reviews"` // oldest first
	Readiness    AcceptanceReadiness   `json:"readiness"`
}

func acceptsCriteria(t string) bool {
	return t == string(domain.TypeStory) || t == string(domain.TypeFeature)
}

// GetAcceptance returns criteria, review history and readiness.
func (s *Service) GetAcceptance(ctx context.Context, key string) (Acceptance, error) {
	is, err := s.issueByKey(ctx, s.db, key)
	if err != nil {
		return Acceptance{}, err
	}
	return s.acceptance(ctx, s.db, is)
}

func (s *Service) acceptance(ctx context.Context, q queryer, is Issue) (Acceptance, error) {
	a := Acceptance{IssueKey: is.Key, InAcceptance: is.State.IsAcceptance, Criteria: []AcceptanceCriterion{}, Reviews: []AcceptanceReview{}}
	var err error
	if a.Criteria, err = s.criteria(ctx, q, is.ID); err != nil {
		return a, err
	}
	if a.Reviews, err = s.reviews(ctx, q, []string{is.ID}); err != nil {
		return a, err
	}
	a.Readiness, err = s.readiness(ctx, q, is.ID)
	return a, err
}

func (s *Service) criteria(ctx context.Context, q queryer, issueID string) ([]AcceptanceCriterion, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, position, description FROM acceptance_criteria
		WHERE issue_id = ? AND deleted_at IS NULL ORDER BY position`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AcceptanceCriterion{}
	for rows.Next() {
		var c AcceptanceCriterion
		if err := rows.Scan(&c.ID, &c.Position, &c.Description); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// reviews returns the reviews of the given issues, oldest first.
func (s *Service) reviews(ctx context.Context, q queryer, issueIDs []string) ([]AcceptanceReview, error) {
	if len(issueIDs) == 0 {
		return []AcceptanceReview{}, nil
	}
	rows, err := q.QueryContext(ctx, `SELECT r.id, r.issue_id, r.round, r.verdict, r.summary, r.created_at,
			u.id, u.kind, u.username, u.display_name, u.avatar_color
		FROM acceptance_reviews r JOIN users u ON u.id = r.reviewer_id
		WHERE r.issue_id IN (`+placeholders(len(issueIDs))+`) ORDER BY r.round`, toAny(issueIDs)...)
	if err != nil {
		return nil, err
	}
	out := []AcceptanceReview{}
	byID := map[string]int{}
	for rows.Next() {
		r := AcceptanceReview{Reviewer: &UserRef{}, Items: []AcceptanceReviewItem{}}
		if err := rows.Scan(&r.ID, &r.issueID, &r.Round, &r.Verdict, &r.Summary, &r.CreatedAt,
			&r.Reviewer.ID, &r.Reviewer.Kind, &r.Reviewer.Username, &r.Reviewer.DisplayName, &r.Reviewer.AvatarColor); err != nil {
			rows.Close()
			return nil, err
		}
		byID[r.ID] = len(out)
		out = append(out, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(out) == 0 {
		return out, err
	}
	ids := make([]string, 0, len(out))
	for _, r := range out {
		ids = append(ids, r.ID)
	}
	items, err := q.QueryContext(ctx, `SELECT ri.review_id, ri.criterion_id, ri.description, ri.result, ri.note
		FROM acceptance_review_items ri JOIN acceptance_criteria c ON c.id = ri.criterion_id
		WHERE ri.review_id IN (`+placeholders(len(ids))+`) ORDER BY c.position`, toAny(ids)...)
	if err != nil {
		return nil, err
	}
	defer items.Close()
	for items.Next() {
		var reviewID string
		var it AcceptanceReviewItem
		if err := items.Scan(&reviewID, &it.CriterionID, &it.Description, &it.Result, &it.Note); err != nil {
			return nil, err
		}
		r := &out[byID[reviewID]]
		r.Items = append(r.Items, it)
	}
	return out, items.Err()
}

func (s *Service) readiness(ctx context.Context, q queryer, issueID string) (AcceptanceReadiness, error) {
	m, err := s.readinessMany(ctx, q, []string{issueID})
	return m[issueID], err
}

// readinessMany computes readiness for several issues with one query.
func (s *Service) readinessMany(ctx context.Context, q queryer, issueIDs []string) (map[string]AcceptanceReadiness, error) {
	out := make(map[string]AcceptanceReadiness, len(issueIDs))
	for _, id := range issueIDs {
		out[id] = AcceptanceReadiness{Warnings: []string{}, Items: []ReadinessItem{}}
	}
	if len(issueIDs) == 0 {
		return out, nil
	}
	rows, err := q.QueryContext(ctx, `WITH RECURSIVE sub(root, id, depth) AS (
			SELECT parent_id, id, 1 FROM issues WHERE parent_id IN (`+placeholders(len(issueIDs))+`) AND deleted_at IS NULL
			UNION ALL SELECT sub.root, c.id, sub.depth + 1 FROM sub CROSS JOIN issues c ON c.parent_id = sub.id WHERE c.deleted_at IS NULL)
		SELECT sub.root, i.key, i.type, i.title, st.id, st.name, st.category, st.color, st.is_acceptance, par.key,
		       a.id, a.kind, a.username, a.display_name, a.avatar_color
		FROM sub JOIN issues i ON i.id = sub.id JOIN workflow_states st ON st.id = i.state_id
		LEFT JOIN issues par ON par.id = i.parent_id LEFT JOIN users a ON a.id = i.assignee_id
		WHERE (i.type <> 'qa' AND st.category NOT IN ('completed', 'canceled'))
		   OR (i.type = 'qa' AND st.category IN ('backlog', 'unstarted'))
		ORDER BY sub.root, i.type = 'qa', sub.depth, i.number`, toAny(issueIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var root string
		var it ReadinessItem
		var parent, aID, aKind, aUser, aName, aColor sql.NullString
		if err := rows.Scan(&root, &it.Key, &it.Type, &it.Title, &it.State.ID, &it.State.Name, &it.State.Category, &it.State.Color, &it.State.IsAcceptance,
			&parent, &aID, &aKind, &aUser, &aName, &aColor); err != nil {
			return nil, err
		}
		it.ParentKey = strPtr(parent)
		if aID.Valid {
			it.Assignee = &UserRef{ID: aID.String, Kind: aKind.String, Username: aUser.String, DisplayName: aName.String, AvatarColor: aColor.String}
		}
		r := out[root]
		if it.Type == string(domain.TypeQA) {
			r.OpenQA++
		} else {
			r.OpenChildren++
		}
		if len(r.Items) < 100 {
			r.Items = append(r.Items, it)
		}
		out[root] = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for id, r := range out {
		if r.OpenChildren > 0 {
			r.Warnings = append(r.Warnings, fmt.Sprintf("未完了の子イシューが %d 件あります", r.OpenChildren))
		}
		if r.OpenQA > 0 {
			r.Warnings = append(r.Warnings, fmt.Sprintf("未回答の QA が %d 件あります", r.OpenQA))
		}
		out[id] = r
	}
	return out, nil
}

// CriterionInput is one criterion in SetCriteria. ID keeps an existing
// criterion (its history stays linked); a missing ID adds a new one.
type CriterionInput struct {
	ID          *string `json:"id"`
	Description string  `json:"description"`
}

// SetCriteria replaces an issue's acceptance checklist. Criteria left out are
// hidden, not deleted, so past reviews keep their items.
func (s *Service) SetCriteria(ctx context.Context, key string, in []CriterionInput) (Acceptance, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return Acceptance{}, err
	}
	if len(in) > 50 {
		return Acceptance{}, Errorf(CodeInvalidArgument, "at most 50 criteria")
	}
	for i := range in {
		in[i].Description = strings.TrimSpace(in[i].Description)
		if in[i].Description == "" {
			return Acceptance{}, Errorf(CodeInvalidArgument, "criterion %d: description is required", i+1)
		}
	}
	var is Issue
	var result Acceptance
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if is, err = s.issueByKey(ctx, tx, key); err != nil {
			return err
		}
		if !acceptsCriteria(is.Type) {
			return Errorf(CodeInvalidArgument, "acceptance criteria can be set on stories and features, not on a %s", is.Type)
		}
		current, err := s.criteria(ctx, tx, is.ID)
		if err != nil {
			return err
		}
		existing := map[string]AcceptanceCriterion{}
		for _, c := range current {
			existing[c.ID] = c
		}
		now := s.timestamp()
		kept := map[string]bool{}
		for i, c := range in {
			if c.ID != nil {
				if _, ok := existing[*c.ID]; !ok {
					return Errorf(CodeInvalidArgument, "criterion %q does not belong to %s", *c.ID, is.Key)
				}
				kept[*c.ID] = true
				if _, err := tx.ExecContext(ctx, `UPDATE acceptance_criteria SET position = ?, description = ? WHERE id = ?`,
					i+1, c.Description, *c.ID); err != nil {
					return err
				}
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO acceptance_criteria (id, issue_id, position, description, created_at)
				VALUES (?,?,?,?,?)`, newID(), is.ID, i+1, c.Description, now); err != nil {
				return err
			}
		}
		for id := range existing {
			if !kept[id] {
				if _, err := tx.ExecContext(ctx, `UPDATE acceptance_criteria SET deleted_at = ? WHERE id = ?`, now, id); err != nil {
					return err
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, now, is.ID); err != nil {
			return err
		}
		if err := s.logActivity(ctx, tx, is.ID, actor, "updated", "acceptance_criteria", fmt.Sprint(len(current)), fmt.Sprint(len(in))); err != nil {
			return err
		}
		result, err = s.acceptance(ctx, tx, is)
		return err
	})
	if err != nil {
		return Acceptance{}, err
	}
	s.bus.Publish(events.Event{Type: "issue.updated", ProjectKey: is.ProjectKey, IssueKey: is.Key, ActorID: actor.User.ID})
	return result, nil
}

// ReviewInput is an acceptance judgement.
type ReviewInput struct {
	Verdict string            `json:"verdict"` // accepted | rejected
	Summary string            `json:"summary"` // required when rejected (差し戻し理由)
	Items   []ReviewItemInput `json:"items"`
	// Force accepts even though children are unfinished or QA is unanswered.
	Force bool `json:"force"`
}

// ReviewItemInput is the result for one criterion.
type ReviewItemInput struct {
	CriterionID string `json:"criterion_id"`
	Result      string `json:"result"` // ok | ng | na
	Note        string `json:"note"`
}

// ReviewResult is returned after a judgement.
type ReviewResult struct {
	Review     AcceptanceReview `json:"review"`
	Issue      Issue            `json:"issue"`
	Acceptance Acceptance       `json:"acceptance"`
}

// ReviewAcceptance records a judgement and moves the issue: accepted goes to
// the first completed state, rejected back to the first started state.
// Only humans judge acceptance.
func (s *Service) ReviewAcceptance(ctx context.Context, key string, in ReviewInput) (ReviewResult, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return ReviewResult{}, err
	}
	if actor.User.Kind != "human" {
		return ReviewResult{}, Errorf(CodeForbidden, "acceptance is judged by a human; agents can request it with request_acceptance")
	}
	if in.Verdict != "accepted" && in.Verdict != "rejected" {
		return ReviewResult{}, Errorf(CodeInvalidArgument, "verdict must be accepted or rejected")
	}
	in.Summary = strings.TrimSpace(in.Summary)
	if in.Verdict == "rejected" && in.Summary == "" {
		return ReviewResult{}, Errorf(CodeInvalidArgument, "summary (差し戻し理由) is required when rejecting")
	}
	is, err := s.issueByKey(ctx, s.db, key)
	if err != nil {
		return ReviewResult{}, err
	}
	if !is.State.IsAcceptance {
		return ReviewResult{}, Errorf(CodeConflict, "%s is not waiting for acceptance (state: %s)", is.Key, is.State.Name)
	}
	criteria, err := s.criteria(ctx, s.db, is.ID)
	if err != nil {
		return ReviewResult{}, err
	}
	results := map[string]ReviewItemInput{}
	for _, it := range in.Items {
		if it.Result != "ok" && it.Result != "ng" && it.Result != "na" {
			return ReviewResult{}, Errorf(CodeInvalidArgument, "result must be ok, ng or na")
		}
		results[it.CriterionID] = it
	}
	hasNG := false
	for i, c := range criteria {
		it, ok := results[c.ID]
		if !ok {
			return ReviewResult{}, Errorf(CodeInvalidArgument, "criterion %d (%s) has no result", i+1, c.Description)
		}
		delete(results, c.ID)
		hasNG = hasNG || it.Result == "ng"
	}
	for id := range results {
		return ReviewResult{}, Errorf(CodeInvalidArgument, "criterion %q is not a current criterion of %s", id, is.Key)
	}
	if in.Verdict == "accepted" && hasNG {
		return ReviewResult{}, Errorf(CodeInvalidArgument, "cannot accept with NG criteria; reject instead")
	}
	if in.Verdict == "accepted" && !in.Force {
		ready, err := s.readiness(ctx, s.db, is.ID)
		if err != nil {
			return ReviewResult{}, err
		}
		if len(ready.Warnings) > 0 {
			return ReviewResult{}, Errorf(CodeConflict, "%s (force: true to accept anyway)", strings.Join(ready.Warnings, "、"))
		}
	}

	// Target state.
	q := `SELECT id FROM workflow_states WHERE project_id = ? AND applies_to = 'all' AND category = 'completed' ORDER BY position LIMIT 1`
	if in.Verdict == "rejected" {
		q = `SELECT id FROM workflow_states WHERE project_id = ? AND applies_to = 'all' AND category = 'started' AND is_acceptance = 0 ORDER BY position LIMIT 1`
	}
	var target int64
	if err := s.db.QueryRowContext(ctx, q, is.projectID).Scan(&target); err != nil {
		return ReviewResult{}, Errorf(CodeInvalidArgument, "project %s has no state to move the issue to", is.ProjectKey)
	}

	review := AcceptanceReview{ID: newID(), Reviewer: actor.User.Ref(), Verdict: in.Verdict, Summary: in.Summary,
		Items: []AcceptanceReviewItem{}, CreatedAt: s.timestamp()}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(round), 0) + 1 FROM acceptance_reviews WHERE issue_id = ?`, is.ID).Scan(&review.Round); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO acceptance_reviews (id, issue_id, round, reviewer_id, verdict, summary, created_at)
			VALUES (?,?,?,?,?,?,?)`, review.ID, is.ID, review.Round, actor.User.ID, review.Verdict, review.Summary, review.CreatedAt); err != nil {
			return err
		}
		for _, c := range criteria {
			it := ReviewItemInput{}
			for _, x := range in.Items {
				if x.CriterionID == c.ID {
					it = x
				}
			}
			item := AcceptanceReviewItem{CriterionID: c.ID, Description: c.Description, Result: it.Result, Note: strings.TrimSpace(it.Note)}
			if _, err := tx.ExecContext(ctx, `INSERT INTO acceptance_review_items (review_id, criterion_id, description, result, note)
				VALUES (?,?,?,?,?)`, review.ID, c.ID, item.Description, item.Result, item.Note); err != nil {
				return err
			}
			review.Items = append(review.Items, item)
		}
		return s.logActivity(ctx, tx, is.ID, actor, in.Verdict, "", "", review.Summary)
	})
	if err != nil {
		return ReviewResult{}, err
	}
	updated, err := s.UpdateIssue(ctx, is.Key, UpdateIssueInput{StateID: Some(target)})
	if err != nil {
		return ReviewResult{}, err
	}
	acc, err := s.GetAcceptance(ctx, is.Key)
	if err != nil {
		return ReviewResult{}, err
	}
	return ReviewResult{Review: review, Issue: updated, Acceptance: acc}, nil
}

// AcceptanceQueueItem is an issue waiting for acceptance.
type AcceptanceQueueItem struct {
	Issue         Issue               `json:"issue"`
	CriteriaCount int                 `json:"criteria_count"`
	Rounds        int                 `json:"rounds"` // reviews so far
	LastReview    *AcceptanceReview   `json:"last_review"`
	Readiness     AcceptanceReadiness `json:"readiness"`
}

// AcceptanceQueue lists issues in the acceptance state, oldest update first.
func (s *Service) AcceptanceQueue(ctx context.Context, projectKey string) ([]AcceptanceQueueItem, error) {
	where := `WHERE st.is_acceptance = 1 AND i.deleted_at IS NULL AND p.archived_at IS NULL`
	args := []any{}
	if projectKey != "" {
		where += ` AND p.key = ?`
		args = append(args, strings.ToUpper(projectKey))
	}
	issues, err := s.queryIssues(ctx, s.db, where+` ORDER BY i.updated_at, i.id`, args...)
	if err != nil {
		return nil, err
	}
	if err := s.enrich(ctx, issues); err != nil {
		return nil, err
	}
	ids := make([]string, len(issues))
	for i, is := range issues {
		ids[i] = is.ID
	}
	counts := map[string]int{}
	if len(ids) > 0 {
		rows, err := s.db.QueryContext(ctx, `SELECT issue_id, COUNT(*) FROM acceptance_criteria
			WHERE deleted_at IS NULL AND issue_id IN (`+placeholders(len(ids))+`) GROUP BY issue_id`, toAny(ids)...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var n int
			if err := rows.Scan(&id, &n); err != nil {
				rows.Close()
				return nil, err
			}
			counts[id] = n
		}
		rows.Close()
	}
	reviews, err := s.reviews(ctx, s.db, ids)
	if err != nil {
		return nil, err
	}
	byIssue := map[string][]AcceptanceReview{}
	for _, r := range reviews {
		byIssue[r.issueID] = append(byIssue[r.issueID], r)
	}
	ready, err := s.readinessMany(ctx, s.db, ids)
	if err != nil {
		return nil, err
	}
	out := make([]AcceptanceQueueItem, 0, len(issues))
	for _, is := range issues {
		item := AcceptanceQueueItem{Issue: is, CriteriaCount: counts[is.ID], Readiness: ready[is.ID]}
		// The list shows counts only; GET /issues/{key}/acceptance has the items.
		item.Readiness.Items = []ReadinessItem{}
		rs := byIssue[is.ID]
		item.Rounds = len(rs)
		if len(rs) > 0 {
			last := rs[len(rs)-1]
			item.LastReview = &last
		}
		out = append(out, item)
	}
	return out, nil
}
