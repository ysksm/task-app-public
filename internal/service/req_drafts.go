package service

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/events"
)

// Drafts of a story map (docs/08-devcycle.md §5.9). A draft is a project whose
// draft_of names its target. People and agents brainstorm in the draft (add,
// rename, move, archive freely); then a human reviews the import plan and
// imports it into the target in one transaction.
//
// req_draft_links pairs each draft element with the target element it stands
// for, with a content hash of both sides taken when the pair was last in step
// (at copy and after each import). That gives a three-way comparison:
//   - draft unchanged since then            → nothing to do (the target wins)
//   - draft changed, target unchanged       → update the target
//   - draft changed, target changed too     → conflict (needs force)
// Nothing is deleted: elements archived in the draft are archived in the
// target, the same way the rest of the layer works. Issue links are neither
// copied nor imported; target stories keep theirs.

// draftKinds in dependency order (parents first). Archives run in reverse.
var draftKinds = []string{"persona", "activity", "step", "job", "release", "story", "criterion"}

var draftKindLabel = map[string]string{
	"persona": "ペルソナ", "activity": "活動", "step": "手順", "job": "ジョブ",
	"release": "リリース", "story": "ストーリー", "criterion": "受け入れ条件",
}

// DraftInput creates a draft of a project.
type DraftInput struct {
	// Key of the draft project; default: the target key + "D" + a number.
	Key  string `json:"key"`
	Name string `json:"name"`
	// From is copy (start from the target's current map, the default) or empty.
	From string `json:"from"`
	// RequirementsMode of the draft; default quick (brainstorming).
	RequirementsMode string `json:"requirements_mode"`
}

// ImportChange is one changed field of an element.
type ImportChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// ImportItem is one thing the import will do in the target.
type ImportItem struct {
	Kind      string         `json:"kind"`
	Action    string         `json:"action"` // create | update | archive | restore
	DraftKey  *string        `json:"draft_key"`
	TargetKey *string        `json:"target_key"` // null for create: the key is given on import
	Label     string         `json:"label"`
	Changes   []ImportChange `json:"changes"`
	// Conflict: the target element changed since the draft last matched it.
	Conflict bool `json:"conflict"`
}

// ImportRef names an element.
type ImportRef struct {
	Kind  string `json:"kind"`
	Key   string `json:"key"`
	Label string `json:"label"`
}

// ImportPlan is what importing a draft would do.
type ImportPlan struct {
	DraftKey  string       `json:"draft_key"`
	TargetKey string       `json:"target_key"`
	Items     []ImportItem `json:"items"`
	// TargetOnly are active target elements the draft knows nothing about
	// (added in the target after the copy). They are kept as they are.
	TargetOnly []ImportRef    `json:"target_only"`
	Counts     map[string]int `json:"counts"` // create, update, archive, restore
	Conflicts  int            `json:"conflicts"`
	Warnings   []string       `json:"warnings"`
}

// ImportResult is what an import did.
type ImportResult struct {
	Plan     ImportPlan `json:"plan"`
	Created  []string   `json:"created"` // target keys given to new elements
	Warnings []string   `json:"warnings"`
}

// ---- snapshots ----

type dRow struct {
	kind     string
	id       string
	num      int
	aNum     int    // steps: the activity's number (for the key)
	parent   string // steps: activity id; criteria: story id
	fields   []string
	values   []string
	refs     map[string][]string // persona | activities | steps | release
	position int
	archived bool
	label    string
}

func (r *dRow) field(name string) string {
	for i, f := range r.fields {
		if f == name {
			return r.values[i]
		}
	}
	return ""
}

func (r *dRow) key(projectKey string) string {
	switch r.kind {
	case "persona":
		return fmtKey(projectKey, "P", r.num)
	case "activity":
		return fmtKey(projectKey, "A", r.num)
	case "step":
		return fmtStepKey(projectKey, r.aNum, r.num)
	case "job":
		return fmtKey(projectKey, "J", r.num)
	case "release":
		return fmtKey(projectKey, "RL", r.num)
	case "story":
		return fmtKey(projectKey, "ST", r.num)
	default:
		return fmtKey(projectKey, "AC", r.num)
	}
}

// hash is the content of the row in its own project's ids.
func (r *dRow) hash() string {
	h := sha1.New()
	fmt.Fprintf(h, "%s|%s|%d|%v", r.kind, r.parent, r.position, r.archived)
	for i, f := range r.fields {
		fmt.Fprintf(h, "|%s=%s", f, r.values[i])
	}
	names := make([]string, 0, len(r.refs))
	for n := range r.refs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		ids := append([]string(nil), r.refs[n]...)
		sort.Strings(ids)
		fmt.Fprintf(h, "|%s:%s", n, strings.Join(ids, ","))
	}
	return hex.EncodeToString(h.Sum(nil))
}

type snapshot struct {
	key   string
	rows  map[string]map[string]*dRow // kind → id → row
	order map[string][]string         // kind → ids in map order
}

func (sn *snapshot) add(r *dRow) {
	if sn.rows[r.kind] == nil {
		sn.rows[r.kind] = map[string]*dRow{}
	}
	sn.rows[r.kind][r.id] = r
	sn.order[r.kind] = append(sn.order[r.kind], r.id)
}

func (sn *snapshot) get(kind, id string) *dRow {
	return sn.rows[kind][id]
}

