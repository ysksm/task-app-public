package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ysksm/task-app/internal/events"
)

// Requirements layer (docs/08-devcycle.md §5, M1): personas, jobs, the story
// map (activities → steps), stories and acceptance criteria, all owned by a
// project. They are living specifications: archived instead of closed, keys
// (TAPP-P1, TAPP-J3, TAPP-A1, TAPP-A1-2, TAPP-ST12, TAPP-AC42) are never
// reused, and every change is written to req_changes.

// ---- types ----

// Persona is who uses the product (L1).
type Persona struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Kind        string `json:"kind"` // human | agent
	Description string `json:"description"`
	Position    int    `json:"position"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Job is a job to be done (L2): in this situation, I want this, so that.
type Job struct {
	Key          string   `json:"key"`
	PersonaKey   *string  `json:"persona_key"`
	Situation    string   `json:"situation"`
	Motivation   string   `json:"motivation"`
	Outcome      string   `json:"outcome"`
	ActivityKeys []string `json:"activity_keys"`
	Position     int      `json:"position"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// StoryActivity is a column group of the story map's backbone (L3).
type StoryActivity struct {
	Key         string  `json:"key"`
	PersonaKey  *string `json:"persona_key"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Position    int     `json:"position"`
	Steps       []Step  `json:"steps"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// Step is one step of an activity (L4).
type Step struct {
	Key         string `json:"key"`
	ActivityKey string `json:"activity_key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Position    int    `json:"position"`
}

// Story is a user story placed under steps (L5).
type Story struct {
	Key           string       `json:"key"`
	Title         string       `json:"title"`
	Body          string       `json:"body"` // free Markdown (option D)
	PersonaKey    *string      `json:"persona_key"`
	IWant         string       `json:"i_want"`
	SoThat        string       `json:"so_that"`
	Release       string       `json:"release"`
	Position      int          `json:"position"`
	StepKeys      []string     `json:"step_keys"`
	Status        string       `json:"status"` // draft | ready | in_progress | done
	CriteriaCount int          `json:"criteria_count"`
	Issues        []StoryIssue `json:"issues"`
	CreatedAt     string       `json:"created_at"`
	UpdatedAt     string       `json:"updated_at"`

	// Detail-only fields.
	Criteria *[]Criterion `json:"criteria,omitempty"`
	History  *[]ReqChange `json:"history,omitempty"`

	id string
}

// StoryIssue is an issue (development task) linked to a story.
type StoryIssue struct {
	Key           string `json:"key"`
	Type          string `json:"type"`
	Title         string `json:"title"`
	StateName     string `json:"state_name"`
	StateCategory string `json:"state_category"`
}

// Criterion is an acceptance criterion of a story (L6).
type Criterion struct {
	Key       string `json:"key"`
	StoryKey  string `json:"story_key"`
	Given     string `json:"given"`
	When      string `json:"when"`
	Then      string `json:"then"`
	Position  int    `json:"position"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ReqChange is one entry of the change history.
type ReqChange struct {
	EntityKey string          `json:"entity_key"`
	Action    string          `json:"action"`
	Detail    json.RawMessage `json:"detail"`
	Actor     UserRef         `json:"actor"`
	CreatedAt string          `json:"created_at"`
}

// StoryRef is a story as shown on an issue: why the issue exists.
type StoryRef struct {
	Key        string      `json:"key"`
	Title      string      `json:"title"`
	Body       string      `json:"body"`
	PersonaKey *string     `json:"persona_key"`
	IWant      string      `json:"i_want"`
	SoThat     string      `json:"so_that"`
	StepKeys   []string    `json:"step_keys"`
	Criteria   []Criterion `json:"criteria"`
}

// StoryMapGaps lists missing links (docs/08 §5.4).
type StoryMapGaps struct {
	StoriesWithoutSteps    []string `json:"stories_without_steps"`
	StepsWithoutStories    []string `json:"steps_without_stories"`
	StoriesWithoutCriteria []string `json:"stories_without_criteria"`
	StoriesWithoutIssues   []string `json:"stories_without_issues"`
	JobsWithoutActivities  []string `json:"jobs_without_activities"`
}

// StoryMap is a project's whole requirements layer.
type StoryMap struct {
	ProjectKey string `json:"project_key"`
	// Mode is the project's requirements_mode (structured | quick), so a client
	// or an agent knows how strictly to write.
	Mode string `json:"mode"`
	// DraftOf is the target's key when this project is a draft (§5.9).
	DraftOf    *string         `json:"draft_of"`
	Personas   []Persona       `json:"personas"`
	Jobs       []Job           `json:"jobs"`
	Activities []StoryActivity `json:"activities"`
	Stories    []Story         `json:"stories"`
	Releases   []string        `json:"releases"` // slice names in map order (same order as release_slices)
	// ReleaseSlices are the rows with their goal, status and progress (§5.8).
	ReleaseSlices []Release    `json:"release_slices"`
	Gaps          StoryMapGaps `json:"gaps"`
}

// ---- inputs ----

// PersonaInput creates a persona.
type PersonaInput struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

// PersonaPatch changes a persona.
type PersonaPatch struct {
	Name        *string `json:"name"`
	Kind        *string `json:"kind"`
	Description *string `json:"description"`
	Position    *int    `json:"position"`
}

// JobInput creates a job.
type JobInput struct {
	PersonaKey   *string  `json:"persona_key"`
	Situation    string   `json:"situation"`
	Motivation   string   `json:"motivation"`
	Outcome      string   `json:"outcome"`
	ActivityKeys []string `json:"activity_keys"`
}

// JobPatch changes a job. ActivityKeys replaces the links.
type JobPatch struct {
	PersonaKey   Opt[string] `json:"persona_key"`
	Situation    *string     `json:"situation"`
	Motivation   *string     `json:"motivation"`
	Outcome      *string     `json:"outcome"`
	ActivityKeys *[]string   `json:"activity_keys"`
	Position     *int        `json:"position"`
}

// StoryActivityInput creates an activity.
type StoryActivityInput struct {
	PersonaKey  *string `json:"persona_key"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
}

// StoryActivityPatch changes an activity.
type StoryActivityPatch struct {
	PersonaKey  Opt[string] `json:"persona_key"`
	Name        *string     `json:"name"`
	Description *string     `json:"description"`
	Position    *int        `json:"position"`
}

// StepInput creates a step.
type StepInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// StepPatch changes a step.
type StepPatch struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Position    *int    `json:"position"`
}

// StoryInput creates a story.
type StoryInput struct {
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	PersonaKey *string  `json:"persona_key"`
	IWant      string   `json:"i_want"`
	SoThat     string   `json:"so_that"`
	Release    string   `json:"release"`
	StepKeys   []string `json:"step_keys"`
}

// StoryPatch changes a story. StepKeys replaces the placement.
type StoryPatch struct {
	Title      *string     `json:"title"`
	Body       *string     `json:"body"`
	PersonaKey Opt[string] `json:"persona_key"`
	IWant      *string     `json:"i_want"`
	SoThat     *string     `json:"so_that"`
	Release    *string     `json:"release"`
	StepKeys   *[]string   `json:"step_keys"`
	Position   *int        `json:"position"`
}

// ReqCriterionInput creates a criterion.
type ReqCriterionInput struct {
	Given string `json:"given"`
	When  string `json:"when"`
	Then  string `json:"then"`
}

// ReqCriterionPatch changes a criterion.
type ReqCriterionPatch struct {
	Given    *string `json:"given"`
	When     *string `json:"when"`
	Then     *string `json:"then"`
	Position *int    `json:"position"`
}

// ---- keys ----

type reqKind struct {
	table, prefix, label string
}

var (
	kindPersona   = reqKind{"req_personas", "P", "persona"}
	kindJob       = reqKind{"req_jobs", "J", "job"}
	kindActivity  = reqKind{"req_activities", "A", "activity"}
	kindStory     = reqKind{"req_stories", "ST", "story"}
	kindCriterion = reqKind{"req_criteria", "AC", "criterion"}
)

var stepKeyRe = regexp.MustCompile(`^A(\d+)-(\d+)$`)

// reqLocal strips the project prefix: "tapp-st12" / "ST12" → "ST12".
func reqLocal(projectKey, raw string) string {
	k := strings.ToUpper(strings.TrimSpace(raw))
	return strings.TrimPrefix(k, projectKey+"-")
}

// ParseReqNum parses a key of the given prefix (ST, AC, ...) into its number.
func ParseReqNum(projectKey, prefix, raw string) (int, bool) {
	k := reqLocal(projectKey, raw)
	if !strings.HasPrefix(k, prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(k[len(prefix):])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// ParseStepKey parses "TAPP-A1-2" / "A1-2" into the activity and step numbers.
func ParseStepKey(projectKey, raw string) (int, int, bool) {
	m := stepKeyRe.FindStringSubmatch(reqLocal(projectKey, raw))
	if m == nil {
		return 0, 0, false
	}
	a, _ := strconv.Atoi(m[1])
	n, _ := strconv.Atoi(m[2])
	return a, n, a > 0 && n > 0
}

func fmtKey(projectKey, prefix string, n int) string {
	return fmt.Sprintf("%s-%s%d", projectKey, prefix, n)
}

func fmtStepKey(projectKey string, a, n int) string {
	return fmt.Sprintf("%s-A%d-%d", projectKey, a, n)
}

// ---- project scope ----

type reqProject struct {
	id, key string
	mode    string // requirements_mode
}

func (s *Service) reqProject(ctx context.Context, key string) (reqProject, Actor, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return reqProject{}, actor, err
	}
	var p reqProject
	err = s.db.QueryRowContext(ctx, `SELECT id, key, requirements_mode FROM projects WHERE key = ?`, strings.ToUpper(key)).Scan(&p.id, &p.key, &p.mode)
	if errors.Is(err, sql.ErrNoRows) {
		return p, actor, Errorf(CodeNotFound, "project %q not found", key)
	}
	return p, actor, err
}

// idOf resolves the key of an active persona, job, activity, story or criterion.
func (p reqProject) idOf(ctx context.Context, q queryer, k reqKind, raw string) (string, error) {
	n, ok := ParseReqNum(p.key, k.prefix, raw)
	if !ok {
		return "", Errorf(CodeNotFound, "%s %q not found", k.label, raw)
	}
	var id string
	err := q.QueryRowContext(ctx, `SELECT id FROM `+k.table+` WHERE project_id = ? AND num = ? AND archived_at IS NULL`, p.id, n).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", Errorf(CodeNotFound, "%s %s not found", k.label, fmtKey(p.key, k.prefix, n))
	}
	return id, err
}

// refOf resolves a key given in a request body: unknown keys are bad input (400), not a missing resource.
func (p reqProject) refOf(ctx context.Context, q queryer, k reqKind, raw string) (string, error) {
	id, err := p.idOf(ctx, q, k, raw)
	var e *Error
	if errors.As(err, &e) && e.Code == CodeNotFound {
		return "", Errorf(CodeInvalidArgument, "%s %q does not exist in project %s", k.label, raw, p.key)
	}
	return id, err
}

func (p reqProject) stepID(ctx context.Context, q queryer, raw string) (string, error) {
	a, n, ok := ParseStepKey(p.key, raw)
	if !ok {
		return "", Errorf(CodeNotFound, "step %q not found", raw)
	}
	var id string
	err := q.QueryRowContext(ctx, `SELECT st.id FROM req_steps st JOIN req_activities a ON a.id = st.activity_id
		WHERE a.project_id = ? AND a.num = ? AND st.num = ? AND st.archived_at IS NULL AND a.archived_at IS NULL`, p.id, a, n).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", Errorf(CodeNotFound, "step %s not found", fmtStepKey(p.key, a, n))
	}
	return id, err
}

func (p reqProject) stepRefs(ctx context.Context, q queryer, raws []string) ([]string, error) {
	ids := []string{}
	seen := map[string]bool{}
	for _, raw := range raws {
		id, err := p.stepID(ctx, q, raw)
		var e *Error
		if errors.As(err, &e) && e.Code == CodeNotFound {
			return nil, Errorf(CodeInvalidArgument, "step %q does not exist in project %s", raw, p.key)
		}
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (p reqProject) optionalPersona(ctx context.Context, q queryer, raw *string) (any, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	return p.refOf(ctx, q, kindPersona, *raw)
}

func nextReqNum(ctx context.Context, tx *sql.Tx, projectID, kind string) (int, error) {
	if _, err := tx.ExecContext(ctx, `INSERT INTO req_counters (project_id, kind, next) VALUES (?, ?, 1)
		ON CONFLICT (project_id, kind) DO UPDATE SET next = next + 1`, projectID, kind); err != nil {
		return 0, err
	}
	var n int
	err := tx.QueryRowContext(ctx, `SELECT next FROM req_counters WHERE project_id = ? AND kind = ?`, projectID, kind).Scan(&n)
	return n, err
}

func (s *Service) reqChange(ctx context.Context, tx *sql.Tx, p reqProject, actor Actor, entityKey string, storyID *string, action string, detail map[string]any) error {
	if detail == nil {
		detail = map[string]any{}
	}
	b, _ := json.Marshal(detail)
	_, err := tx.ExecContext(ctx, `INSERT INTO req_changes (id, project_id, entity_key, story_id, actor_id, action, detail, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		newID(), p.id, entityKey, nullString(storyID), actor.User.ID, action, string(b), s.timestamp())
	return err
}

func (s *Service) reqChanged(p reqProject, actor Actor) {
	s.bus.Publish(events.Event{Type: "storymap.updated", ProjectKey: p.key, ActorID: actor.User.ID})
}

// patchSet collects "col = ?" assignments and the changed fields for the history.
type patchSet struct {
	sets   []string
	args   []any
	detail map[string]any
}

func (ps *patchSet) add(col, field string, v any) {
	ps.sets = append(ps.sets, col+" = ?")
	ps.args = append(ps.args, v)
	if ps.detail == nil {
		ps.detail = map[string]any{}
	}
	ps.detail[field] = v
}

func (ps *patchSet) exec(ctx context.Context, tx *sql.Tx, table, id, now string) error {
	if len(ps.sets) == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `UPDATE `+table+` SET `+strings.Join(ps.sets, ", ")+`, updated_at = ? WHERE id = ?`, append(ps.args, now, id)...)
	return err
}

func required(field string, v *string) (string, error) {
	t := strings.TrimSpace(*v)
	if t == "" {
		return "", Errorf(CodeInvalidArgument, "%s is required", field)
	}
	return t, nil
}

func validKind(k string) (string, error) {
	if k == "" {
		return "human", nil
	}
	if k != "human" && k != "agent" {
		return "", Errorf(CodeInvalidArgument, "kind must be human or agent")
	}
	return k, nil
}

func nextPosition(ctx context.Context, tx *sql.Tx, table, scopeCol, scopeID string) (int, error) {
	var n int
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) + 1 FROM `+table+` WHERE `+scopeCol+` = ? AND archived_at IS NULL`, scopeID).Scan(&n)
	return n, err
}

// ---- story map (read) ----

// GetStoryMap returns a project's personas, jobs, activities with steps,
// stories, release names and the missing links.
func (s *Service) GetStoryMap(ctx context.Context, projectKey string) (StoryMap, error) {
	p, _, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return StoryMap{}, err
	}
	return s.loadStoryMap(ctx, p)
}

func (s *Service) loadStoryMap(ctx context.Context, p reqProject) (StoryMap, error) {
	m := StoryMap{ProjectKey: p.key, Mode: p.mode, Personas: []Persona{}, Jobs: []Job{}, Activities: []StoryActivity{}, Stories: []Story{}, Releases: []string{}, ReleaseSlices: []Release{}}
	var draftOf sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT t.key FROM projects d JOIN projects t ON t.id = d.draft_of WHERE d.id = ?`, p.id).Scan(&draftOf); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return m, err
	}
	m.DraftOf = strPtr(draftOf)
	personaKey := map[string]string{}

	rows, err := s.db.QueryContext(ctx, `SELECT id, num, name, kind, description, position, created_at, updated_at
		FROM req_personas WHERE project_id = ? AND archived_at IS NULL ORDER BY position, num`, p.id)
	if err != nil {
		return m, err
	}
	for rows.Next() {
		var id string
		var n int
		var x Persona
		if err := rows.Scan(&id, &n, &x.Name, &x.Kind, &x.Description, &x.Position, &x.CreatedAt, &x.UpdatedAt); err != nil {
			rows.Close()
			return m, err
		}
		x.Key = fmtKey(p.key, "P", n)
		personaKey[id] = x.Key
		m.Personas = append(m.Personas, x)
	}
	rows.Close()
	ref := func(id sql.NullString) *string {
		if k, ok := personaKey[id.String]; ok && id.Valid {
			return &k
		}
		return nil
	}

	activityKey := map[string]string{}
	activityIdx := map[string]int{}
	rows, err = s.db.QueryContext(ctx, `SELECT id, num, persona_id, name, description, position, created_at, updated_at
		FROM req_activities WHERE project_id = ? AND archived_at IS NULL ORDER BY position, num`, p.id)
	if err != nil {
		return m, err
	}
	activityNum := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		var pid sql.NullString
		var x StoryActivity
		if err := rows.Scan(&id, &n, &pid, &x.Name, &x.Description, &x.Position, &x.CreatedAt, &x.UpdatedAt); err != nil {
			rows.Close()
			return m, err
		}
		x.Key = fmtKey(p.key, "A", n)
		x.PersonaKey = ref(pid)
		x.Steps = []Step{}
		activityKey[id], activityIdx[id], activityNum[id] = x.Key, len(m.Activities), n
		m.Activities = append(m.Activities, x)
	}
	rows.Close()

	stepKey := map[string]string{}
	stepOrder := []string{}
	rows, err = s.db.QueryContext(ctx, `SELECT id, activity_id, num, name, description, position
		FROM req_steps WHERE project_id = ? AND archived_at IS NULL ORDER BY position, num`, p.id)
	if err != nil {
		return m, err
	}
	for rows.Next() {
		var id, aid string
		var n int
		var x Step
		if err := rows.Scan(&id, &aid, &n, &x.Name, &x.Description, &x.Position); err != nil {
			rows.Close()
			return m, err
		}
		i, ok := activityIdx[aid]
		if !ok {
			continue // archived activity
		}
		x.Key = fmtStepKey(p.key, activityNum[aid], n)
		x.ActivityKey = activityKey[aid]
		stepKey[id] = x.Key
		m.Activities[i].Steps = append(m.Activities[i].Steps, x)
	}
	rows.Close()
	for _, a := range m.Activities {
		for _, st := range a.Steps {
			stepOrder = append(stepOrder, st.Key)
		}
	}

	jobIdx := map[string]int{}
	rows, err = s.db.QueryContext(ctx, `SELECT id, num, persona_id, situation, motivation, outcome, position, created_at, updated_at
		FROM req_jobs WHERE project_id = ? AND archived_at IS NULL ORDER BY position, num`, p.id)
	if err != nil {
		return m, err
	}
	for rows.Next() {
		var id string
		var n int
		var pid sql.NullString
		var x Job
		if err := rows.Scan(&id, &n, &pid, &x.Situation, &x.Motivation, &x.Outcome, &x.Position, &x.CreatedAt, &x.UpdatedAt); err != nil {
			rows.Close()
			return m, err
		}
		x.Key = fmtKey(p.key, "J", n)
		x.PersonaKey = ref(pid)
		x.ActivityKeys = []string{}
		jobIdx[id] = len(m.Jobs)
		m.Jobs = append(m.Jobs, x)
	}
	rows.Close()
	rows, err = s.db.QueryContext(ctx, `SELECT ja.job_id, ja.activity_id FROM req_job_activities ja JOIN req_jobs j ON j.id = ja.job_id WHERE j.project_id = ?`, p.id)
	if err != nil {
		return m, err
	}
	for rows.Next() {
		var jid, aid string
		if err := rows.Scan(&jid, &aid); err != nil {
			rows.Close()
			return m, err
		}
		if i, ok := jobIdx[jid]; ok {
			if k, ok := activityKey[aid]; ok {
				m.Jobs[i].ActivityKeys = append(m.Jobs[i].ActivityKeys, k)
			}
		}
	}
	rows.Close()
	for i := range m.Jobs {
		sort.Strings(m.Jobs[i].ActivityKeys)
	}

	stories, err := s.loadStories(ctx, p, `s.project_id = ?`, []any{p.id}, personaKey, stepKey)
	if err != nil {
		return m, err
	}
	m.Stories = stories

	// Release slices in their own order; every name on a story has one (ensureRelease).
	rels, err := s.loadReleases(ctx, s.db, p)
	if err != nil {
		return m, err
	}
	m.ReleaseSlices = withCounts(rels, m.Stories)
	for _, r := range m.ReleaseSlices {
		m.Releases = append(m.Releases, r.Name)
	}

	// Gaps.
	g := StoryMapGaps{StoriesWithoutSteps: []string{}, StepsWithoutStories: []string{}, StoriesWithoutCriteria: []string{},
		StoriesWithoutIssues: []string{}, JobsWithoutActivities: []string{}}
	used := map[string]bool{}
	for _, st := range m.Stories {
		for _, k := range st.StepKeys {
			used[k] = true
		}
		if len(st.StepKeys) == 0 {
			g.StoriesWithoutSteps = append(g.StoriesWithoutSteps, st.Key)
		}
		if st.CriteriaCount == 0 {
			g.StoriesWithoutCriteria = append(g.StoriesWithoutCriteria, st.Key)
		}
		if len(st.Issues) == 0 {
			g.StoriesWithoutIssues = append(g.StoriesWithoutIssues, st.Key)
		}
	}
	for _, k := range stepOrder {
		if !used[k] {
			g.StepsWithoutStories = append(g.StepsWithoutStories, k)
		}
	}
	for _, j := range m.Jobs {
		if len(j.ActivityKeys) == 0 {
			g.JobsWithoutActivities = append(g.JobsWithoutActivities, j.Key)
		}
	}
	m.Gaps = g
	return m, nil
}

