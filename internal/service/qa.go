package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/events"
)

// Answer formats of a QA question (docs/01-requirements.md §4.7.1).
const (
	AnswerYesNo        = "yes_no"
	AnswerOkNg         = "ok_ng"
	AnswerSingleChoice = "single_choice"
	AnswerMultiChoice  = "multi_choice"
	AnswerText         = "text"
)

// fixedOptions are the options of the two-button formats.
var fixedOptions = map[string][]QAOption{
	AnswerYesNo: {{ID: "yes", Label: "はい"}, {ID: "no", Label: "いいえ"}},
	AnswerOkNg:  {{ID: "ok", Label: "OK"}, {ID: "ng", Label: "NG"}},
}

// QA statuses, derived from the workflow state category.
const (
	QAStatusOpen     = "open"     // waiting for an answer
	QAStatusAnswered = "answered" // every required question answered
	QAStatusClosed   = "closed"
)

// QA is a question ticket with its questions and answers.
type QA struct {
	Key                 string   `json:"key"`
	ProjectKey          string   `json:"project_key"`
	Title               string   `json:"title"`
	Status              string   `json:"status"`
	State               StateRef `json:"state"`
	TargetKey           *string  `json:"target_key"`
	Context             string   `json:"context"`
	ProvisionalDecision string   `json:"provisional_decision"`
	IsBlocking          bool     `json:"is_blocking"`
	AskedBy             *UserRef `json:"asked_by"`
	Assignee            *UserRef `json:"assignee"`
	Items               []QAItem `json:"items"`
	CreatedAt           string   `json:"created_at"`
	AnsweredAt          *string  `json:"answered_at"`
	ClosedAt            *string  `json:"closed_at"`
}

// QAItem is one question in a QA ticket.
type QAItem struct {
	ID         string     `json:"id"`
	Position   int        `json:"position"`
	Question   string     `json:"question"`
	AnswerType string     `json:"answer_type"`
	Options    []QAOption `json:"options"`
	AllowOther bool       `json:"allow_other"`
	Required   bool       `json:"required"`
	Answer     *QAAnswer  `json:"answer"`
}

// QAOption is a selectable answer.
type QAOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Recommended bool   `json:"recommended,omitempty"`
}

// QAAnswer is a human's answer to one question.
type QAAnswer struct {
	SelectedOptionIDs []string   `json:"selected_option_ids"`
	SelectedOptions   []QAOption `json:"selected_options"`
	Text              *string    `json:"text"` // text answer, or the "other" free text
	Note              *string    `json:"note"`
	AnsweredBy        *UserRef   `json:"answered_by"`
	AnsweredAt        string     `json:"answered_at"`
}

// ---- create ----

// CreateQAInput asks one or more questions.
type CreateQAInput struct {
	TargetKey           *string       `json:"target_key"`  // issue the questions are about (becomes the parent)
	ProjectKey          string        `json:"project_key"` // required without target_key
	Title               string        `json:"title"`       // defaults to the first question
	Context             string        `json:"context"`
	ProvisionalDecision string        `json:"provisional_decision"`
	IsBlocking          bool          `json:"is_blocking"`
	Assignee            *string       `json:"assignee"` // defaults to the project owner
	Priority            *int          `json:"priority"`
	Items               []QAItemInput `json:"items"`
}

// QAItemInput is one question to ask.
type QAItemInput struct {
	Question   string          `json:"question"`
	AnswerType string          `json:"answer_type"`
	Options    []QAOptionInput `json:"options"`
	AllowOther bool            `json:"allow_other"`
	Required   *bool           `json:"required"`
}

// QAOptionInput describes a choice. For yes_no/ok_ng only id+recommended are used.
type QAOptionInput struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Recommended bool   `json:"recommended"`
}

const maxQAItems = 20