func loadSnapshot(ctx context.Context, q queryer, projectID, projectKey string) (*snapshot, error) {
	sn := &snapshot{key: projectKey, rows: map[string]map[string]*dRow{}, order: map[string][]string{}}
	scan := func(query string, fn func(rows *sql.Rows) (*dRow, error)) error {
		rows, err := q.QueryContext(ctx, query, projectID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			r, err := fn(rows)
			if err != nil {
				return err
			}
			sn.add(r)
		}
		return rows.Err()
	}
	ref := func(v sql.NullString) []string {
		if v.Valid {
			return []string{v.String}
		}
		return nil
	}
	if err := scan(`SELECT id, num, name, kind, description, position, archived_at IS NOT NULL FROM req_personas WHERE project_id = ? ORDER BY position, num`, func(rows *sql.Rows) (*dRow, error) {
		r := &dRow{kind: "persona", fields: []string{"name", "kind", "description"}, values: make([]string, 3)}
		err := rows.Scan(&r.id, &r.num, &r.values[0], &r.values[1], &r.values[2], &r.position, &r.archived)
		r.label = r.values[0]
		return r, err
	}); err != nil {
		return nil, err
	}
	if err := scan(`SELECT id, num, persona_id, name, description, position, archived_at IS NOT NULL FROM req_activities WHERE project_id = ? ORDER BY position, num`, func(rows *sql.Rows) (*dRow, error) {
		r := &dRow{kind: "activity", fields: []string{"name", "description"}, values: make([]string, 2)}
		var persona sql.NullString
		err := rows.Scan(&r.id, &r.num, &persona, &r.values[0], &r.values[1], &r.position, &r.archived)
		r.refs = map[string][]string{"persona": ref(persona)}
		r.label = r.values[0]
		return r, err
	}); err != nil {
		return nil, err
	}
	if err := scan(`SELECT st.id, st.num, a.num, st.activity_id, st.name, st.description, st.position, st.archived_at IS NOT NULL OR a.archived_at IS NOT NULL
		FROM req_steps st JOIN req_activities a ON a.id = st.activity_id WHERE st.project_id = ? ORDER BY a.position, a.num, st.position, st.num`, func(rows *sql.Rows) (*dRow, error) {
		r := &dRow{kind: "step", fields: []string{"name", "description"}, values: make([]string, 2)}
		err := rows.Scan(&r.id, &r.num, &r.aNum, &r.parent, &r.values[0], &r.values[1], &r.position, &r.archived)
		r.label = r.values[0]
		return r, err
	}); err != nil {
		return nil, err
	}
	if err := scan(`SELECT id, num, persona_id, situation, motivation, outcome, position, archived_at IS NOT NULL FROM req_jobs WHERE project_id = ? ORDER BY position, num`, func(rows *sql.Rows) (*dRow, error) {
		r := &dRow{kind: "job", fields: []string{"situation", "motivation", "outcome"}, values: make([]string, 3)}
		var persona sql.NullString
		err := rows.Scan(&r.id, &r.num, &persona, &r.values[0], &r.values[1], &r.values[2], &r.position, &r.archived)
		r.refs = map[string][]string{"persona": ref(persona), "activities": nil}
		r.label = r.values[1]
		return r, err
	}); err != nil {
		return nil, err
	}
	if err := scan(`SELECT id, num, name, goal, status, metric, position FROM req_releases WHERE project_id = ? ORDER BY position, num`, func(rows *sql.Rows) (*dRow, error) {
		r := &dRow{kind: "release", fields: []string{"name", "goal", "status", "metric"}, values: make([]string, 4)}
		err := rows.Scan(&r.id, &r.num, &r.values[0], &r.values[1], &r.values[2], &r.values[3], &r.position)
		r.label = r.values[0]
		return r, err
	}); err != nil {
		return nil, err
	}
	releaseByName := map[string]string{}
	for _, id := range sn.order["release"] {
		releaseByName[sn.rows["release"][id].field("name")] = id
	}
	if err := scan(`SELECT id, num, title, body, persona_id, i_want, so_that, release, position, archived_at IS NOT NULL FROM req_stories WHERE project_id = ? ORDER BY position, num`, func(rows *sql.Rows) (*dRow, error) {
		// The release is a reference to the slice (by id), so renaming a slice
		// does not count as changing every story in it.
		r := &dRow{kind: "story", fields: []string{"title", "body", "i_want", "so_that"}, values: make([]string, 4)}
		var persona sql.NullString
		var release string
		err := rows.Scan(&r.id, &r.num, &r.values[0], &r.values[1], &persona, &r.values[2], &r.values[3], &release, &r.position, &r.archived)
		r.refs = map[string][]string{"persona": ref(persona), "steps": nil, "release": nil}
		if release != "" {
			if rid, ok := releaseByName[release]; ok {
				r.refs["release"] = []string{rid}
			} else {
				r.fields, r.values = append(r.fields, "release_name"), append(r.values, release)
			}
		}
		r.label = r.values[0]
		return r, err
	}); err != nil {
		return nil, err
	}
	if err := scan(`SELECT id, num, story_id, given_text, when_text, then_text, position, archived_at IS NOT NULL FROM req_criteria WHERE project_id = ? ORDER BY position, num`, func(rows *sql.Rows) (*dRow, error) {
		r := &dRow{kind: "criterion", fields: []string{"given", "when", "then"}, values: make([]string, 3)}
		err := rows.Scan(&r.id, &r.num, &r.parent, &r.values[0], &r.values[1], &r.values[2], &r.position, &r.archived)
		r.label = r.values[2]
		return r, err
	}); err != nil {
		return nil, err
	}
	// Many-to-many references, to active elements only (what the map shows).
	pairs := func(query, kind, refName string) error {
		rows, err := q.QueryContext(ctx, query, projectID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var owner, other string
			if err := rows.Scan(&owner, &other); err != nil {
				return err
			}
			if r := sn.get(kind, owner); r != nil {
				r.refs[refName] = append(r.refs[refName], other)
			}
		}
		return rows.Err()
	}
	if err := pairs(`SELECT ja.job_id, ja.activity_id FROM req_job_activities ja JOIN req_activities a ON a.id = ja.activity_id
		WHERE a.project_id = ? AND a.archived_at IS NULL`, "job", "activities"); err != nil {
		return nil, err
	}
	if err := pairs(`SELECT ss.story_id, ss.step_id FROM req_story_steps ss JOIN req_steps st ON st.id = ss.step_id JOIN req_activities a ON a.id = st.activity_id
		WHERE st.project_id = ? AND st.archived_at IS NULL AND a.archived_at IS NULL`, "story", "steps"); err != nil {
		return nil, err
	}
	return sn, nil
}

// ---- links ----

type draftLink struct {
	targetID, draftHash, targetHash string
}

type draftLinks struct {
	byDraft  map[string]map[string]draftLink // kind → draft id → link
	byTarget map[string]map[string]string    // kind → target id → draft id
}