// loadStories loads active stories matching where (alias s) with their
// placement, criteria count, linked issues and status.
func (s *Service) loadStories(ctx context.Context, p reqProject, where string, args []any, personaKey, stepKey map[string]string) ([]Story, error) {
	if personaKey == nil {
		personaKey = map[string]string{}
		rows, err := s.db.QueryContext(ctx, `SELECT id, num FROM req_personas WHERE project_id = ? AND archived_at IS NULL`, p.id)
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
			personaKey[id] = fmtKey(p.key, "P", n)
		}
		rows.Close()
	}
	if stepKey == nil {
		stepKey = map[string]string{}
		rows, err := s.db.QueryContext(ctx, `SELECT st.id, a.num, st.num FROM req_steps st JOIN req_activities a ON a.id = st.activity_id
			WHERE st.project_id = ? AND st.archived_at IS NULL AND a.archived_at IS NULL`, p.id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var a, n int
			if err := rows.Scan(&id, &a, &n); err != nil {
				rows.Close()
				return nil, err
			}
			stepKey[id] = fmtStepKey(p.key, a, n)
		}
		rows.Close()
	}

	rows, err := s.db.QueryContext(ctx, `SELECT s.id, s.num, s.title, s.body, s.persona_id, s.i_want, s.so_that, s.release, s.position, s.created_at, s.updated_at,
		(SELECT COUNT(*) FROM req_criteria c WHERE c.story_id = s.id AND c.archived_at IS NULL)
		FROM req_stories s WHERE `+where+` AND s.archived_at IS NULL ORDER BY s.position, s.num`, args...)
	if err != nil {
		return nil, err
	}
	out := []Story{}
	idx := map[string]int{}
	for rows.Next() {
		var x Story
		var n int
		var pid sql.NullString
		if err := rows.Scan(&x.id, &n, &x.Title, &x.Body, &pid, &x.IWant, &x.SoThat, &x.Release, &x.Position, &x.CreatedAt, &x.UpdatedAt, &x.CriteriaCount); err != nil {
			rows.Close()
			return nil, err
		}
		x.Key = fmtKey(p.key, "ST", n)
		if k, ok := personaKey[pid.String]; ok && pid.Valid {
			x.PersonaKey = &k
		}
		x.StepKeys, x.Issues = []string{}, []StoryIssue{}
		idx[x.id] = len(out)
		out = append(out, x)
	}
	rows.Close()
	if len(out) == 0 {
		return out, nil
	}

	rows, err = s.db.QueryContext(ctx, `SELECT ss.story_id, ss.step_id FROM req_story_steps ss JOIN req_stories s ON s.id = ss.story_id WHERE s.project_id = ?`, p.id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sid, stid string
		if err := rows.Scan(&sid, &stid); err != nil {
			rows.Close()
			return nil, err
		}
		if i, ok := idx[sid]; ok {
			if k, ok := stepKey[stid]; ok {
				out[i].StepKeys = append(out[i].StepKeys, k)
			}
		}
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx, `SELECT si.story_id, i.key, i.type, i.title, st.name, st.category
		FROM req_story_issues si JOIN req_stories s ON s.id = si.story_id
		JOIN issues i ON i.id = si.issue_id JOIN workflow_states st ON st.id = i.state_id
		WHERE s.project_id = ? AND i.deleted_at IS NULL ORDER BY i.number`, p.id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sid string
		var x StoryIssue
		if err := rows.Scan(&sid, &x.Key, &x.Type, &x.Title, &x.StateName, &x.StateCategory); err != nil {
			rows.Close()
			return nil, err
		}
		if i, ok := idx[sid]; ok {
			out[i].Issues = append(out[i].Issues, x)
		}
	}
	rows.Close()

	for i := range out {
		sort.Slice(out[i].StepKeys, func(a, b int) bool { return stepLess(out[i].StepKeys[a], out[i].StepKeys[b]) })
		out[i].Status = storyStatus(out[i])
	}
	return out, nil
}

