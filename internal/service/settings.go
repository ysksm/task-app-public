package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/events"
)

// Project settings: workflow states and labels (PRJ-04). Only the states of
// regular issues (applies_to = all) are editable; the QA workflow is fixed
// because answering QA moves tickets between its states.

var categoryOrder = map[string]int{"backlog": 0, "unstarted": 1, "started": 2, "completed": 3, "canceled": 4}

// StateInput creates a workflow state.
type StateInput struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Color    string `json:"color"`
}

// StatePatch changes a workflow state. The category cannot change.
type StatePatch struct {
	Name         *string `json:"name"`
	Color        *string `json:"color"`
	Position     *int    `json:"position"` // 1-based position among the regular states
	IsDefault    *bool   `json:"is_default"`
	IsAcceptance *bool   `json:"is_acceptance"`
}

func (s *Service) editableStates(ctx context.Context, q queryer, projectID string) ([]State, error) {
	all, err := s.statesForProject(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	out := []State{}
	for _, st := range all {
		if st.AppliesTo == "all" {
			out = append(out, st)
		}
	}
	return out, nil
}

func (s *Service) renumber(ctx context.Context, tx *sql.Tx, states []State) error {
	for i, st := range states {
		if _, err := tx.ExecContext(ctx, `UPDATE workflow_states SET position = ? WHERE id = ?`, i+1, st.ID); err != nil {
			return err
		}
	}
	return nil
}

func uniqueName(states []State, name string, except int64) error {
	for _, st := range states {
		if st.ID != except && strings.EqualFold(st.Name, name) {
			return Errorf(CodeConflict, "a state named %q already exists", name)
		}
	}
	return nil
}

// CreateState adds a workflow state at the end of its category.
func (s *Service) CreateState(ctx context.Context, projectKey string, in StateInput) (State, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return State{}, err
	}
	p, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return State{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return State{}, Errorf(CodeInvalidArgument, "name is required")
	}
	rank, ok := categoryOrder[in.Category]
	if !ok {
		return State{}, Errorf(CodeInvalidArgument, "category must be backlog, unstarted, started, completed or canceled")
	}
	if in.Color == "" {
		in.Color = "#8A8F98"
	}
	var created State
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		states, err := s.editableStates(ctx, tx, p.ID)
		if err != nil {
			return err
		}
		if err := uniqueName(states, in.Name, 0); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO workflow_states (project_id, name, category, applies_to, color, position)
			VALUES (?,?,?,'all',?,0)`, p.ID, in.Name, in.Category, in.Color)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		created = State{ID: id, Name: in.Name, Category: in.Category, AppliesTo: "all", Color: in.Color}
		at := len(states)
		for i, st := range states {
			if categoryOrder[st.Category] > rank {
				at = i
				break
			}
		}
		ordered := append(append(append([]State{}, states[:at]...), created), states[at:]...)
		created.Position = at + 1
		return s.renumber(ctx, tx, ordered)
	})
	if err != nil {
		return State{}, err
	}
	s.bus.Publish(events.Event{Type: "project.updated", ProjectKey: p.Key, ActorID: actor.User.ID})
	return created, nil
}

func (s *Service) stateOf(ctx context.Context, q queryer, projectID string, id int64) (State, []State, error) {
	states, err := s.editableStates(ctx, q, projectID)
	if err != nil {
		return State{}, nil, err
	}
	for _, st := range states {
		if st.ID == id {
			return st, states, nil
		}
	}
	return State{}, nil, Errorf(CodeNotFound, "state %d not found among the editable states of this project", id)
}

// UpdateState renames, recolors, reorders or flags a workflow state.
func (s *Service) UpdateState(ctx context.Context, projectKey string, id int64, in StatePatch) (State, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return State{}, err
	}
	p, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return State{}, err
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		cur, states, err := s.stateOf(ctx, tx, p.ID, id)
		if err != nil {
			return err
		}
		if in.Name != nil {
			name := strings.TrimSpace(*in.Name)
			if name == "" {
				return Errorf(CodeInvalidArgument, "name is required")
			}
			if err := uniqueName(states, name, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE workflow_states SET name = ? WHERE id = ?`, name, id); err != nil {
				return err
			}
		}
		if in.Color != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE workflow_states SET color = ? WHERE id = ?`, *in.Color, id); err != nil {
				return err
			}
		}
		if in.IsDefault != nil {
			if !*in.IsDefault {
				return Errorf(CodeInvalidArgument, "choose another state as the default instead")
			}
			if cur.Category != string(domain.CategoryBacklog) && cur.Category != string(domain.CategoryUnstarted) {
				return Errorf(CodeInvalidArgument, "the default state must be a backlog or unstarted state")
			}
			if _, err := tx.ExecContext(ctx, `UPDATE workflow_states SET is_default = (id = ?) WHERE project_id = ? AND applies_to = 'all'`, id, p.ID); err != nil {
				return err
			}
		}
		if in.IsAcceptance != nil {
			if !*in.IsAcceptance {
				return Errorf(CodeInvalidArgument, "choose another state as the acceptance state instead")
			}
			if cur.Category != string(domain.CategoryStarted) {
				return Errorf(CodeInvalidArgument, "the acceptance state must be a started state")
			}
			if _, err := tx.ExecContext(ctx, `UPDATE workflow_states SET is_acceptance = (id = ?) WHERE project_id = ? AND applies_to = 'all'`, id, p.ID); err != nil {
				return err
			}
		}
		if in.Position != nil {
			// Moves within its category only, so the category order stays intact.
			var same []State
			for _, st := range states {
				if st.Category == cur.Category && st.ID != id {
					same = append(same, st)
				}
			}
			at := *in.Position - 1
			if at < 0 {
				at = 0
			}
			first := -1
			for i, st := range states {
				if st.Category == cur.Category {
					first = i
					break
				}
			}
			idx := at - first
			if idx < 0 {
				idx = 0
			}
			if idx > len(same) {
				idx = len(same)
			}
			moved := append(append(append([]State{}, same[:idx]...), cur), same[idx:]...)
			ordered := []State{}
			placed := false
			for _, st := range states {
				if st.Category == cur.Category {
					if !placed {
						ordered = append(ordered, moved...)
						placed = true
					}
					continue
				}
				ordered = append(ordered, st)
			}
			if err := s.renumber(ctx, tx, ordered); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return State{}, err
	}
	s.bus.Publish(events.Event{Type: "project.updated", ProjectKey: p.Key, ActorID: actor.User.ID})
	st, _, err := s.stateOf(ctx, s.db, p.ID, id)
	return st, err
}

// DeleteState removes an unused workflow state.
func (s *Service) DeleteState(ctx context.Context, projectKey string, id int64) error {
	actor, err := requireActor(ctx)
	if err != nil {
		return err
	}
	p, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return err
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		cur, states, err := s.stateOf(ctx, tx, p.ID, id)
		if err != nil {
			return err
		}
		var used int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE state_id = ?`, id).Scan(&used); err != nil {
			return err
		}
		if used > 0 {
			return Errorf(CodeConflict, "%d issue(s) are in %q; move them to another state first", used, cur.Name)
		}
		if cur.IsDefault {
			return Errorf(CodeConflict, "%q is the default state; choose another default first", cur.Name)
		}
		if cur.IsAcceptance {
			return Errorf(CodeConflict, "%q is the acceptance state; choose another acceptance state first", cur.Name)
		}
		// Acceptance needs a completed state and a started state to move issues to.
		if cur.Category == string(domain.CategoryCompleted) || cur.Category == string(domain.CategoryStarted) {
			others := 0
			for _, st := range states {
				if st.ID != id && st.Category == cur.Category && !st.IsAcceptance {
					others++
				}
			}
			if others == 0 {
				return Errorf(CodeConflict, "%q is the last %s state", cur.Name, cur.Category)
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_states WHERE id = ?`, id); err != nil {
			return err
		}
		rest := []State{}
		for _, st := range states {
			if st.ID != id {
				rest = append(rest, st)
			}
		}
		return s.renumber(ctx, tx, rest)
	})
	if err != nil {
		return err
	}
	s.bus.Publish(events.Event{Type: "project.updated", ProjectKey: p.Key, ActorID: actor.User.ID})
	return nil
}

// LabelPatch changes a label.
type LabelPatch struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

func (s *Service) labelOf(ctx context.Context, projectID, id string) (Label, error) {
	var l Label
	err := s.db.QueryRowContext(ctx, `SELECT id, name, color FROM labels WHERE id = ? AND project_id = ?`, id, projectID).Scan(&l.ID, &l.Name, &l.Color)
	if errors.Is(err, sql.ErrNoRows) {
		return l, Errorf(CodeNotFound, "label %s not found", id)
	}
	return l, err
}

// UpdateLabel renames or recolors a label.
func (s *Service) UpdateLabel(ctx context.Context, projectKey, id string, in LabelPatch) (Label, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return Label{}, err
	}
	p, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return Label{}, err
	}
	l, err := s.labelOf(ctx, p.ID, id)
	if err != nil {
		return Label{}, err
	}
	if in.Name != nil {
		l.Name = strings.TrimSpace(*in.Name)
		if l.Name == "" {
			return Label{}, Errorf(CodeInvalidArgument, "label name is required")
		}
	}
	if in.Color != nil {
		l.Color = *in.Color
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE labels SET name = ?, color = ? WHERE id = ?`, l.Name, l.Color, l.ID); err != nil {
		if isUniqueViolation(err) {
			return Label{}, Errorf(CodeConflict, "label %q already exists", l.Name)
		}
		return Label{}, err
	}
	s.bus.Publish(events.Event{Type: "project.updated", ProjectKey: p.Key, ActorID: actor.User.ID})
	return l, nil
}

// DeleteLabel removes a label from the project and from every issue.
func (s *Service) DeleteLabel(ctx context.Context, projectKey, id string) error {
	actor, err := requireActor(ctx)
	if err != nil {
		return err
	}
	p, err := s.GetProject(ctx, projectKey)
	if err != nil {
		return err
	}
	if _, err := s.labelOf(ctx, p.ID, id); err != nil {
		return err
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue_labels WHERE label_id = ?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM labels WHERE id = ?`, id)
		return err
	})
	if err != nil {
		return err
	}
	s.bus.Publish(events.Event{Type: "project.updated", ProjectKey: p.Key, ActorID: actor.User.ID})
	return nil
}