func loadDraftLinks(ctx context.Context, q queryer, draftID string) (*draftLinks, error) {
	l := &draftLinks{byDraft: map[string]map[string]draftLink{}, byTarget: map[string]map[string]string{}}
	for _, k := range draftKinds {
		l.byDraft[k], l.byTarget[k] = map[string]draftLink{}, map[string]string{}
	}
	rows, err := q.QueryContext(ctx, `SELECT kind, draft_id, target_id, draft_hash, target_hash FROM req_draft_links WHERE draft_project_id = ?`, draftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, d string
		var x draftLink
		if err := rows.Scan(&k, &d, &x.targetID, &x.draftHash, &x.targetHash); err != nil {
			return nil, err
		}
		if l.byDraft[k] == nil {
			continue
		}
		l.byDraft[k][d] = x
		l.byTarget[k][x.targetID] = d
	}
	return l, rows.Err()
}

// ---- projects ----

type draftPair struct {
	draft, target reqProject
}

func (s *Service) draftPair(ctx context.Context, q queryer, draftKey string) (draftPair, Actor, error) {
	d, actor, err := s.reqProject(ctx, draftKey)
	if err != nil {
		return draftPair{}, actor, err
	}
	var t reqProject
	err = q.QueryRowContext(ctx, `SELECT t.id, t.key, t.requirements_mode FROM projects d JOIN projects t ON t.id = d.draft_of WHERE d.id = ?`, d.id).Scan(&t.id, &t.key, &t.mode)
	if errors.Is(err, sql.ErrNoRows) {
		return draftPair{}, actor, Errorf(CodeInvalidArgument, "project %s is not a draft", d.key)
	}
	return draftPair{draft: d, target: t}, actor, err
}