func stepLess(a, b string) bool {
	var a1, a2, b1, b2 int
	fmt.Sscanf(a[strings.LastIndex(a, "-A")+2:], "%d-%d", &a1, &a2)
	fmt.Sscanf(b[strings.LastIndex(b, "-A")+2:], "%d-%d", &b1, &b2)
	if a1 != b1 {
		return a1 < b1
	}
	return a2 < b2
}

// storyStatus colours a story card (docs/08 §5.5): draft = no criteria,
// ready = criteria but no work started, in_progress = linked work under way,
// done = every linked issue finished (at least one completed).
func storyStatus(st Story) string {
	if st.CriteriaCount == 0 {
		return "draft"
	}
	if len(st.Issues) == 0 {
		return "ready"
	}
	finished, completed, started := 0, 0, 0
	for _, is := range st.Issues {
		switch is.StateCategory {
		case "completed":
			finished++
			completed++
		case "canceled":
			finished++
		case "started":
			started++
		}
	}
	if finished == len(st.Issues) && completed > 0 {
		return "done"
	}
	if started > 0 || completed > 0 {
		return "in_progress"
	}
	return "ready"
}

func (s *Service) loadCriteria(ctx context.Context, p reqProject, storyIDs []string) (map[string][]Criterion, error) {
	out := map[string][]Criterion{}
	if len(storyIDs) == 0 {
		return out, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(storyIDs)), ",")
	args := make([]any, len(storyIDs))
	for i, id := range storyIDs {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx, `SELECT c.story_id, c.num, s.num, c.given_text, c.when_text, c.then_text, c.position, c.created_at, c.updated_at
		FROM req_criteria c JOIN req_stories s ON s.id = c.story_id
		WHERE c.story_id IN (`+ph+`) AND c.archived_at IS NULL ORDER BY c.position, c.num`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sid string
		var n, sn int
		var c Criterion
		if err := rows.Scan(&sid, &n, &sn, &c.Given, &c.When, &c.Then, &c.Position, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.Key, c.StoryKey = fmtKey(p.key, "AC", n), fmtKey(p.key, "ST", sn)
		out[sid] = append(out[sid], c)
	}
	return out, rows.Err()
}