// CreateQA creates a QA ticket (an issue of type qa) with its questions.
func (s *Service) CreateQA(ctx context.Context, in CreateQAInput) (QA, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return QA{}, err
	}
	if len(in.Items) == 0 || len(in.Items) > maxQAItems {
		return QA{}, Errorf(CodeInvalidArgument, "a QA needs 1-%d questions", maxQAItems)
	}
	items := make([]QAItem, len(in.Items))
	for i, it := range in.Items {
		item, err := buildQAItem(i, it)
		if err != nil {
			return QA{}, err
		}
		items[i] = item
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = truncate(items[0].Question, 120)
		if len(items) > 1 {
			title = fmt.Sprintf("%s（他%d件）", truncate(items[0].Question, 100), len(items)-1)
		}
	}

	var created createdIssue
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		projectKey := in.ProjectKey
		var targetID any
		if in.TargetKey != nil && *in.TargetKey != "" {
			t, err := s.issueByKey(ctx, tx, *in.TargetKey)
			if err != nil {
				return Errorf(CodeInvalidArgument, "target %q not found", *in.TargetKey)
			}
			targetID, projectKey = t.ID, t.ProjectKey
		}
		assignee := in.Assignee
		if assignee == nil {
			var owner string
			err := tx.QueryRowContext(ctx, `SELECT u.username FROM projects p JOIN users u ON u.id = COALESCE(p.default_qa_assignee_id, p.owner_id)
				WHERE p.key = ?`, strings.ToUpper(projectKey)).Scan(&owner)
			if err == nil {
				assignee = &owner
			}
		}
		created, err = s.createIssueTx(ctx, tx, actor, CreateIssueInput{
			ProjectKey: projectKey, Type: string(domain.TypeQA), Title: title, ParentKey: in.TargetKey,
			Description: qaDescription(in.Context, in.ProvisionalDecision, items), Assignee: assignee, Priority: in.Priority,
		})
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO qa_details (issue_id, target_issue_id, context, provisional_decision, is_blocking)
			VALUES (?,?,?,?,?)`, created.id, targetID, in.Context, in.ProvisionalDecision, in.IsBlocking); err != nil {
			return err
		}
		for _, it := range items {
			opts, _ := json.Marshal(it.Options)
			if _, err := tx.ExecContext(ctx, `INSERT INTO qa_items (id, issue_id, position, question, answer_type, options, allow_other, required)
				VALUES (?,?,?,?,?,?,?,?)`, newID(), created.id, it.Position, it.Question, it.AnswerType, string(opts), it.AllowOther, it.Required); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return QA{}, err
	}
	s.bus.Publish(events.Event{Type: "issue.created", ProjectKey: created.projectKey, IssueKey: created.key, ActorID: actor.User.ID})
	return s.GetQA(ctx, created.key)
}

func buildQAItem(i int, in QAItemInput) (QAItem, error) {
	item := QAItem{Position: i, Question: strings.TrimSpace(in.Question), AnswerType: in.AnswerType, Required: true}
	if in.Required != nil {
		item.Required = *in.Required
	}
	where := fmt.Sprintf("question %d", i+1)
	if item.Question == "" {
		return item, Errorf(CodeInvalidArgument, "%s: question text is required", where)
	}
	switch in.AnswerType {
	case AnswerYesNo, AnswerOkNg:
		recommended := map[string]bool{}
		for _, o := range in.Options {
			recommended[o.ID] = o.Recommended
		}
		for _, o := range fixedOptions[in.AnswerType] {
			o.Recommended = recommended[o.ID]
			item.Options = append(item.Options, o)
		}
	case AnswerSingleChoice, AnswerMultiChoice:
		if len(in.Options) < 2 || len(in.Options) > 6 {
			return item, Errorf(CodeInvalidArgument, "%s: %s needs 2-6 options", where, in.AnswerType)
		}
		seen := map[string]bool{}
		for j, o := range in.Options {
			id := strings.TrimSpace(o.ID)
			if id == "" {
				id = string(rune('a' + j))
			}
			label := strings.TrimSpace(o.Label)
			if label == "" {
				return item, Errorf(CodeInvalidArgument, "%s: option %d needs a label", where, j+1)
			}
			if seen[id] {
				return item, Errorf(CodeInvalidArgument, "%s: duplicate option id %q", where, id)
			}
			seen[id] = true
			item.Options = append(item.Options, QAOption{ID: id, Label: label, Description: o.Description, Recommended: o.Recommended})
		}
		item.AllowOther = in.AllowOther
	case AnswerText:
		if len(in.Options) > 0 {
			return item, Errorf(CodeInvalidArgument, "%s: text questions take no options", where)
		}
	default:
		return item, Errorf(CodeInvalidArgument, "%s: answer_type must be one of yes_no, ok_ng, single_choice, multi_choice, text", where)
	}
	if item.Options == nil {
		item.Options = []QAOption{}
	}
	return item, nil
}

// qaDescription renders the questions as Markdown so that the issue reads
// well in places that only show the description (lists, Jira clients).
func qaDescription(context, provisional string, items []QAItem) string {
	var b strings.Builder
	if context != "" {
		b.WriteString(context + "\n\n")
	}
	for i, it := range items {
		fmt.Fprintf(&b, "%d. %s\n", i+1, it.Question)
		for _, o := range it.Options {
			mark := ""
			if o.Recommended {
				mark = "（推奨）"
			}
			fmt.Fprintf(&b, "   - %s%s\n", o.Label, mark)
		}
	}
	if provisional != "" {
		b.WriteString("\n暫定判断: " + provisional + "\n")
	}
	return strings.TrimSpace(b.String())
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

// ---- read ----

// GetQA returns a QA ticket with its questions and answers.
func (s *Service) GetQA(ctx context.Context, key string) (QA, error) {
	list, err := s.loadQAs(ctx, []string{strings.ToUpper(key)})
	if err != nil {
		return QA{}, err
	}
	if len(list) == 0 {
		return QA{}, Errorf(CodeNotFound, "QA %q not found", key)
	}
	return list[0], nil
}

// ListQAOptions filters ListQA.
type ListQAOptions struct {
	Status     string // open | answered | closed | "" (all)
	AskedBy    string // username or "me"
	Assignee   string // username or "me"
	TargetKey  string
	ProjectKey string
	Limit      int
}

// ListQA returns QA tickets, newest first.
func (s *Service) ListQA(ctx context.Context, opt ListQAOptions) ([]QA, error) {
	and := []domain.Filter{{Field: domain.FieldType, Op: domain.OpEq, Value: string(domain.TypeQA)}}
	switch opt.Status {
	case "":
	case QAStatusOpen:
		and = append(and, domain.Filter{Field: domain.FieldStateCategory, Op: domain.OpIn, Value: []any{"backlog", "unstarted"}})
	case QAStatusAnswered:
		and = append(and, domain.Filter{Field: domain.FieldStateCategory, Op: domain.OpEq, Value: "started"})
	case QAStatusClosed:
		and = append(and, domain.Filter{Field: domain.FieldStateCategory, Op: domain.OpIn, Value: []any{"completed", "canceled"}})
	default:
		return nil, Errorf(CodeInvalidArgument, "status must be open, answered or closed")
	}
	if opt.AskedBy != "" {
		and = append(and, domain.Filter{Field: domain.FieldReporter, Op: domain.OpEq, Value: opt.AskedBy})
	}
	if opt.Assignee != "" {
		and = append(and, domain.Filter{Field: domain.FieldAssignee, Op: domain.OpEq, Value: opt.Assignee})
	}
	if opt.TargetKey != "" {
		and = append(and, domain.Filter{Field: domain.FieldParent, Op: domain.OpEq, Value: opt.TargetKey})
	}
	if opt.ProjectKey != "" {
		and = append(and, domain.Filter{Field: domain.FieldProject, Op: domain.OpEq, Value: opt.ProjectKey})
	}
	res, err := s.ListIssues(ctx, ListOptions{Filter: domain.Filter{And: and}, Sort: "created", Limit: opt.Limit})
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(res.Issues))
	for i, is := range res.Issues {
		keys[i] = is.Key
	}
	return s.loadQAs(ctx, keys)
}

// loadQAs loads QA tickets by key, keeping the given order.
func (s *Service) loadQAs(ctx context.Context, keys []string) ([]QA, error) {
	out := []QA{}
	if len(keys) == 0 {
		return out, nil
	}
	byKey := map[string]*QA{}
	ids := map[string]*QA{}
	for start := 0; start < len(keys); start += 500 {
		end := min(start+500, len(keys))
		issues, err := s.queryIssues(ctx, s.db, `WHERE i.key IN (`+placeholders(end-start)+`) AND i.type = 'qa' AND i.deleted_at IS NULL`, toAny(keys[start:end])...)
		if err != nil {
			return nil, err
		}
		for _, is := range issues {
			q := &QA{Key: is.Key, ProjectKey: is.ProjectKey, Title: is.Title, State: is.State, Status: qaStatus(is.State.Category),
				TargetKey: is.ParentKey, AskedBy: is.Reporter, Assignee: is.Assignee, Items: []QAItem{}, CreatedAt: is.CreatedAt}
			byKey[is.Key] = q
			ids[is.ID] = q
		}
	}
	idList := make([]string, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	for start := 0; start < len(idList); start += 500 {
		end := min(start+500, len(idList))
		chunk, ph := toAny(idList[start:end]), placeholders(end-start)
		rows, err := s.db.QueryContext(ctx, `SELECT issue_id, context, provisional_decision, is_blocking, answered_at, closed_at
			FROM qa_details WHERE issue_id IN (`+ph+`)`, chunk...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var answered, closed sql.NullString
			var d QA
			if err := rows.Scan(&id, &d.Context, &d.ProvisionalDecision, &d.IsBlocking, &answered, &closed); err != nil {
				rows.Close()
				return nil, err
			}
			q := ids[id]
			q.Context, q.ProvisionalDecision, q.IsBlocking = d.Context, d.ProvisionalDecision, d.IsBlocking
			q.AnsweredAt, q.ClosedAt = strPtr(answered), strPtr(closed)
		}
		rows.Close()

		rows, err = s.db.QueryContext(ctx, `SELECT q.issue_id, q.id, q.position, q.question, q.answer_type, q.options, q.allow_other, q.required,
				q.selected_option_ids, q.answer_text, q.answer_note, q.answered_at,
				u.id, u.kind, u.username, u.display_name, u.avatar_color
			FROM qa_items q LEFT JOIN users u ON u.id = q.answered_by
			WHERE q.issue_id IN (`+ph+`) ORDER BY q.issue_id, q.position`, chunk...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var issueID, opts string
			var it QAItem
			var sel, text, note, at, uID, uKind, uName, uDisplay, uColor sql.NullString
			if err := rows.Scan(&issueID, &it.ID, &it.Position, &it.Question, &it.AnswerType, &opts, &it.AllowOther, &it.Required,
				&sel, &text, &note, &at, &uID, &uKind, &uName, &uDisplay, &uColor); err != nil {
				rows.Close()
				return nil, err
			}
			_ = json.Unmarshal([]byte(opts), &it.Options)
			if it.Options == nil {
				it.Options = []QAOption{}
			}
			if at.Valid {
				a := &QAAnswer{SelectedOptionIDs: []string{}, SelectedOptions: []QAOption{}, Text: strPtr(text), Note: strPtr(note), AnsweredAt: at.String}
				if sel.Valid {
					_ = json.Unmarshal([]byte(sel.String), &a.SelectedOptionIDs)
				}
				for _, id := range a.SelectedOptionIDs {
					for _, o := range it.Options {
						if o.ID == id {
							a.SelectedOptions = append(a.SelectedOptions, o)
						}
					}
				}
				if uID.Valid {
					a.AnsweredBy = &UserRef{ID: uID.String, Kind: uKind.String, Username: uName.String, DisplayName: uDisplay.String, AvatarColor: uColor.String}
				}
				it.Answer = a
			}
			ids[issueID].Items = append(ids[issueID].Items, it)
		}
		rows.Close()
	}
	for _, k := range keys {
		if q, ok := byKey[strings.ToUpper(k)]; ok {
			out = append(out, *q)
		}
	}
	return out, nil
}

func qaStatus(category string) string {
	switch domain.StateCategory(category) {
	case domain.CategoryStarted:
		return QAStatusAnswered
	case domain.CategoryCompleted, domain.CategoryCanceled:
		return QAStatusClosed
	}
	return QAStatusOpen
}

// ---- answer / close ----

// AnswerInput answers some or all questions of a QA ticket.
type AnswerInput struct {
	Answers []ItemAnswerInput `json:"answers"`
}

// ItemAnswerInput is the answer to one question.
type ItemAnswerInput struct {
	ItemID            string   `json:"item_id"`
	SelectedOptionIDs []string `json:"selected_option_ids"`
	Text              *string  `json:"text"` // text answer, or "other" for choices with allow_other
	Note              *string  `json:"note"`
}

// AnswerQA records answers. Questions may be answered in several steps and
// re-answered until the ticket is closed. When every required question has
// an answer the ticket moves to the "answered" state.
func (s *Service) AnswerQA(ctx context.Context, key string, in AnswerInput) (QA, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return QA{}, err
	}
	if len(in.Answers) == 0 {
		return QA{}, Errorf(CodeInvalidArgument, "answers are required")
	}
	q, err := s.GetQA(ctx, key)
	if err != nil {
		return QA{}, err
	}
	if q.Status == QAStatusClosed {
		return QA{}, Errorf(CodeConflict, "%s is closed", q.Key)
	}
	items := map[string]QAItem{}
	for _, it := range q.Items {
		items[it.ID] = it
	}
	type row struct {
		id         string
		sel        []string
		text, note *string
	}
	rows := []row{}
	seen := map[string]bool{}
	for _, a := range in.Answers {
		it, ok := items[a.ItemID]
		if !ok {
			return QA{}, Errorf(CodeInvalidArgument, "unknown item_id %q", a.ItemID)
		}
		if seen[a.ItemID] {
			return QA{}, Errorf(CodeInvalidArgument, "item %q answered twice", a.ItemID)
		}
		seen[a.ItemID] = true
		sel, text, err := validateAnswer(it, a)
		if err != nil {
			return QA{}, err
		}
		rows = append(rows, row{it.ID, sel, text, trimPtr(a.Note)})
	}

	var issueID string
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `SELECT id FROM issues WHERE key = ?`, q.Key).Scan(&issueID); err != nil {
			return err
		}
		now := s.timestamp()
		for _, r := range rows {
			selJSON, _ := json.Marshal(r.sel)
			if _, err := tx.ExecContext(ctx, `UPDATE qa_items SET selected_option_ids = ?, answer_text = ?, answer_note = ?, answered_by = ?, answered_at = ?
				WHERE id = ?`, string(selJSON), nullString(r.text), nullString(r.note), actor.User.ID, now, r.id); err != nil {
				return err
			}
		}
		var missing int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM qa_items WHERE issue_id = ? AND required = 1 AND answered_at IS NULL`, issueID).Scan(&missing); err != nil {
			return err
		}
		if err := s.logActivity(ctx, tx, issueID, actor, "answered", "", "", fmt.Sprintf("%d", len(rows))); err != nil {
			return err
		}
		if missing == 0 && q.Status == QAStatusOpen {
			if _, err := tx.ExecContext(ctx, `UPDATE qa_details SET answered_at = ? WHERE issue_id = ?`, now, issueID); err != nil {
				return err
			}
			return s.moveQAState(ctx, tx, issueID, "started", actor)
		}
		_, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, now, issueID)
		return err
	})
	if err != nil {
		return QA{}, err
	}
	s.bus.Publish(events.Event{Type: "issue.updated", ProjectKey: q.ProjectKey, IssueKey: q.Key, ActorID: actor.User.ID})
	return s.GetQA(ctx, q.Key)
}