// draftKey picks a free key: TAPP → TAPPD1, TAPPD2, ... within 10 characters.
func draftKeyFor(ctx context.Context, q queryer, target string) (string, error) {
	for n := 1; n < 1000; n++ {
		suffix := fmt.Sprintf("D%d", n)
		base := target
		if len(base)+len(suffix) > 10 {
			base = base[:10-len(suffix)]
		}
		k := base + suffix
		var c int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE key = ?`, k).Scan(&c); err != nil {
			return "", err
		}
		if c == 0 {
			return k, nil
		}
	}
	return "", Errorf(CodeConflict, "no free draft key for %s", target)
}

// ListDrafts returns the active drafts of a project, newest first.
func (s *Service) ListDrafts(ctx context.Context, targetKey string) ([]Project, error) {
	t, err := s.GetProject(ctx, targetKey)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, projectSelect+` WHERE p.draft_of = ? AND p.archived_at IS NULL ORDER BY p.created_at DESC`, t.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateDraft makes a draft project of targetKey, by default a copy of its map.
func (s *Service) CreateDraft(ctx context.Context, targetKey string, in DraftInput) (Project, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return Project{}, err
	}
	t, err := s.GetProject(ctx, targetKey)
	if err != nil {
		return Project{}, err
	}
	if t.DraftOf != nil {
		return Project{}, Errorf(CodeInvalidArgument, "project %s is itself a draft; make the draft from %s", t.Key, *t.DraftOf)
	}
	if in.From == "" {
		in.From = "copy"
	}
	if in.From != "copy" && in.From != "empty" {
		return Project{}, Errorf(CodeInvalidArgument, "from must be copy or empty")
	}
	if in.RequirementsMode == "" {
		in.RequirementsMode = domain.ReqModeQuick
	}
	if !domain.ValidRequirementsMode(in.RequirementsMode) {
		return Project{}, Errorf(CodeInvalidArgument, "requirements_mode must be structured or quick")
	}
	var key string
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		key = strings.ToUpper(strings.TrimSpace(in.Key))
		if key == "" {
			if key, err = draftKeyFor(ctx, tx, t.Key); err != nil {
				return err
			}
		} else if err := domain.ValidateProjectKey(key); err != nil {
			return Errorf(CodeInvalidArgument, "%s", err.Error())
		}
		name := strings.TrimSpace(in.Name)
		if name == "" {
			name = t.Name + "（下書き）"
		}
		id, err := s.insertProject(ctx, tx, actor, CreateProjectInput{
			Key: key, Name: name, Description: "下書き: " + t.Key + " のストーリーマップを試す場所",
			Tier: domain.TierSide, RequirementsMode: in.RequirementsMode,
		}, &t.ID)
		if err != nil {
			return err
		}
		if in.From == "copy" {
			return s.copyMap(ctx, tx, reqProject{id: t.ID, key: t.Key, mode: t.RequirementsMode}, reqProject{id: id, key: key, mode: in.RequirementsMode})
		}
		return nil
	})
	if err != nil {
		return Project{}, err
	}
	s.bus.Publish(events.Event{Type: "project.created", ProjectKey: key, ActorID: actor.User.ID})
	return s.GetProject(ctx, key)
}

// copyMap copies the target's active elements into an empty draft, keeping
// their numbers (TAPP-ST3 becomes TAPPD1-ST3) and links them pairwise.
func (s *Service) copyMap(ctx context.Context, tx *sql.Tx, from, to reqProject) error {
	src, err := loadSnapshot(ctx, tx, from.id, from.key)
	if err != nil {
		return err
	}
	now := s.timestamp()
	ids := map[string]string{} // source id → draft id
	pairs := map[string][][2]string{}
	m := func(id string) any {
		if v, ok := ids[id]; ok {
			return v
		}
		return nil
	}
	first := func(r *dRow, ref string) any {
		if v := r.refs[ref]; len(v) > 0 {
			return m(v[0])
		}
		return nil
	}
	for _, k := range draftKinds {
		for _, sid := range src.order[k] {
			r := src.get(k, sid)
			if r.archived {
				continue
			}
			if k == "criterion" && ids[r.parent] == "" {
				continue // criterion of an archived story
			}
			id := newID()
			var err error
			switch k {
			case "persona":
				_, err = tx.ExecContext(ctx, `INSERT INTO req_personas (id, project_id, num, name, kind, description, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
					id, to.id, r.num, r.field("name"), r.field("kind"), r.field("description"), r.position, now, now)
			case "activity":
				_, err = tx.ExecContext(ctx, `INSERT INTO req_activities (id, project_id, num, persona_id, name, description, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
					id, to.id, r.num, first(r, "persona"), r.field("name"), r.field("description"), r.position, now, now)
			case "step":
				_, err = tx.ExecContext(ctx, `INSERT INTO req_steps (id, project_id, activity_id, num, name, description, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
					id, to.id, ids[r.parent], r.num, r.field("name"), r.field("description"), r.position, now, now)
			case "job":
				_, err = tx.ExecContext(ctx, `INSERT INTO req_jobs (id, project_id, num, persona_id, situation, motivation, outcome, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
					id, to.id, r.num, first(r, "persona"), r.field("situation"), r.field("motivation"), r.field("outcome"), r.position, now, now)
				for _, a := range r.refs["activities"] {
					if err == nil && ids[a] != "" {
						_, err = tx.ExecContext(ctx, `INSERT INTO req_job_activities (job_id, activity_id) VALUES (?, ?)`, id, ids[a])
					}
				}
			case "release":
				_, err = tx.ExecContext(ctx, `INSERT INTO req_releases (id, project_id, num, name, goal, status, metric, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
					id, to.id, r.num, r.field("name"), r.field("goal"), r.field("status"), r.field("metric"), r.position, now, now)
			case "story":
				_, err = tx.ExecContext(ctx, `INSERT INTO req_stories (id, project_id, num, title, body, persona_id, i_want, so_that, release, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
					id, to.id, r.num, r.field("title"), r.field("body"), first(r, "persona"), r.field("i_want"), r.field("so_that"), storyRelease(src, r), r.position, now, now)
				for _, st := range r.refs["steps"] {
					if err == nil && ids[st] != "" {
						_, err = tx.ExecContext(ctx, `INSERT INTO req_story_steps (story_id, step_id) VALUES (?, ?)`, id, ids[st])
					}
				}
			case "criterion":
				_, err = tx.ExecContext(ctx, `INSERT INTO req_criteria (id, project_id, story_id, num, given_text, when_text, then_text, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
					id, to.id, ids[r.parent], r.num, r.field("given"), r.field("when"), r.field("then"), r.position, now, now)
			}
			if err != nil {
				return err
			}
			ids[sid] = id
			pairs[k] = append(pairs[k], [2]string{id, sid})
		}
	}
	// Counters continue from the target's, so a new draft element never reuses
	// a number the target has used (keys stay comparable).
	if _, err := tx.ExecContext(ctx, `INSERT INTO req_counters (project_id, kind, next)
		SELECT ?, kind, next FROM req_counters WHERE project_id = ? AND kind NOT LIKE 'step:%'`, to.id, from.id); err != nil {
		return err
	}
	for _, p := range pairs["activity"] {
		dst, src := p[0], p[1]
		if _, err := tx.ExecContext(ctx, `INSERT INTO req_counters (project_id, kind, next)
			SELECT ?, 'step:' || ?, next FROM req_counters WHERE project_id = ? AND kind = 'step:' || ?`, to.id, dst, from.id, src); err != nil {
			return err
		}
	}
	return s.writeLinks(ctx, tx, to, from, pairs, nil)
}

// storyRelease is the release name a story row points at.
func storyRelease(sn *snapshot, r *dRow) string {
	if v := r.refs["release"]; len(v) > 0 {
		if rel := sn.get("release", v[0]); rel != nil {
			return rel.field("name")
		}
	}
	return r.field("release_name")
}

// writeLinks replaces the draft's links with the given pairs. A pair in keep
// (an element the import did not touch, still paired the same way) keeps its
// old hashes: refreshing them would forget that the target changed it, and a
// later draft edit would then overwrite that change without a conflict.
// Every other pair gets fresh hashes of both sides.
func (s *Service) writeLinks(ctx context.Context, tx *sql.Tx, draft, target reqProject, pairs map[string][][2]string, keep map[string]map[string]draftLink) error {
	ds, err := loadSnapshot(ctx, tx, draft.id, draft.key)
	if err != nil {
		return err
	}
	ts, err := loadSnapshot(ctx, tx, target.id, target.key)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM req_draft_links WHERE draft_project_id = ?`, draft.id); err != nil {
		return err
	}
	for k, ps := range pairs {
		for _, p := range ps {
			d, t := ds.get(k, p[0]), ts.get(k, p[1])
			if d == nil || t == nil {
				continue
			}
			dh, th := d.hash(), t.hash()
			if old, ok := keep[k][d.id]; ok && old.targetID == t.id {
				dh, th = old.draftHash, old.targetHash
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO req_draft_links (draft_project_id, kind, draft_id, target_id, draft_hash, target_hash) VALUES (?,?,?,?,?,?)`,
				draft.id, k, d.id, t.id, dh, th); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---- plan ----

type planner struct {
	pair   draftPair
	ds, ts *snapshot
	links  *draftLinks
	// releaseMatch pairs an unlinked draft slice with a target slice of the same name.
	releaseMatch map[string]string
}

func (pl *planner) targetOf(kind, draftID string) (string, bool) {
	if l, ok := pl.links.byDraft[kind][draftID]; ok {
		return l.targetID, true
	}
	if kind == "release" {
		if t, ok := pl.releaseMatch[draftID]; ok {
			return t, true
		}
	}
	return "", false
}

// refLabel names a referenced element for the plan: the target's name if it
// is linked, otherwise the draft's (it will be created).
func (pl *planner) refLabel(kind string, draftIDs []string) string {
	names := []string{}
	for _, id := range draftIDs {
		if t, ok := pl.targetOf(kind, id); ok {
			if r := pl.ts.get(kind, t); r != nil {
				names = append(names, r.label)
				continue
			}
		}
		if r := pl.ds.get(kind, id); r != nil {
			names = append(names, r.label)
		}
	}
	sort.Strings(names)
	return strings.Join(names, "、")
}

func (pl *planner) targetRefLabel(kind string, ids []string) string {
	names := []string{}
	for _, id := range ids {
		if r := pl.ts.get(kind, id); r != nil {
			names = append(names, r.label)
		}
	}
	sort.Strings(names)
	return strings.Join(names, "、")
}

var refKind = map[string]string{"persona": "persona", "activities": "activity", "steps": "step", "release": "release"}

var fieldLabel = map[string]string{
	"name": "名前", "kind": "種類", "description": "説明", "situation": "状況", "motivation": "したいこと",
	"outcome": "そうすれば", "goal": "目的", "status": "状態", "metric": "確かめ方", "title": "タイトル",
	"body": "本文", "i_want": "したいこと", "so_that": "そうすれば", "given": "前提", "when": "操作", "then": "結果",
	"persona": "誰の", "activities": "活動", "steps": "手順", "release": "リリース", "release_name": "リリース", "position": "並び",
}

// translated reports whether the draft row's references, in target ids, equal the target row's.
func (pl *planner) refsEqual(d, t *dRow, name string) bool {
	k := refKind[name]
	want := []string{}
	for _, id := range d.refs[name] {
		if tid, ok := pl.targetOf(k, id); ok {
			want = append(want, tid)
		} else {
			want = append(want, "new:"+id)
		}
	}
	got := append([]string(nil), t.refs[name]...)
	sort.Strings(want)
	sort.Strings(got)
	return strings.Join(want, ",") == strings.Join(got, ",")
}

func (pl *planner) diff(d, t *dRow) []ImportChange {
	ch := []ImportChange{}
	for i, f := range d.fields {
		if f == "release_name" {
			continue
		}
		if t == nil {
			if d.values[i] != "" {
				ch = append(ch, ImportChange{Field: fieldLabel[f], To: d.values[i]})
			}
			continue
		}
		if d.values[i] != t.field(f) {
			ch = append(ch, ImportChange{Field: fieldLabel[f], From: t.field(f), To: d.values[i]})
		}
	}
	names := make([]string, 0, len(d.refs))
	for n := range d.refs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if t == nil {
			if len(d.refs[n]) > 0 {
				ch = append(ch, ImportChange{Field: fieldLabel[n], To: pl.refLabel(refKind[n], d.refs[n])})
			}
			continue
		}
		if !pl.refsEqual(d, t, n) {
			ch = append(ch, ImportChange{Field: fieldLabel[n], From: pl.targetRefLabel(refKind[n], t.refs[n]), To: pl.refLabel(refKind[n], d.refs[n])})
		}
	}
	if t != nil && d.position != t.position {
		ch = append(ch, ImportChange{Field: fieldLabel["position"], From: fmt.Sprint(t.position), To: fmt.Sprint(d.position)})
	}
	return ch
}