// ---- personas ----

// ListPersonas returns a project's active personas.
func (s *Service) ListPersonas(ctx context.Context, projectKey string) ([]Persona, error) {
	m, err := s.GetStoryMap(ctx, projectKey)
	return m.Personas, err
}

func (s *Service) getPersona(ctx context.Context, p reqProject, key string) (Persona, error) {
	m, err := s.loadStoryMap(ctx, p)
	if err != nil {
		return Persona{}, err
	}
	for _, x := range m.Personas {
		if x.Key == key {
			return x, nil
		}
	}
	return Persona{}, Errorf(CodeNotFound, "persona %s not found", key)
}

// CreatePersona adds a persona.
func (s *Service) CreatePersona(ctx context.Context, projectKey string, in PersonaInput) (Persona, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Persona{}, err
	}
	name, err := required("name", &in.Name)
	if err != nil {
		return Persona{}, err
	}
	kind, err := validKind(in.Kind)
	if err != nil {
		return Persona{}, err
	}
	var key string
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		n, err := nextReqNum(ctx, tx, p.id, "persona")
		if err != nil {
			return err
		}
		pos, err := nextPosition(ctx, tx, "req_personas", "project_id", p.id)
		if err != nil {
			return err
		}
		now := s.timestamp()
		if _, err := tx.ExecContext(ctx, `INSERT INTO req_personas (id, project_id, num, name, kind, description, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			newID(), p.id, n, name, kind, in.Description, pos, now, now); err != nil {
			return err
		}
		key = fmtKey(p.key, "P", n)
		return s.reqChange(ctx, tx, p, actor, key, nil, "created", map[string]any{"name": name})
	})
	if err != nil {
		return Persona{}, err
	}
	s.reqChanged(p, actor)
	return s.getPersona(ctx, p, key)
}

// UpdatePersona changes a persona.
func (s *Service) UpdatePersona(ctx context.Context, projectKey, key string, in PersonaPatch) (Persona, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Persona{}, err
	}
	id, err := p.idOf(ctx, s.db, kindPersona, key)
	if err != nil {
		return Persona{}, err
	}
	n, _ := ParseReqNum(p.key, "P", key)
	full := fmtKey(p.key, "P", n)
	var ps patchSet
	if in.Name != nil {
		v, err := required("name", in.Name)
		if err != nil {
			return Persona{}, err
		}
		ps.add("name", "name", v)
	}
	if in.Kind != nil {
		v, err := validKind(*in.Kind)
		if err != nil {
			return Persona{}, err
		}
		ps.add("kind", "kind", v)
	}
	if in.Description != nil {
		ps.add("description", "description", *in.Description)
	}
	if in.Position != nil {
		ps.add("position", "position", *in.Position)
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if err := ps.exec(ctx, tx, "req_personas", id, s.timestamp()); err != nil {
			return err
		}
		if ps.detail == nil {
			return nil
		}
		return s.reqChange(ctx, tx, p, actor, full, nil, "updated", ps.detail)
	})
	if err != nil {
		return Persona{}, err
	}
	s.reqChanged(p, actor)
	return s.getPersona(ctx, p, full)
}

// ArchivePersona hides a persona. References to it are dropped from the map.
func (s *Service) ArchivePersona(ctx context.Context, projectKey, key string) error {
	return s.archiveReq(ctx, projectKey, kindPersona, key)
}

func (s *Service) archiveReq(ctx context.Context, projectKey string, k reqKind, key string) error {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return err
	}
	id, err := p.idOf(ctx, s.db, k, key)
	if err != nil {
		return err
	}
	n, _ := ParseReqNum(p.key, k.prefix, key)
	full := fmtKey(p.key, k.prefix, n)
	var storyID *string
	switch k {
	case kindStory:
		storyID = &id
	case kindCriterion:
		var sid string
		if err := s.db.QueryRowContext(ctx, `SELECT story_id FROM req_criteria WHERE id = ?`, id).Scan(&sid); err != nil {
			return err
		}
		storyID = &sid
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		now := s.timestamp()
		if _, err := tx.ExecContext(ctx, `UPDATE `+k.table+` SET archived_at = ?, updated_at = ? WHERE id = ?`, now, now, id); err != nil {
			return err
		}
		if k == kindActivity {
			if _, err := tx.ExecContext(ctx, `UPDATE req_steps SET archived_at = ?, updated_at = ? WHERE activity_id = ? AND archived_at IS NULL`, now, now, id); err != nil {
				return err
			}
		}
		return s.reqChange(ctx, tx, p, actor, full, storyID, "archived", nil)
	})
	if err != nil {
		return err
	}
	s.reqChanged(p, actor)
	return nil
}

// ---- jobs ----

// ListJobs returns a project's active jobs.
func (s *Service) ListJobs(ctx context.Context, projectKey string) ([]Job, error) {
	m, err := s.GetStoryMap(ctx, projectKey)
	return m.Jobs, err
}

func (s *Service) getJob(ctx context.Context, p reqProject, key string) (Job, error) {
	m, err := s.loadStoryMap(ctx, p)
	if err != nil {
		return Job{}, err
	}
	for _, x := range m.Jobs {
		if x.Key == key {
			return x, nil
		}
	}
	return Job{}, Errorf(CodeNotFound, "job %s not found", key)
}

func (s *Service) setJobActivities(ctx context.Context, tx *sql.Tx, p reqProject, jobID string, keys []string) error {
	ids := []string{}
	seen := map[string]bool{}
	for _, k := range keys {
		id, err := p.refOf(ctx, tx, kindActivity, k)
		if err != nil {
			return err
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM req_job_activities WHERE job_id = ?`, jobID); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `INSERT INTO req_job_activities (job_id, activity_id) VALUES (?, ?)`, jobID, id); err != nil {
			return err
		}
	}
	return nil
}