func validateAnswer(it QAItem, a ItemAnswerInput) ([]string, *string, error) {
	where := fmt.Sprintf("question %d", it.Position+1)
	text := trimPtr(a.Text)
	valid := map[string]bool{}
	for _, o := range it.Options {
		valid[o.ID] = true
	}
	sel := []string{}
	dup := map[string]bool{}
	for _, id := range a.SelectedOptionIDs {
		if !valid[id] {
			return nil, nil, Errorf(CodeInvalidArgument, "%s: unknown option %q", where, id)
		}
		if !dup[id] {
			sel = append(sel, id)
			dup[id] = true
		}
	}
	switch it.AnswerType {
	case AnswerYesNo, AnswerOkNg:
		if len(sel) != 1 {
			return nil, nil, Errorf(CodeInvalidArgument, "%s: choose exactly one of %s", where, optionIDs(it.Options))
		}
		if text != nil {
			return nil, nil, Errorf(CodeInvalidArgument, "%s: use note for comments on a %s question", where, it.AnswerType)
		}
	case AnswerSingleChoice:
		if text != nil && !it.AllowOther {
			return nil, nil, Errorf(CodeInvalidArgument, "%s: free text is not allowed for this question", where)
		}
		if len(sel)+boolInt(text != nil) != 1 {
			return nil, nil, Errorf(CodeInvalidArgument, "%s: choose exactly one option%s", where, otherHint(it))
		}
	case AnswerMultiChoice:
		if text != nil && !it.AllowOther {
			return nil, nil, Errorf(CodeInvalidArgument, "%s: free text is not allowed for this question", where)
		}
		if len(sel) == 0 && text == nil {
			return nil, nil, Errorf(CodeInvalidArgument, "%s: choose at least one option%s", where, otherHint(it))
		}
	case AnswerText:
		if len(sel) > 0 {
			return nil, nil, Errorf(CodeInvalidArgument, "%s: text questions take no options", where)
		}
		if text == nil {
			return nil, nil, Errorf(CodeInvalidArgument, "%s: text is required", where)
		}
	}
	return sel, text, nil
}