// skipped: elements that do not reach the target because their parent was archived in the draft.
func (pl *planner) orphan(d *dRow) bool {
	switch d.kind {
	case "criterion":
		st := pl.ds.get("story", d.parent)
		return st == nil || st.archived
	case "step":
		a := pl.ds.get("activity", d.parent)
		return a == nil || a.archived
	}
	return false
}

func strp(v string) *string { return &v }

func (pl *planner) plan() ImportPlan {
	plan := ImportPlan{DraftKey: pl.pair.draft.key, TargetKey: pl.pair.target.key, Items: []ImportItem{}, TargetOnly: []ImportRef{},
		Counts: map[string]int{"create": 0, "update": 0, "archive": 0, "restore": 0}, Warnings: []string{}}
	// Unlinked draft slices whose name the target already has are matched by name.
	pl.releaseMatch = map[string]string{}
	targetByName := map[string]string{}
	for _, id := range pl.ts.order["release"] {
		if _, linked := pl.links.byTarget["release"][id]; !linked {
			targetByName[pl.ts.get("release", id).field("name")] = id
		}
	}
	for _, id := range pl.ds.order["release"] {
		if _, linked := pl.links.byDraft["release"][id]; !linked {
			if t, ok := targetByName[pl.ds.get("release", id).field("name")]; ok {
				pl.releaseMatch[id] = t
			}
		}
	}
	add := func(it ImportItem) {
		plan.Items = append(plan.Items, it)
		plan.Counts[it.Action]++
		if it.Conflict {
			plan.Conflicts++
		}
	}
	noSteps := 0
	for _, k := range draftKinds {
		for _, id := range pl.ds.order[k] {
			d := pl.ds.get(k, id)
			dk := strp(d.key(pl.ds.key))
			l, linked := pl.links.byDraft[k][id]
			var t *dRow
			if linked {
				t = pl.ts.get(k, l.targetID)
			} else if tid, ok := pl.releaseMatch[id]; ok {
				t = pl.ts.get(k, tid)
			}
			if d.archived || pl.orphan(d) {
				// Archived in the draft. Steps of an archived activity and criteria
				// of an archived story go along with their parent.
				if pl.orphan(d) && (k == "step" || k == "criterion") {
					continue
				}
				if linked && t != nil && !t.archived {
					add(ImportItem{Kind: k, Action: "archive", DraftKey: dk, TargetKey: strp(t.key(pl.ts.key)), Label: t.label, Changes: []ImportChange{}, Conflict: t.hash() != l.targetHash})
				}
				continue
			}
			if !linked && t == nil {
				add(ImportItem{Kind: k, Action: "create", DraftKey: dk, Label: d.label, Changes: pl.diff(d, nil)})
				if k == "story" && len(d.refs["steps"]) == 0 {
					noSteps++
				}
				continue
			}
			if !linked { // a slice matched by name
				if ch := pl.diff(d, t); len(ch) > 0 {
					add(ImportItem{Kind: k, Action: "update", DraftKey: dk, TargetKey: strp(t.key(pl.ts.key)), Label: t.label, Changes: ch})
				}
				continue
			}
			draftChanged := d.hash() != l.draftHash
			if t == nil || t.archived {
				// Gone from the target since the copy. Only a draft that kept
				// working on it brings it back.
				if draftChanged {
					action := "restore"
					var tk *string
					if t == nil {
						action = "create"
					} else {
						tk = strp(t.key(pl.ts.key))
					}
					add(ImportItem{Kind: k, Action: action, DraftKey: dk, TargetKey: tk, Label: d.label, Changes: pl.diff(d, t), Conflict: true})
				}
				continue
			}
			if !draftChanged {
				continue
			}
			ch := pl.diff(d, t)
			if len(ch) == 0 {
				continue
			}
			add(ImportItem{Kind: k, Action: "update", DraftKey: dk, TargetKey: strp(t.key(pl.ts.key)), Label: t.label, Changes: ch, Conflict: t.hash() != l.targetHash})
		}
		// Linked elements whose draft row is gone (a deleted slice), in key order.
		gone := []string{}
		for did := range pl.links.byDraft[k] {
			if pl.ds.get(k, did) == nil {
				gone = append(gone, did)
			}
		}
		sort.Slice(gone, func(i, j int) bool {
			a, b := pl.ts.get(k, pl.links.byDraft[k][gone[i]].targetID), pl.ts.get(k, pl.links.byDraft[k][gone[j]].targetID)
			return a != nil && b != nil && a.num < b.num
		})
		for _, did := range gone {
			l := pl.links.byDraft[k][did]
			if t := pl.ts.get(k, l.targetID); t != nil && !t.archived {
				add(ImportItem{Kind: k, Action: "archive", TargetKey: strp(t.key(pl.ts.key)), Label: t.label, Changes: []ImportChange{}, Conflict: t.hash() != l.targetHash})
			}
		}
		for _, tid := range pl.ts.order[k] {
			t := pl.ts.get(k, tid)
			if t.archived {
				continue
			}
			if _, linked := pl.links.byTarget[k][tid]; linked {
				continue
			}
			matched := false
			for _, m := range pl.releaseMatch {
				if m == tid {
					matched = true
				}
			}
			if !matched {
				plan.TargetOnly = append(plan.TargetOnly, ImportRef{Kind: k, Key: t.key(pl.ts.key), Label: t.label})
			}
		}
	}
	// A slice renamed in the draft to the name of a slice only the target has
	// would clash on import; say so before the button is pressed.
	onlyNames := map[string]bool{}
	for _, r := range plan.TargetOnly {
		if r.Kind == "release" {
			onlyNames[r.Label] = true
		}
	}
	for _, it := range plan.Items {
		if it.Kind != "release" || it.Action == "archive" {
			continue
		}
		for _, c := range it.Changes {
			if c.Field == fieldLabel["name"] && onlyNames[c.To] {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("リリース「%s」は %s にもう別にあるので、この名前には変えられません。下書きで別の名前にしてください", c.To, pl.pair.target.key))
			}
		}
	}
	if noSteps > 0 && pl.pair.target.mode == domain.ReqModeStructured {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("新しいストーリーのうち %d 件は手順がありません。%s では「未整理」に入ります", noSteps, pl.pair.target.key))
	}
	if plan.Conflicts > 0 {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("%d 件は %s の側でも変わっています。取り込むと下書きの内容で上書きします", plan.Conflicts, pl.pair.target.key))
	}
	return plan
}