// CreateJob adds a job.
func (s *Service) CreateJob(ctx context.Context, projectKey string, in JobInput) (Job, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Job{}, err
	}
	motivation, err := required("motivation", &in.Motivation)
	if err != nil {
		return Job{}, err
	}
	var key string
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		persona, err := p.optionalPersona(ctx, tx, in.PersonaKey)
		if err != nil {
			return err
		}
		n, err := nextReqNum(ctx, tx, p.id, "job")
		if err != nil {
			return err
		}
		pos, err := nextPosition(ctx, tx, "req_jobs", "project_id", p.id)
		if err != nil {
			return err
		}
		id, now := newID(), s.timestamp()
		if _, err := tx.ExecContext(ctx, `INSERT INTO req_jobs (id, project_id, num, persona_id, situation, motivation, outcome, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			id, p.id, n, persona, strings.TrimSpace(in.Situation), motivation, strings.TrimSpace(in.Outcome), pos, now, now); err != nil {
			return err
		}
		if err := s.setJobActivities(ctx, tx, p, id, in.ActivityKeys); err != nil {
			return err
		}
		key = fmtKey(p.key, "J", n)
		return s.reqChange(ctx, tx, p, actor, key, nil, "created", map[string]any{"motivation": motivation})
	})
	if err != nil {
		return Job{}, err
	}
	s.reqChanged(p, actor)
	return s.getJob(ctx, p, key)
}

// UpdateJob changes a job.
func (s *Service) UpdateJob(ctx context.Context, projectKey, key string, in JobPatch) (Job, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Job{}, err
	}
	id, err := p.idOf(ctx, s.db, kindJob, key)
	if err != nil {
		return Job{}, err
	}
	n, _ := ParseReqNum(p.key, "J", key)
	full := fmtKey(p.key, "J", n)
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var ps patchSet
		if in.PersonaKey.Set {
			var v *string
			if !in.PersonaKey.Null {
				v = &in.PersonaKey.Value
			}
			persona, err := p.optionalPersona(ctx, tx, v)
			if err != nil {
				return err
			}
			ps.add("persona_id", "persona_key", persona)
			ps.detail["persona_key"] = v
		}
		if in.Situation != nil {
			ps.add("situation", "situation", strings.TrimSpace(*in.Situation))
		}
		if in.Motivation != nil {
			v, err := required("motivation", in.Motivation)
			if err != nil {
				return err
			}
			ps.add("motivation", "motivation", v)
		}
		if in.Outcome != nil {
			ps.add("outcome", "outcome", strings.TrimSpace(*in.Outcome))
		}
		if in.Position != nil {
			ps.add("position", "position", *in.Position)
		}
		if err := ps.exec(ctx, tx, "req_jobs", id, s.timestamp()); err != nil {
			return err
		}
		if in.ActivityKeys != nil {
			if err := s.setJobActivities(ctx, tx, p, id, *in.ActivityKeys); err != nil {
				return err
			}
			if ps.detail == nil {
				ps.detail = map[string]any{}
			}
			ps.detail["activity_keys"] = *in.ActivityKeys
		}
		if ps.detail == nil {
			return nil
		}
		return s.reqChange(ctx, tx, p, actor, full, nil, "updated", ps.detail)
	})
	if err != nil {
		return Job{}, err
	}
	s.reqChanged(p, actor)
	return s.getJob(ctx, p, full)
}

// ArchiveJob hides a job.
func (s *Service) ArchiveJob(ctx context.Context, projectKey, key string) error {
	return s.archiveReq(ctx, projectKey, kindJob, key)
}

// ---- activities and steps ----

func (s *Service) getActivity(ctx context.Context, p reqProject, key string) (StoryActivity, error) {
	m, err := s.loadStoryMap(ctx, p)
	if err != nil {
		return StoryActivity{}, err
	}
	for _, x := range m.Activities {
		if x.Key == key {
			return x, nil
		}
	}
	return StoryActivity{}, Errorf(CodeNotFound, "activity %s not found", key)
}

// CreateActivity adds an activity at the end of the backbone.
func (s *Service) CreateActivity(ctx context.Context, projectKey string, in StoryActivityInput) (StoryActivity, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return StoryActivity{}, err
	}
	name, err := required("name", &in.Name)
	if err != nil {
		return StoryActivity{}, err
	}
	var key string
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		persona, err := p.optionalPersona(ctx, tx, in.PersonaKey)
		if err != nil {
			return err
		}
		n, err := nextReqNum(ctx, tx, p.id, "activity")
		if err != nil {
			return err
		}
		pos, err := nextPosition(ctx, tx, "req_activities", "project_id", p.id)
		if err != nil {
			return err
		}
		now := s.timestamp()
		if _, err := tx.ExecContext(ctx, `INSERT INTO req_activities (id, project_id, num, persona_id, name, description, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			newID(), p.id, n, persona, name, in.Description, pos, now, now); err != nil {
			return err
		}
		key = fmtKey(p.key, "A", n)
		return s.reqChange(ctx, tx, p, actor, key, nil, "created", map[string]any{"name": name})
	})
	if err != nil {
		return StoryActivity{}, err
	}
	s.reqChanged(p, actor)
	return s.getActivity(ctx, p, key)
}