func optionIDs(opts []QAOption) string {
	ids := make([]string, len(opts))
	for i, o := range opts {
		ids[i] = o.ID
	}
	return strings.Join(ids, "/")
}

func otherHint(it QAItem) string {
	if it.AllowOther {
		return " (or write text as \"other\")"
	}
	return ""
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// CloseQA closes a QA ticket (typically after the agent acted on the answers).
func (s *Service) CloseQA(ctx context.Context, key string) (QA, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return QA{}, err
	}
	q, err := s.GetQA(ctx, key)
	if err != nil {
		return QA{}, err
	}
	if q.Status == QAStatusClosed {
		return q, nil
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var issueID string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM issues WHERE key = ?`, q.Key).Scan(&issueID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE qa_details SET closed_at = ? WHERE issue_id = ?`, s.timestamp(), issueID); err != nil {
			return err
		}
		return s.moveQAState(ctx, tx, issueID, "completed", actor)
	})
	if err != nil {
		return QA{}, err
	}
	s.bus.Publish(events.Event{Type: "issue.updated", ProjectKey: q.ProjectKey, IssueKey: q.Key, ActorID: actor.User.ID})
	return s.GetQA(ctx, q.Key)
}

// moveQAState sets a QA issue to the first QA state of the given category.
func (s *Service) moveQAState(ctx context.Context, tx *sql.Tx, issueID, category string, actor Actor) error {
	var stateID int64
	var oldName, newName string
	err := tx.QueryRowContext(ctx, `SELECT ws.id, ws.name, cur.name FROM issues i
		JOIN workflow_states cur ON cur.id = i.state_id
		JOIN workflow_states ws ON ws.project_id = i.project_id AND ws.applies_to = 'qa' AND ws.category = ?
		WHERE i.id = ? ORDER BY ws.position LIMIT 1`, category, issueID).Scan(&stateID, &newName, &oldName)
	if errors.Is(err, sql.ErrNoRows) {
		return Errorf(CodeInternal, "project has no QA state in category %s", category)
	}
	if err != nil {
		return err
	}
	now := s.timestamp()
	sets := "state_id = ?, updated_at = ?"
	args := []any{stateID, now}
	switch category {
	case "started":
		sets += ", started_at = COALESCE(started_at, ?), completed_at = NULL"
		args = append(args, now)
	case "completed":
		sets += ", completed_at = ?"
		args = append(args, now)
	}
	args = append(args, issueID)
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET `+sets+` WHERE id = ?`, args...); err != nil {
		return err
	}
	return s.logActivity(ctx, tx, issueID, actor, "updated", "state", oldName, newName)
}