func (s *Service) newPlanner(ctx context.Context, q queryer, pair draftPair) (*planner, error) {
	ds, err := loadSnapshot(ctx, q, pair.draft.id, pair.draft.key)
	if err != nil {
		return nil, err
	}
	ts, err := loadSnapshot(ctx, q, pair.target.id, pair.target.key)
	if err != nil {
		return nil, err
	}
	links, err := loadDraftLinks(ctx, q, pair.draft.id)
	if err != nil {
		return nil, err
	}
	return &planner{pair: pair, ds: ds, ts: ts, links: links}, nil
}

// PlanImport returns what importing the draft into its target would do.
func (s *Service) PlanImport(ctx context.Context, draftKey string) (ImportPlan, error) {
	pair, _, err := s.draftPair(ctx, s.db, draftKey)
	if err != nil {
		return ImportPlan{}, err
	}
	pl, err := s.newPlanner(ctx, s.db, pair)
	if err != nil {
		return ImportPlan{}, err
	}
	return pl.plan(), nil
}

// ImportInput confirms an import.
type ImportInput struct {
	// Force overwrites target elements that changed since the draft matched them.
	Force bool `json:"force"`
}

// ApplyImport imports the draft into its target in one transaction. Only a
// human imports; agents prepare drafts and show the plan.
func (s *Service) ApplyImport(ctx context.Context, draftKey string, in ImportInput) (ImportResult, error) {
	pair, actor, err := s.draftPair(ctx, s.db, draftKey)
	if err != nil {
		return ImportResult{}, err
	}
	if actor.User.Kind != "human" {
		return ImportResult{}, Errorf(CodeForbidden, "a draft is imported by a human; show the plan (get_import_plan) and ask the owner")
	}
	var res ImportResult
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		pl, err := s.newPlanner(ctx, tx, pair)
		if err != nil {
			return err
		}
		plan := pl.plan()
		if plan.Conflicts > 0 && !in.Force {
			return Errorf(CodeConflict, "%d element(s) changed in %s since the draft matched them; review the plan and import with force", plan.Conflicts, pair.target.key)
		}
		ap := &applier{s: s, ctx: ctx, tx: tx, pl: pl, actor: actor, now: s.timestamp(), ids: map[string]map[string]string{}, created: []string{}, warnings: []string{}, tempName: map[string]string{}, touched: map[string]map[string]bool{}}
		if err := ap.run(plan); err != nil {
			return err
		}
		res = ImportResult{Plan: plan, Created: ap.created, Warnings: ap.warnings}
		return s.writeLinks(ctx, tx, pair.draft, pair.target, ap.pairs(), ap.untouched())
	})
	if err != nil {
		return ImportResult{}, err
	}
	s.reqChanged(pair.target, actor)
	s.reqChanged(pair.draft, actor)
	return res, nil
}

// ---- apply ----

type applier struct {
	s        *Service
	ctx      context.Context
	tx       *sql.Tx
	pl       *planner
	actor    Actor
	now      string
	ids      map[string]map[string]string // kind → draft id → target id, for elements created now
	tempName map[string]string            // renamed slices: target id → temporary name
	touched  map[string]map[string]bool   // kind → target ids written by this import
	created  []string
	warnings []string
}

func (ap *applier) target(kind, draftID string) (string, bool) {
	if t, ok := ap.ids[kind][draftID]; ok {
		return t, true
	}
	return ap.pl.targetOf(kind, draftID)
}

func (ap *applier) ref(d *dRow, name string) any {
	if v := d.refs[name]; len(v) > 0 {
		if t, ok := ap.target(refKind[name], v[0]); ok {
			return t
		}
	}
	return nil
}

func (ap *applier) refs(d *dRow, name string) []string {
	out := []string{}
	for _, id := range d.refs[name] {
		if t, ok := ap.target(refKind[name], id); ok {
			out = append(out, t)
		}
	}
	return out
}