// UpdateActivity changes an activity.
func (s *Service) UpdateActivity(ctx context.Context, projectKey, key string, in StoryActivityPatch) (StoryActivity, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return StoryActivity{}, err
	}
	id, err := p.idOf(ctx, s.db, kindActivity, key)
	if err != nil {
		return StoryActivity{}, err
	}
	n, _ := ParseReqNum(p.key, "A", key)
	full := fmtKey(p.key, "A", n)
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var ps patchSet
		if in.PersonaKey.Set {
			var v *string
			if !in.PersonaKey.Null {
				v = &in.PersonaKey.Value
			}
			persona, err := p.optionalPersona(ctx, tx, v)
			if err != nil {
				return err
			}
			ps.add("persona_id", "persona_key", persona)
			ps.detail["persona_key"] = v
		}
		if in.Name != nil {
			v, err := required("name", in.Name)
			if err != nil {
				return err
			}
			ps.add("name", "name", v)
		}
		if in.Description != nil {
			ps.add("description", "description", *in.Description)
		}
		if in.Position != nil {
			ps.add("position", "position", *in.Position)
		}
		if err := ps.exec(ctx, tx, "req_activities", id, s.timestamp()); err != nil {
			return err
		}
		if ps.detail == nil {
			return nil
		}
		return s.reqChange(ctx, tx, p, actor, full, nil, "updated", ps.detail)
	})
	if err != nil {
		return StoryActivity{}, err
	}
	s.reqChanged(p, actor)
	return s.getActivity(ctx, p, full)
}

// ArchiveActivity hides an activity and its steps.
func (s *Service) ArchiveActivity(ctx context.Context, projectKey, key string) error {
	return s.archiveReq(ctx, projectKey, kindActivity, key)
}

func (s *Service) getStep(ctx context.Context, p reqProject, key string) (Step, error) {
	m, err := s.loadStoryMap(ctx, p)
	if err != nil {
		return Step{}, err
	}
	for _, a := range m.Activities {
		for _, x := range a.Steps {
			if x.Key == key {
				return x, nil
			}
		}
	}
	return Step{}, Errorf(CodeNotFound, "step %s not found", key)
}

// CreateStep adds a step at the end of an activity.
func (s *Service) CreateStep(ctx context.Context, projectKey, activityKey string, in StepInput) (Step, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Step{}, err
	}
	aid, err := p.idOf(ctx, s.db, kindActivity, activityKey)
	if err != nil {
		return Step{}, err
	}
	an, _ := ParseReqNum(p.key, "A", activityKey)
	name, err := required("name", &in.Name)
	if err != nil {
		return Step{}, err
	}
	var key string
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		n, err := nextReqNum(ctx, tx, p.id, "step:"+aid)
		if err != nil {
			return err
		}
		pos, err := nextPosition(ctx, tx, "req_steps", "activity_id", aid)
		if err != nil {
			return err
		}
		now := s.timestamp()
		if _, err := tx.ExecContext(ctx, `INSERT INTO req_steps (id, project_id, activity_id, num, name, description, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			newID(), p.id, aid, n, name, in.Description, pos, now, now); err != nil {
			return err
		}
		key = fmtStepKey(p.key, an, n)
		return s.reqChange(ctx, tx, p, actor, key, nil, "created", map[string]any{"name": name})
	})
	if err != nil {
		return Step{}, err
	}
	s.reqChanged(p, actor)
	return s.getStep(ctx, p, key)
}

// UpdateStep changes a step (it stays in its activity).
func (s *Service) UpdateStep(ctx context.Context, projectKey, key string, in StepPatch) (Step, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Step{}, err
	}
	id, err := p.stepID(ctx, s.db, key)
	if err != nil {
		return Step{}, err
	}
	a, n, _ := ParseStepKey(p.key, key)
	full := fmtStepKey(p.key, a, n)
	var ps patchSet
	if in.Name != nil {
		v, err := required("name", in.Name)
		if err != nil {
			return Step{}, err
		}
		ps.add("name", "name", v)
	}
	if in.Description != nil {
		ps.add("description", "description", *in.Description)
	}
	if in.Position != nil {
		ps.add("position", "position", *in.Position)
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if err := ps.exec(ctx, tx, "req_steps", id, s.timestamp()); err != nil {
			return err
		}
		if ps.detail == nil {
			return nil
		}
		return s.reqChange(ctx, tx, p, actor, full, nil, "updated", ps.detail)
	})
	if err != nil {
		return Step{}, err
	}
	s.reqChanged(p, actor)
	return s.getStep(ctx, p, full)
}

// ArchiveStep hides a step. Stories placed only there show up as without steps.
func (s *Service) ArchiveStep(ctx context.Context, projectKey, key string) error {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return err
	}
	id, err := p.stepID(ctx, s.db, key)
	if err != nil {
		return err
	}
	a, n, _ := ParseStepKey(p.key, key)
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		now := s.timestamp()
		if _, err := tx.ExecContext(ctx, `UPDATE req_steps SET archived_at = ?, updated_at = ? WHERE id = ?`, now, now, id); err != nil {
			return err
		}
		return s.reqChange(ctx, tx, p, actor, fmtStepKey(p.key, a, n), nil, "archived", nil)
	})
	if err != nil {
		return err
	}
	s.reqChanged(p, actor)
	return nil
}

// ---- stories ----

// ListStories returns a project's active stories in map order.
func (s *Service) ListStories(ctx context.Context, projectKey string) ([]Story, error) {
	p, _, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	return s.loadStories(ctx, p, `s.project_id = ?`, []any{p.id}, nil, nil)
}

// GetStory returns a story with its criteria and change history.
func (s *Service) GetStory(ctx context.Context, projectKey, key string) (Story, error) {
	p, _, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Story{}, err
	}
	return s.getStory(ctx, p, key)
}

func (s *Service) getStory(ctx context.Context, p reqProject, key string) (Story, error) {
	id, err := p.idOf(ctx, s.db, kindStory, key)
	if err != nil {
		return Story{}, err
	}
	list, err := s.loadStories(ctx, p, `s.id = ?`, []any{id}, nil, nil)
	if err != nil {
		return Story{}, err
	}
	if len(list) == 0 {
		return Story{}, Errorf(CodeNotFound, "story %s not found", key)
	}
	st := list[0]
	crit, err := s.loadCriteria(ctx, p, []string{id})
	if err != nil {
		return Story{}, err
	}
	criteria := crit[id]
	if criteria == nil {
		criteria = []Criterion{}
	}
	st.Criteria = &criteria
	rows, err := s.db.QueryContext(ctx, `SELECT c.entity_key, c.action, c.detail, c.created_at, `+userRefCols+`
		FROM req_changes c JOIN users u ON u.id = c.actor_id WHERE c.story_id = ? ORDER BY c.created_at DESC, c.rowid DESC LIMIT 50`, id)
	if err != nil {
		return Story{}, err
	}
	defer rows.Close()
	history := []ReqChange{}
	st.History = &history
	for rows.Next() {
		var h ReqChange
		var detail string
		if err := rows.Scan(append([]any{&h.EntityKey, &h.Action, &detail, &h.CreatedAt}, userRefDest(&h.Actor)...)...); err != nil {
			return Story{}, err
		}
		h.Detail = json.RawMessage(detail)
		history = append(history, h)
	}
	return st, rows.Err()
}

func setStorySteps(ctx context.Context, tx *sql.Tx, storyID string, stepIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM req_story_steps WHERE story_id = ?`, storyID); err != nil {
		return err
	}
	for _, id := range stepIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO req_story_steps (story_id, step_id) VALUES (?, ?)`, storyID, id); err != nil {
			return err
		}
	}
	return nil
}

// CreateStory adds a story, optionally placed under steps.
func (s *Service) CreateStory(ctx context.Context, projectKey string, in StoryInput) (Story, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Story{}, err
	}
	title, err := required("title", &in.Title)
	if err != nil {
		return Story{}, err
	}
	var key string
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		persona, err := p.optionalPersona(ctx, tx, in.PersonaKey)
		if err != nil {
			return err
		}
		steps, err := p.stepRefs(ctx, tx, in.StepKeys)
		if err != nil {
			return err
		}
		n, err := nextReqNum(ctx, tx, p.id, "story")
		if err != nil {
			return err
		}
		pos, err := nextPosition(ctx, tx, "req_stories", "project_id", p.id)
		if err != nil {
			return err
		}
		id, now := newID(), s.timestamp()
		if _, err := tx.ExecContext(ctx, `INSERT INTO req_stories (id, project_id, num, title, body, persona_id, i_want, so_that, release, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, p.id, n, title, strings.TrimSpace(in.Body), persona, strings.TrimSpace(in.IWant), strings.TrimSpace(in.SoThat), strings.TrimSpace(in.Release), pos, now, now); err != nil {
			return err
		}
		if err := setStorySteps(ctx, tx, id, steps); err != nil {
			return err
		}
		if err := s.ensureRelease(ctx, tx, p, actor, strings.TrimSpace(in.Release)); err != nil {
			return err
		}
		key = fmtKey(p.key, "ST", n)
		return s.reqChange(ctx, tx, p, actor, key, &id, "created", map[string]any{"title": title})
	})
	if err != nil {
		return Story{}, err
	}
	s.reqChanged(p, actor)
	return s.getStory(ctx, p, key)
}

// UpdateStory changes a story.
func (s *Service) UpdateStory(ctx context.Context, projectKey, key string, in StoryPatch) (Story, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Story{}, err
	}
	id, err := p.idOf(ctx, s.db, kindStory, key)
	if err != nil {
		return Story{}, err
	}
	n, _ := ParseReqNum(p.key, "ST", key)
	full := fmtKey(p.key, "ST", n)
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var ps patchSet
		if in.Title != nil {
			v, err := required("title", in.Title)
			if err != nil {
				return err
			}
			ps.add("title", "title", v)
		}
		if in.PersonaKey.Set {
			var v *string
			if !in.PersonaKey.Null {
				v = &in.PersonaKey.Value
			}
			persona, err := p.optionalPersona(ctx, tx, v)
			if err != nil {
				return err
			}
			ps.add("persona_id", "persona_key", persona)
			ps.detail["persona_key"] = v
		}
		if in.Body != nil {
			ps.add("body", "body", strings.TrimSpace(*in.Body))
		}
		if in.IWant != nil {
			ps.add("i_want", "i_want", strings.TrimSpace(*in.IWant))
		}
		if in.SoThat != nil {
			ps.add("so_that", "so_that", strings.TrimSpace(*in.SoThat))
		}
		if in.Release != nil {
			rel := strings.TrimSpace(*in.Release)
			if err := s.ensureRelease(ctx, tx, p, actor, rel); err != nil {
				return err
			}
			ps.add("release", "release", rel)
		}
		if in.Position != nil {
			ps.add("position", "position", *in.Position)
		}
		if in.StepKeys != nil {
			steps, err := p.stepRefs(ctx, tx, *in.StepKeys)
			if err != nil {
				return err
			}
			if err := setStorySteps(ctx, tx, id, steps); err != nil {
				return err
			}
			if ps.detail == nil {
				ps.detail = map[string]any{}
			}
			ps.detail["step_keys"] = *in.StepKeys
			if len(ps.sets) == 0 {
				ps.sets, ps.args = []string{"position = position"}, []any{}
			}
		}
		if err := ps.exec(ctx, tx, "req_stories", id, s.timestamp()); err != nil {
			return err
		}
		if ps.detail == nil {
			return nil
		}
		return s.reqChange(ctx, tx, p, actor, full, &id, "updated", ps.detail)
	})
	if err != nil {
		return Story{}, err
	}
	s.reqChanged(p, actor)
	return s.getStory(ctx, p, full)
}

// ArchiveStory hides a story; its key is not reused.
func (s *Service) ArchiveStory(ctx context.Context, projectKey, key string) error {
	return s.archiveReq(ctx, projectKey, kindStory, key)
}