func (ap *applier) change(key string, storyID *string, action string, detail map[string]any) error {
	if detail == nil {
		detail = map[string]any{}
	}
	detail["from_draft"] = ap.pl.pair.draft.key
	return ap.s.reqChange(ap.ctx, ap.tx, ap.pl.pair.target, ap.actor, key, storyID, action, detail)
}

func changeDetail(ch []ImportChange) map[string]any {
	m := map[string]any{}
	for _, c := range ch {
		m[c.Field] = c.To
	}
	return m
}

// untouched are the existing links of elements this import did not write.
func (ap *applier) untouched() map[string]map[string]draftLink {
	out := map[string]map[string]draftLink{}
	for k, links := range ap.pl.links.byDraft {
		out[k] = map[string]draftLink{}
		for did, l := range links {
			if !ap.touched[k][l.targetID] {
				out[k][did] = l
			}
		}
	}
	return out
}

// pairs is every draft↔target pair after the import, for writeLinks.
func (ap *applier) pairs() map[string][][2]string {
	out := map[string][][2]string{}
	for _, k := range draftKinds {
		for _, did := range ap.pl.ds.order[k] {
			if t, ok := ap.target(k, did); ok {
				out[k] = append(out[k], [2]string{did, t})
			}
		}
	}
	return out
}

func (ap *applier) run(plan ImportPlan) error {
	ctx := ap.ctx
	tp := ap.pl.pair.target
	byDraftKey := map[string]*dRow{}
	for _, k := range draftKinds {
		for _, id := range ap.pl.ds.order[k] {
			d := ap.pl.ds.get(k, id)
			byDraftKey[k+"|"+d.key(ap.pl.ds.key)] = d
		}
	}
	// Renamed slices first get a temporary name, so swapping names never clashes.
	for _, it := range plan.Items {
		if it.Kind == "release" && it.Action != "archive" && it.TargetKey != nil {
			d := byDraftKey["release|"+*it.DraftKey]
			t, _ := ap.target("release", d.id)
			cur := ap.pl.ts.get("release", t)
			if cur == nil || cur.field("name") == d.field("name") {
				continue
			}
			// The slice and its stories (also target-only ones) move to a
			// temporary name; upsert then renames both to the final name.
			tmp := "\x00draft:" + t
			if _, err := ap.tx.ExecContext(ctx, `UPDATE req_releases SET name = ? WHERE id = ?`, tmp, t); err != nil {
				return err
			}
			if _, err := ap.tx.ExecContext(ctx, `UPDATE req_stories SET release = ?, updated_at = ? WHERE project_id = ? AND release = ?`, tmp, ap.now, ap.pl.pair.target.id, cur.field("name")); err != nil {
				return err
			}
			ap.tempName[t] = tmp
		}
	}
	for _, k := range draftKinds {
		for _, it := range plan.Items {
			if it.Kind != k || it.Action == "archive" {
				continue
			}
			d := byDraftKey[k+"|"+*it.DraftKey]
			if err := ap.upsert(ctx, tp, d, it); err != nil {
				return err
			}
		}
	}
	for i := len(draftKinds) - 1; i >= 0; i-- {
		k := draftKinds[i]
		for _, it := range plan.Items {
			if it.Kind != k || it.Action != "archive" {
				continue
			}
			if err := ap.archive(ctx, tp, it); err != nil {
				return err
			}
		}
	}
	return nil
}