// LinkStoryIssue links a development issue of the same project to a story.
func (s *Service) LinkStoryIssue(ctx context.Context, projectKey, storyKey, issueKey string) (Story, error) {
	return s.linkStoryIssue(ctx, projectKey, storyKey, issueKey, true)
}

// UnlinkStoryIssue removes the link.
func (s *Service) UnlinkStoryIssue(ctx context.Context, projectKey, storyKey, issueKey string) (Story, error) {
	return s.linkStoryIssue(ctx, projectKey, storyKey, issueKey, false)
}

func (s *Service) linkStoryIssue(ctx context.Context, projectKey, storyKey, issueKey string, link bool) (Story, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Story{}, err
	}
	sid, err := p.idOf(ctx, s.db, kindStory, storyKey)
	if err != nil {
		return Story{}, err
	}
	is, err := s.issueByKey(ctx, s.db, issueKey)
	if err != nil {
		return Story{}, err
	}
	if is.ProjectKey != p.key {
		return Story{}, Errorf(CodeInvalidArgument, "issue %s belongs to project %s, not %s", is.Key, is.ProjectKey, p.key)
	}
	n, _ := ParseReqNum(p.key, "ST", storyKey)
	full := fmtKey(p.key, "ST", n)
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var res sql.Result
		var err error
		action := "linked"
		if link {
			res, err = tx.ExecContext(ctx, `INSERT INTO req_story_issues (story_id, issue_id, created_at) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`, sid, is.ID, s.timestamp())
		} else {
			action = "unlinked"
			res, err = tx.ExecContext(ctx, `DELETE FROM req_story_issues WHERE story_id = ? AND issue_id = ?`, sid, is.ID)
		}
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		return s.reqChange(ctx, tx, p, actor, full, &sid, action, map[string]any{"issue_key": is.Key})
	})
	if err != nil {
		return Story{}, err
	}
	s.reqChanged(p, actor)
	s.bus.Publish(events.Event{Type: "issue.updated", ProjectKey: is.ProjectKey, IssueKey: is.Key, ActorID: actor.User.ID})
	return s.getStory(ctx, p, full)
}

// ---- criteria ----

func (s *Service) criterionByID(ctx context.Context, p reqProject, id string) (Criterion, error) {
	var sid string
	if err := s.db.QueryRowContext(ctx, `SELECT story_id FROM req_criteria WHERE id = ?`, id).Scan(&sid); err != nil {
		return Criterion{}, err
	}
	m, err := s.loadCriteria(ctx, p, []string{sid})
	if err != nil {
		return Criterion{}, err
	}
	var n int
	_ = s.db.QueryRowContext(ctx, `SELECT num FROM req_criteria WHERE id = ?`, id).Scan(&n)
	for _, c := range m[sid] {
		if c.Key == fmtKey(p.key, "AC", n) {
			return c, nil
		}
	}
	return Criterion{}, Errorf(CodeNotFound, "criterion not found")
}

func criterionText(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

// CreateCriterion adds an acceptance criterion to a story.
func (s *Service) CreateCriterion(ctx context.Context, projectKey, storyKey string, in ReqCriterionInput) (Criterion, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Criterion{}, err
	}
	sid, err := p.idOf(ctx, s.db, kindStory, storyKey)
	if err != nil {
		return Criterion{}, err
	}
	then, err := required("then", &in.Then)
	if err != nil {
		return Criterion{}, err
	}
	id := newID()
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		n, err := nextReqNum(ctx, tx, p.id, "criterion")
		if err != nil {
			return err
		}
		pos, err := nextPosition(ctx, tx, "req_criteria", "story_id", sid)
		if err != nil {
			return err
		}
		now := s.timestamp()
		if _, err := tx.ExecContext(ctx, `INSERT INTO req_criteria (id, project_id, story_id, num, given_text, when_text, then_text, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			id, p.id, sid, n, criterionText(&in.Given), criterionText(&in.When), then, pos, now, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE req_stories SET updated_at = ? WHERE id = ?`, now, sid); err != nil {
			return err
		}
		return s.reqChange(ctx, tx, p, actor, fmtKey(p.key, "AC", n), &sid, "created", map[string]any{"given": in.Given, "when": in.When, "then": then})
	})
	if err != nil {
		return Criterion{}, err
	}
	s.reqChanged(p, actor)
	return s.criterionByID(ctx, p, id)
}

// UpdateCriterion changes a criterion.
func (s *Service) UpdateCriterion(ctx context.Context, projectKey, key string, in ReqCriterionPatch) (Criterion, error) {
	p, actor, err := s.reqProject(ctx, projectKey)
	if err != nil {
		return Criterion{}, err
	}
	id, err := p.idOf(ctx, s.db, kindCriterion, key)
	if err != nil {
		return Criterion{}, err
	}
	var sid string
	if err := s.db.QueryRowContext(ctx, `SELECT story_id FROM req_criteria WHERE id = ?`, id).Scan(&sid); err != nil {
		return Criterion{}, err
	}
	n, _ := ParseReqNum(p.key, "AC", key)
	var ps patchSet
	if in.Given != nil {
		ps.add("given_text", "given", criterionText(in.Given))
	}
	if in.When != nil {
		ps.add("when_text", "when", criterionText(in.When))
	}
	if in.Then != nil {
		v, err := required("then", in.Then)
		if err != nil {
			return Criterion{}, err
		}
		ps.add("then_text", "then", v)
	}
	if in.Position != nil {
		ps.add("position", "position", *in.Position)
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if err := ps.exec(ctx, tx, "req_criteria", id, s.timestamp()); err != nil {
			return err
		}
		if ps.detail == nil {
			return nil
		}
		return s.reqChange(ctx, tx, p, actor, fmtKey(p.key, "AC", n), &sid, "updated", ps.detail)
	})
	if err != nil {
		return Criterion{}, err
	}
	s.reqChanged(p, actor)
	return s.criterionByID(ctx, p, id)
}

// ArchiveCriterion hides a criterion.
func (s *Service) ArchiveCriterion(ctx context.Context, projectKey, key string) error {
	return s.archiveReq(ctx, projectKey, kindCriterion, key)
}

// ---- issues ----

// storiesOfIssue returns the active stories an issue is linked to, with criteria.
func (s *Service) storiesOfIssue(ctx context.Context, is Issue) ([]StoryRef, error) {
	p := reqProject{id: is.projectID, key: is.ProjectKey}
	list, err := s.loadStories(ctx, p, `s.id IN (SELECT story_id FROM req_story_issues WHERE issue_id = ?)`, []any{is.ID}, nil, nil)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(list))
	for i, st := range list {
		ids[i] = st.id
	}
	crit, err := s.loadCriteria(ctx, p, ids)
	if err != nil {
		return nil, err
	}
	out := make([]StoryRef, len(list))
	for i, st := range list {
		c := crit[st.id]
		if c == nil {
			c = []Criterion{}
		}
		out[i] = StoryRef{Key: st.Key, Title: st.Title, Body: st.Body, PersonaKey: st.PersonaKey, IWant: st.IWant, SoThat: st.SoThat, StepKeys: st.StepKeys, Criteria: c}
	}
	return out, nil
}

const userRefCols = `u.id, u.kind, u.username, u.display_name, u.avatar_color`

func userRefDest(u *UserRef) []any {
	return []any{&u.ID, &u.Kind, &u.Username, &u.DisplayName, &u.AvatarColor}
}