func (ap *applier) upsert(ctx context.Context, tp reqProject, d *dRow, it ImportItem) error {
	tx := ap.tx
	tid, exists := ap.target(d.kind, d.id)
	if exists && ap.pl.ts.get(d.kind, tid) == nil {
		exists = false // gone from the target (a deleted slice): create it again
	}
	if !exists {
		tid = newID()
	}
	var num, aNum int
	var err error
	if !exists {
		counter := d.kind
		if d.kind == "step" {
			counter = "step:" + ap.mustTarget("activity", d.parent)
		}
		if num, err = nextReqNum(ctx, tx, tp.id, counter); err != nil {
			return err
		}
	} else {
		num = ap.pl.ts.get(d.kind, tid).num
	}
	now := ap.now
	var storyID *string
	switch d.kind {
	case "persona":
		if !exists {
			_, err = tx.ExecContext(ctx, `INSERT INTO req_personas (id, project_id, num, name, kind, description, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
				tid, tp.id, num, d.field("name"), d.field("kind"), d.field("description"), d.position, now, now)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE req_personas SET name = ?, kind = ?, description = ?, position = ?, archived_at = NULL, updated_at = ? WHERE id = ?`,
				d.field("name"), d.field("kind"), d.field("description"), d.position, now, tid)
		}
	case "activity":
		if !exists {
			_, err = tx.ExecContext(ctx, `INSERT INTO req_activities (id, project_id, num, persona_id, name, description, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
				tid, tp.id, num, ap.ref(d, "persona"), d.field("name"), d.field("description"), d.position, now, now)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE req_activities SET persona_id = ?, name = ?, description = ?, position = ?, archived_at = NULL, updated_at = ? WHERE id = ?`,
				ap.ref(d, "persona"), d.field("name"), d.field("description"), d.position, now, tid)
		}
	case "step":
		aid := ap.mustTarget("activity", d.parent)
		if err = tx.QueryRowContext(ctx, `SELECT num FROM req_activities WHERE id = ?`, aid).Scan(&aNum); err != nil {
			return err
		}
		if !exists {
			_, err = tx.ExecContext(ctx, `INSERT INTO req_steps (id, project_id, activity_id, num, name, description, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
				tid, tp.id, aid, num, d.field("name"), d.field("description"), d.position, now, now)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE req_steps SET name = ?, description = ?, position = ?, archived_at = NULL, updated_at = ? WHERE id = ?`,
				d.field("name"), d.field("description"), d.position, now, tid)
		}
	case "job":
		if !exists {
			_, err = tx.ExecContext(ctx, `INSERT INTO req_jobs (id, project_id, num, persona_id, situation, motivation, outcome, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
				tid, tp.id, num, ap.ref(d, "persona"), d.field("situation"), d.field("motivation"), d.field("outcome"), d.position, now, now)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE req_jobs SET persona_id = ?, situation = ?, motivation = ?, outcome = ?, position = ?, archived_at = NULL, updated_at = ? WHERE id = ?`,
				ap.ref(d, "persona"), d.field("situation"), d.field("motivation"), d.field("outcome"), d.position, now, tid)
		}
		if err == nil {
			err = replacePairs(ctx, tx, `req_job_activities`, "job_id", "activity_id", tid, ap.refs(d, "activities"))
		}
	case "release":
		if !exists {
			_, err = tx.ExecContext(ctx, `INSERT INTO req_releases (id, project_id, num, name, goal, status, metric, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
				tid, tp.id, num, d.field("name"), d.field("goal"), d.field("status"), d.field("metric"), d.position, now, now)
		} else {
			old := ap.pl.ts.get("release", tid).field("name")
			if tmp, ok := ap.tempName[tid]; ok {
				old = tmp
			}
			_, err = tx.ExecContext(ctx, `UPDATE req_releases SET name = ?, goal = ?, status = ?, metric = ?, position = ?, updated_at = ? WHERE id = ?`,
				d.field("name"), d.field("goal"), d.field("status"), d.field("metric"), d.position, now, tid)
			if err == nil && old != d.field("name") {
				// Stories (also target-only ones) follow the rename.
				_, err = tx.ExecContext(ctx, `UPDATE req_stories SET release = ?, updated_at = ? WHERE project_id = ? AND release = ?`, d.field("name"), now, tp.id, old)
			}
		}
		if isUniqueViolation(err) {
			return Errorf(CodeConflict, "release %q already exists in %s", d.field("name"), tp.key)
		}
	case "story":
		release := d.field("release_name")
		if v := d.refs["release"]; len(v) > 0 {
			if rel := ap.pl.ds.get("release", v[0]); rel != nil {
				release = rel.field("name")
			}
		}
		if !exists {
			_, err = tx.ExecContext(ctx, `INSERT INTO req_stories (id, project_id, num, title, body, persona_id, i_want, so_that, release, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
				tid, tp.id, num, d.field("title"), d.field("body"), ap.ref(d, "persona"), d.field("i_want"), d.field("so_that"), release, d.position, now, now)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE req_stories SET title = ?, body = ?, persona_id = ?, i_want = ?, so_that = ?, release = ?, position = ?, archived_at = NULL, updated_at = ? WHERE id = ?`,
				d.field("title"), d.field("body"), ap.ref(d, "persona"), d.field("i_want"), d.field("so_that"), release, d.position, now, tid)
		}
		if err == nil {
			err = setStorySteps(ctx, tx, tid, ap.refs(d, "steps"))
		}
		if err == nil {
			err = ap.s.ensureRelease(ctx, tx, tp, ap.actor, release)
		}
		storyID = &tid
	case "criterion":
		sid := ap.mustTarget("story", d.parent)
		if !exists {
			_, err = tx.ExecContext(ctx, `INSERT INTO req_criteria (id, project_id, story_id, num, given_text, when_text, then_text, position, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
				tid, tp.id, sid, num, d.field("given"), d.field("when"), d.field("then"), d.position, now, now)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE req_criteria SET given_text = ?, when_text = ?, then_text = ?, position = ?, archived_at = NULL, updated_at = ? WHERE id = ?`,
				d.field("given"), d.field("when"), d.field("then"), d.position, now, tid)
		}
		storyID = &sid
	}
	if err != nil {
		return err
	}
	if ap.ids[d.kind] == nil {
		ap.ids[d.kind] = map[string]string{}
	}
	ap.ids[d.kind][d.id] = tid
	ap.touch(d.kind, tid)
	key := (&dRow{kind: d.kind, num: num, aNum: aNum}).key(tp.key)
	action := "updated"
	if !exists {
		action = "created"
		ap.created = append(ap.created, key)
	}
	return ap.change(key, storyID, action, changeDetail(it.Changes))
}

func (ap *applier) touch(kind, targetID string) {
	if ap.touched[kind] == nil {
		ap.touched[kind] = map[string]bool{}
	}
	ap.touched[kind][targetID] = true
}

// mustTarget is the target id of a parent handled earlier in the same import.
func (ap *applier) mustTarget(kind, draftID string) string {
	t, _ := ap.target(kind, draftID)
	return t
}

func (ap *applier) archive(ctx context.Context, tp reqProject, it ImportItem) error {
	tx := ap.tx
	var t *dRow
	for _, id := range ap.pl.ts.order[it.Kind] {
		if r := ap.pl.ts.get(it.Kind, id); r.key(tp.key) == *it.TargetKey {
			t = r
		}
	}
	if t == nil {
		return nil
	}
	now := ap.now
	ap.touch(it.Kind, t.id)
	var storyID *string
	switch it.Kind {
	case "release":
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM req_stories WHERE project_id = ? AND archived_at IS NULL AND release = ?`, tp.id, t.field("name")).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			ap.warnings = append(ap.warnings, fmt.Sprintf("リリース %s（%s）は %s にまだストーリーが %d 件あるので残しました", *it.TargetKey, t.label, tp.key, n))
			return nil
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM req_releases WHERE id = ?`, t.id); err != nil {
			return err
		}
		return ap.change(*it.TargetKey, nil, "archived", nil)
	case "story":
		storyID = &t.id
	case "criterion":
		storyID = &t.parent
	}
	table := map[string]string{"persona": "req_personas", "activity": "req_activities", "step": "req_steps", "job": "req_jobs", "story": "req_stories", "criterion": "req_criteria"}[it.Kind]
	if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET archived_at = ?, updated_at = ? WHERE id = ?`, now, now, t.id); err != nil {
		return err
	}
	if it.Kind == "activity" {
		if _, err := tx.ExecContext(ctx, `UPDATE req_steps SET archived_at = ?, updated_at = ? WHERE activity_id = ? AND archived_at IS NULL`, now, now, t.id); err != nil {
			return err
		}
	}
	return ap.change(*it.TargetKey, storyID, "archived", nil)
}

func replacePairs(ctx context.Context, tx *sql.Tx, table, ownerCol, otherCol, owner string, others []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE `+ownerCol+` = ?`, owner); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, o := range others {
		if seen[o] {
			continue
		}
		seen[o] = true
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+table+` (`+ownerCol+`, `+otherCol+`) VALUES (?, ?)`, owner, o); err != nil {
			return err
		}
	}
	return nil
}
