// Package mcpserver exposes taskapp to AI agents over the Model Context
// Protocol (stdio and Streamable HTTP). Tools call the same service layer as
// the web UI, so hierarchy rules and history apply equally to agents.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/service"
)

const instructions = `taskapp is the task tracker shared by humans and AI agents.

How to work with it:
1. Find your work: list_my_issues (or search_issues). Read details with get_issue.
2. When you start, transition_issue to "In Progress".
3. At every meaningful step call record_checkpoint: what you did (summary), what you decided (decision),
   and WHY (rationale = 判断基準). Mark confidence "low" when a human should double-check.
4. When you need a human decision, call ask_question instead of guessing. Pick the answer format that makes
   answering fastest: yes_no / ok_ng for confirmations, single_choice or multi_choice (2-6 options, mark the
   recommended one) for alternatives, text only when free text is really needed. Several questions can go into
   one ticket. Set is_blocking when you cannot continue without the answer, and state your provisional decision.
5. Poll get_question_answers for answers, act on them, then close_question.
6. Break work down with create_issue (tasks can be nested under tasks).
7. When done: transition_issue to "In Review"; for stories/features ready for the human acceptance check, request_acceptance.

Requirements (story map): get_storymap returns the project's mode. In "quick" projects the owner writes stories
loosely (a title, free text, one-line criteria) and leaves them unsorted (未整理). In "structured" projects every
story belongs under a step and every criterion has given, when and then: fill all three, and treat stories
without steps or criteria (gaps) as things to fix. Rows are release slices (release_slices: name, goal, status,
progress); the first slice not shipped is what to build next. Propose a goal for slices that have none (update_release). In either mode, to tidy stories, read get_storymap, propose each change (placement under a step, release,
splitting a criterion) with ask_question as yes_no questions, and apply only the approved ones with update_story.
create_story adds a story; only the title is required. Build the map itself with create_persona and
create_activity (with its steps; create_step adds one later), and fix it with update_persona, update_activity, update_step.
For bigger rethinks (several stories, new activities, renaming slices) brainstorm in a draft instead of the
project itself: create_draft copies the map into a draft project (e.g. TAPPD1, same numbers), work there with the
usual tools, then show get_import_plan to the owner. Only the owner imports a draft (in the web UI).`

// New builds an MCP server. fixed is the actor for stdio sessions; for HTTP
// sessions the actor comes from the bearer token of each request.
func New(svc *service.Service, fixed *service.Actor) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "taskapp", Title: "taskapp", Version: "0.1.0"},
		&mcp.ServerOptions{Instructions: instructions, Capabilities: &mcp.ServerCapabilities{}})
	h := &handlers{svc: svc, fixed: fixed}
	h.register(s)
	return s
}

// HTTPHandler serves MCP over Streamable HTTP, authenticated with taskapp API tokens.
//
// behindProxy turns off the SDK's DNS-rebinding check, which rejects requests
// that reach a loopback address with a non-loopback Host header. A reverse
// proxy on the same machine (tailscale serve, Caddy) sends exactly that. The
// bearer token is still required, and browsers never attach it on their own.
func HTTPHandler(svc *service.Service, behindProxy bool) http.Handler {
	server := New(svc, nil)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{DisableLocalhostProtection: behindProxy})
	verify := func(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		u, err := svc.Authenticate(ctx, token)
		if err != nil {
			return nil, auth.ErrInvalidToken
		}
		// Tokens do not expire by default; the middleware requires a value.
		return &auth.TokenInfo{UserID: u.ID, Expiration: time.Now().Add(24 * time.Hour)}, nil
	}
	return auth.RequireBearerToken(verify, nil)(handler)
}

type handlers struct {
	svc   *service.Service
	fixed *service.Actor
}

// actorCtx attaches the calling agent to ctx.
func (h *handlers) actorCtx(ctx context.Context, req *mcp.CallToolRequest) (context.Context, error) {
	if req != nil && req.Extra != nil && req.Extra.TokenInfo != nil {
		u, err := h.svc.UserByID(ctx, req.Extra.TokenInfo.UserID)
		if err != nil {
			return nil, err
		}
		return service.WithActor(ctx, service.Actor{User: u, Source: domain.SourceMCP}), nil
	}
	if h.fixed != nil {
		return service.WithActor(ctx, *h.fixed), nil
	}
	return nil, errors.New("not authenticated")
}

// tool registers a typed tool whose handler runs as the calling agent and
// returns its result as JSON text.
func tool[In any](s *mcp.Server, h *handlers, name, title, desc string, readOnly bool, fn func(ctx context.Context, in In) (any, error)) {
	t := &mcp.Tool{Name: name, Title: title, Description: desc}
	if readOnly {
		t.Annotations = &mcp.ToolAnnotations{ReadOnlyHint: true}
	}
	mcp.AddTool(s, t, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		ctx, err := h.actorCtx(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		out, err := fn(ctx, in)
		if err != nil {
			// Tool errors are returned to the model so it can correct itself.
			return nil, nil, errors.New(service.AsError(err).Message)
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil, nil
	})
}

// ---- inputs ----

type empty struct{}

type storyMapIn struct {
	Project string `json:"project" jsonschema:"project key, e.g. TAPP"`
}

type criterionIn struct {
	Given string `json:"given,omitempty" jsonschema:"precondition (optional)"`
	When  string `json:"when,omitempty" jsonschema:"action (optional)"`
	Then  string `json:"then" jsonschema:"the expected result; one line is enough"`
}

type criterionPatchIn struct {
	Key   string  `json:"key" jsonschema:"criterion key, e.g. TAPP-AC42"`
	Given *string `json:"given,omitempty"`
	When  *string `json:"when,omitempty"`
	Then  *string `json:"then,omitempty"`
}

type createStoryIn struct {
	Project    string        `json:"project" jsonschema:"project key, e.g. TAPP"`
	Title      string        `json:"title" jsonschema:"one line; nothing else is required"`
	Body       string        `json:"body,omitempty" jsonschema:"free text (Markdown)"`
	Release    string        `json:"release,omitempty" jsonschema:"e.g. R1; empty = not decided"`
	StepKeys   []string      `json:"step_keys,omitempty" jsonschema:"steps to place the story under, e.g. TAPP-A1-2; empty = unsorted (未整理)"`
	Criteria   []criterionIn `json:"criteria,omitempty"`
	PersonaKey string        `json:"persona_key,omitempty" jsonschema:"who the story is for, e.g. TAPP-P1"`
	IWant      string        `json:"i_want,omitempty" jsonschema:"I want to ..."`
	SoThat     string        `json:"so_that,omitempty" jsonschema:"so that ..."`
}
type createPersonaIn struct {
	Project     string `json:"project" jsonschema:"project key, e.g. TAPP"`
	Name        string `json:"name" jsonschema:"e.g. 自分, 新しく入った開発者"`
	Kind        string `json:"kind,omitempty" jsonschema:"human (default) or agent"`
	Description string `json:"description,omitempty" jsonschema:"who they are, what bothers them, what they want"`
}
type updatePersonaIn struct {
	Project     string  `json:"project" jsonschema:"project key, e.g. TAPP"`
	Key         string  `json:"key" jsonschema:"persona key, e.g. TAPP-P1"`
	Name        *string `json:"name,omitempty"`
	Kind        *string `json:"kind,omitempty" jsonschema:"human or agent"`
	Description *string `json:"description,omitempty"`
	Position    *int    `json:"position,omitempty" jsonschema:"0-based order"`
}
type stepIn struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
type createActivityIn struct {
	Project     string   `json:"project" jsonschema:"project key, e.g. TAPP"`
	Name        string   `json:"name" jsonschema:"what the persona does, e.g. 朝の支度; activities run left to right in the order created"`
	PersonaKey  string   `json:"persona_key,omitempty" jsonschema:"whose activity, e.g. TAPP-P1"`
	Description string   `json:"description,omitempty"`
	Steps       []stepIn `json:"steps,omitempty" jsonschema:"its steps in order (the columns stories are placed under)"`
}
type updateActivityIn struct {
	Project     string  `json:"project" jsonschema:"project key, e.g. TAPP"`
	Key         string  `json:"key" jsonschema:"activity key, e.g. TAPP-A2"`
	Name        *string `json:"name,omitempty"`
	PersonaKey  *string `json:"persona_key,omitempty" jsonschema:"\"\" removes the persona"`
	Description *string `json:"description,omitempty"`
	Position    *int    `json:"position,omitempty" jsonschema:"0-based order"`
}
type createStepIn struct {
	Project     string `json:"project" jsonschema:"project key, e.g. TAPP"`
	Activity    string `json:"activity" jsonschema:"activity key, e.g. TAPP-A2; the step is added at the end"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
type updateStepIn struct {
	Project     string  `json:"project" jsonschema:"project key, e.g. TAPP"`
	Key         string  `json:"key" jsonschema:"step key, e.g. TAPP-A2-1"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Position    *int    `json:"position,omitempty" jsonschema:"0-based order within the activity"`
}

type updateStoryIn struct {
	Project        string             `json:"project" jsonschema:"project key, e.g. TAPP"`
	Key            string             `json:"key" jsonschema:"story key, e.g. TAPP-ST12"`
	Title          *string            `json:"title,omitempty"`
	Body           *string            `json:"body,omitempty" jsonschema:"free text (Markdown); replaces the body"`
	PersonaKey     *string            `json:"persona_key,omitempty" jsonschema:"persona key; empty string clears it"`
	IWant          *string            `json:"i_want,omitempty"`
	SoThat         *string            `json:"so_that,omitempty"`
	Release        *string            `json:"release,omitempty"`
	StepKeys       *[]string          `json:"step_keys,omitempty" jsonschema:"replaces the placement; [] moves the story back to unsorted (未整理)"`
	AddCriteria    []criterionIn      `json:"add_criteria,omitempty"`
	UpdateCriteria []criterionPatchIn `json:"update_criteria,omitempty"`
}

type createDraftIn struct {
	Project string `json:"project" jsonschema:"key of the project to draft, e.g. TAPP"`
	From    string `json:"from,omitempty" jsonschema:"copy (default: start from the current map) or empty"`
	Name    string `json:"name,omitempty"`
}

type draftKeyIn struct {
	Draft string `json:"draft" jsonschema:"key of the draft project, e.g. TAPPD1"`
}

type updateReleaseIn struct {
	Project string  `json:"project" jsonschema:"project key, e.g. TAPP"`
	Name    string  `json:"name" jsonschema:"the slice as stories name it, e.g. R1; created (planned) if missing"`
	NewName *string `json:"new_name,omitempty" jsonschema:"rename; the stories follow"`
	Goal    *string `json:"goal,omitempty" jsonschema:"the outcome: what someone can do once this slice ships"`
	Status  *string `json:"status,omitempty" jsonschema:"planned | building | shipped"`
	Metric  *string `json:"metric,omitempty" jsonschema:"optional: how we will know it worked"`
}

type myIssuesIn struct {
	Project       string `json:"project,omitempty" jsonschema:"limit to a project key, e.g. APP"`
	IncludeClosed bool   `json:"include_closed,omitempty" jsonschema:"also return completed and canceled issues"`
}

type searchIn struct {
	Project       string   `json:"project,omitempty" jsonschema:"project key"`
	Types         []string `json:"types,omitempty" jsonschema:"epic, story, feature, task, qa"`
	StateCategory []string `json:"state_category,omitempty" jsonschema:"backlog, unstarted, started, completed, canceled"`
	Assignee      string   `json:"assignee,omitempty" jsonschema:"username, or me"`
	Parent        string   `json:"parent,omitempty" jsonschema:"direct parent issue key"`
	Under         string   `json:"under,omitempty" jsonschema:"issue key; returns all its descendants"`
	Text          string   `json:"text,omitempty" jsonschema:"search in key, title and description"`
	Limit         int      `json:"limit,omitempty" jsonschema:"max results (default 50)"`
}

type keyIn struct {
	Key string `json:"key" jsonschema:"issue key, e.g. APP-12"`
}

type treeIn struct {
	Key      string `json:"key" jsonschema:"root issue key"`
	MaxDepth int    `json:"max_depth,omitempty" jsonschema:"levels below the root to include (0 = all)"`
}

type createIn struct {
	ProjectKey  string   `json:"project_key,omitempty" jsonschema:"required unless parent_key is given"`
	ParentKey   string   `json:"parent_key,omitempty" jsonschema:"parent issue; tasks may be nested under tasks"`
	Type        string   `json:"type" jsonschema:"epic, story, feature or task"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty" jsonschema:"Markdown"`
	Assignee    string   `json:"assignee,omitempty" jsonschema:"username, or me"`
	Priority    int      `json:"priority,omitempty" jsonschema:"0 none, 1 urgent, 2 high, 3 medium, 4 low"`
	Labels      []string `json:"labels,omitempty"`
}

type updateIn struct {
	Key         string    `json:"key"`
	Title       *string   `json:"title,omitempty"`
	Description *string   `json:"description,omitempty"`
	Assignee    *string   `json:"assignee,omitempty" jsonschema:"username, me, or empty string to unassign"`
	Priority    *int      `json:"priority,omitempty"`
	Labels      *[]string `json:"labels,omitempty" jsonschema:"replaces all labels"`
	ParentKey   *string   `json:"parent_key,omitempty" jsonschema:"new parent key, or empty string for root"`
	DueDate     *string   `json:"due_date,omitempty" jsonschema:"YYYY-MM-DD, or empty string to clear"`
}

type transitionIn struct {
	Key   string `json:"key"`
	State string `json:"state" jsonschema:"state name, e.g. Todo, In Progress, In Review, Done"`
}

type commentIn struct {
	Key  string `json:"key"`
	Body string `json:"body" jsonschema:"Markdown"`
}

type artifactsIn struct {
	Branch  string   `json:"branch,omitempty"`
	Commits []string `json:"commits,omitempty"`
	PR      string   `json:"pr,omitempty" jsonschema:"pull request URL"`
	Files   []string `json:"files,omitempty"`
}

type checkpointIn struct {
	Key          string       `json:"key" jsonschema:"issue you are working on"`
	Summary      string       `json:"summary" jsonschema:"what you did since the last checkpoint"`
	Decision     string       `json:"decision,omitempty" jsonschema:"what you decided"`
	Rationale    string       `json:"rationale" jsonschema:"why: the criteria behind the decision (判断基準)"`
	Alternatives []string     `json:"alternatives,omitempty" jsonschema:"options you considered and why they were rejected"`
	Confidence   string       `json:"confidence,omitempty" jsonschema:"high, medium or low (low = a human should review)"`
	Progress     *int         `json:"progress,omitempty" jsonschema:"0-100"`
	NextSteps    string       `json:"next_steps,omitempty"`
	Artifacts    *artifactsIn `json:"artifacts,omitempty"`
}

type optionIn struct {
	ID          string `json:"id,omitempty" jsonschema:"short id (defaults to a, b, c...); for yes_no use yes/no, for ok_ng use ok/ng"`
	Label       string `json:"label,omitempty" jsonschema:"required for single_choice/multi_choice"`
	Description string `json:"description,omitempty"`
	Recommended bool   `json:"recommended,omitempty" jsonschema:"mark your recommended option"`
}

type questionIn struct {
	Question   string     `json:"question"`
	AnswerType string     `json:"answer_type" jsonschema:"yes_no, ok_ng, single_choice, multi_choice or text"`
	Options    []optionIn `json:"options,omitempty" jsonschema:"2-6 options for single_choice/multi_choice; for yes_no/ok_ng only to mark the recommended one"`
	AllowOther bool       `json:"allow_other,omitempty" jsonschema:"allow a free-text other answer (choices only)"`
	Required   *bool      `json:"required,omitempty" jsonschema:"default true"`
}

type askIn struct {
	TargetKey           string       `json:"target_key" jsonschema:"issue the questions are about"`
	Questions           []questionIn `json:"questions" jsonschema:"one or more questions"`
	Title               string       `json:"title,omitempty" jsonschema:"summary; defaults to the first question"`
	Context             string       `json:"context,omitempty" jsonschema:"background the human needs to answer"`
	ProvisionalDecision string       `json:"provisional_decision,omitempty" jsonschema:"what you will do if nobody answers"`
	IsBlocking          bool         `json:"is_blocking,omitempty" jsonschema:"true if you cannot continue without the answer"`
	Assignee            string       `json:"assignee,omitempty" jsonschema:"who should answer (default: project owner)"`
}

type answersIn struct {
	Key    string `json:"key,omitempty" jsonschema:"a specific QA key; otherwise lists your questions"`
	Status string `json:"status,omitempty" jsonschema:"open, answered or closed (default: open and answered)"`
}

type acceptanceIn struct {
	Key  string `json:"key" jsonschema:"story or feature key"`
	Note string `json:"note,omitempty" jsonschema:"comment for the reviewer (what to check)"`
}

// ---- tools ----

func (h *handlers) register(s *mcp.Server) {
	svc := h.svc

	tool(s, h, "list_projects", "List projects", "List active projects with progress.", true,
		func(ctx context.Context, _ empty) (any, error) {
			ps, err := svc.ListProjects(ctx, false)
			return map[string]any{"projects": ps}, err
		})

	tool(s, h, "get_storymap", "Get story map", "A project's requirements: personas, jobs to be done, the story map (activities and their steps), stories placed under steps with their status (draft = no acceptance criteria, ready, in_progress, done), releases, missing links (gaps) and mode (structured: place stories under steps and write given/when/then; quick: one line is fine). Use it to understand who an issue is for and why.", true,
		func(ctx context.Context, in storyMapIn) (any, error) {
			return svc.GetStoryMap(ctx, in.Project)
		})

	tool(s, h, "create_story", "Create story", "Add a story to a project's story map. Only the title is required: without steps it goes to the unsorted column (未整理). Criteria are one line each (then); given/when are optional in quick projects and expected in structured ones.", false,
		func(ctx context.Context, in createStoryIn) (any, error) {
			si := service.StoryInput{Title: in.Title, Body: in.Body, IWant: in.IWant, SoThat: in.SoThat, Release: in.Release, StepKeys: in.StepKeys}
			if in.PersonaKey != "" {
				si.PersonaKey = &in.PersonaKey
			}
			st, err := svc.CreateStory(ctx, in.Project, si)
			if err != nil {
				return nil, err
			}
			for _, c := range in.Criteria {
				if _, err := svc.CreateCriterion(ctx, in.Project, st.Key, service.ReqCriterionInput{Given: c.Given, When: c.When, Then: c.Then}); err != nil {
					return nil, err
				}
			}
			return svc.GetStory(ctx, in.Project, st.Key)
		})

	tool(s, h, "update_story", "Update story", "Tidy a story: title, body, persona, release, placement (step_keys) and acceptance criteria (add or change). "+
		"Before you move or split things the owner wrote, ask with ask_question (yes_no) and apply only what was approved.", false,
		func(ctx context.Context, in updateStoryIn) (any, error) {
			patch := service.StoryPatch{Title: in.Title, Body: in.Body, IWant: in.IWant, SoThat: in.SoThat, Release: in.Release, StepKeys: in.StepKeys}
			if in.PersonaKey != nil {
				if *in.PersonaKey == "" {
					patch.PersonaKey = service.Null[string]()
				} else {
					patch.PersonaKey = service.Some(*in.PersonaKey)
				}
			}
			if _, err := svc.UpdateStory(ctx, in.Project, in.Key, patch); err != nil {
				return nil, err
			}
			for _, c := range in.UpdateCriteria {
				if _, err := svc.UpdateCriterion(ctx, in.Project, c.Key, service.ReqCriterionPatch{Given: c.Given, When: c.When, Then: c.Then}); err != nil {
					return nil, err
				}
			}
			for _, c := range in.AddCriteria {
				if _, err := svc.CreateCriterion(ctx, in.Project, in.Key, service.ReqCriterionInput{Given: c.Given, When: c.When, Then: c.Then}); err != nil {
					return nil, err
				}
			}
			return svc.GetStory(ctx, in.Project, in.Key)
		})

	tool(s, h, "create_persona", "Create persona", "Add a persona (who the stories are for) to a project. Link stories and activities to it with persona_key.", false,
		func(ctx context.Context, in createPersonaIn) (any, error) {
			return svc.CreatePersona(ctx, in.Project, service.PersonaInput{Name: in.Name, Kind: in.Kind, Description: in.Description})
		})

	tool(s, h, "update_persona", "Update persona", "Change a persona's name, kind, description or order. Ask with ask_question before changing what the owner wrote.", false,
		func(ctx context.Context, in updatePersonaIn) (any, error) {
			return svc.UpdatePersona(ctx, in.Project, in.Key, service.PersonaPatch{Name: in.Name, Kind: in.Kind, Description: in.Description, Position: in.Position})
		})

	tool(s, h, "create_activity", "Create activity", "Add an activity (a column group of the story map: what the persona does, left to right) with its steps. "+
		"Place stories under the steps with create_story / update_story step_keys.", false,
		func(ctx context.Context, in createActivityIn) (any, error) {
			ai := service.StoryActivityInput{Name: in.Name, Description: in.Description}
			if in.PersonaKey != "" {
				ai.PersonaKey = &in.PersonaKey
			}
			a, err := svc.CreateActivity(ctx, in.Project, ai)
			if err != nil {
				return nil, err
			}
			for _, st := range in.Steps {
				step, err := svc.CreateStep(ctx, in.Project, a.Key, service.StepInput{Name: st.Name, Description: st.Description})
				if err != nil {
					return nil, err
				}
				a.Steps = append(a.Steps, step)
			}
			return a, nil
		})

	tool(s, h, "update_activity", "Update activity", "Rename an activity, change its persona or description, or move it (position). Ask with ask_question before changing what the owner wrote.", false,
		func(ctx context.Context, in updateActivityIn) (any, error) {
			patch := service.StoryActivityPatch{Name: in.Name, Description: in.Description, Position: in.Position}
			if in.PersonaKey != nil {
				if *in.PersonaKey == "" {
					patch.PersonaKey = service.Null[string]()
				} else {
					patch.PersonaKey = service.Some(*in.PersonaKey)
				}
			}
			return svc.UpdateActivity(ctx, in.Project, in.Key, patch)
		})

	tool(s, h, "create_step", "Create step", "Add a step at the end of an activity.", false,
		func(ctx context.Context, in createStepIn) (any, error) {
			return svc.CreateStep(ctx, in.Project, in.Activity, service.StepInput{Name: in.Name, Description: in.Description})
		})

	tool(s, h, "update_step", "Update step", "Rename a step, change its description or move it within its activity (position).", false,
		func(ctx context.Context, in updateStepIn) (any, error) {
			return svc.UpdateStep(ctx, in.Project, in.Key, service.StepPatch{Name: in.Name, Description: in.Description, Position: in.Position})
		})

	tool(s, h, "update_release", "Update release slice", "Set a release slice's goal, status, metric or name (the rows of get_storymap's release_slices). "+
		"Name slices by their outcome. Ask with ask_question before changing what the owner wrote.", false,
		func(ctx context.Context, in updateReleaseIn) (any, error) {
			return svc.UpsertRelease(ctx, in.Project, in.Name, service.ReleasePatch{Name: in.NewName, Goal: in.Goal, Status: in.Status, Metric: in.Metric})
		})

	tool(s, h, "create_draft", "Create story map draft", "Make a draft project of a project's story map to brainstorm in (a copy of the current map by default; from=empty for a blank one). "+
		"Edit it with the usual story map tools on the draft's key; the project itself does not change until the owner imports the draft.", false,
		func(ctx context.Context, in createDraftIn) (any, error) {
			return svc.CreateDraft(ctx, in.Project, service.DraftInput{From: in.From, Name: in.Name})
		})

	tool(s, h, "get_import_plan", "Get draft import plan", "What importing a draft into its project would do: elements to create, update (with the changed fields), archive or restore, "+
		"conflicts (changed in the project too), elements only the project has (kept) and warnings. Show it to the owner, who imports in the web UI.", true,
		func(ctx context.Context, in draftKeyIn) (any, error) {
			return svc.PlanImport(ctx, in.Draft)
		})

	tool(s, h, "list_my_issues", "My issues", "Issues assigned to you (open ones by default), across projects.", true,
		func(ctx context.Context, in myIssuesIn) (any, error) {
			and := []domain.Filter{{Field: domain.FieldAssignee, Op: domain.OpEq, Value: "me"}}
			if !in.IncludeClosed {
				and = append(and, domain.Filter{Field: domain.FieldStateCategory, Op: domain.OpNotIn, Value: []any{"completed", "canceled"}})
			}
			if in.Project != "" {
				and = append(and, domain.Filter{Field: domain.FieldProject, Op: domain.OpEq, Value: in.Project})
			}
			res, err := svc.ListIssues(ctx, service.ListOptions{Filter: domain.Filter{And: and}, Sort: "priority", Limit: 200})
			return map[string]any{"issues": compact(res.Issues), "total": res.Total}, err
		})

	tool(s, h, "search_issues", "Search issues", "Search issues by project, type, state, assignee, parent or text.", true,
		func(ctx context.Context, in searchIn) (any, error) {
			and := []domain.Filter{}
			add := func(field, op string, v any) { and = append(and, domain.Filter{Field: field, Op: op, Value: v}) }
			if in.Project != "" {
				add(domain.FieldProject, domain.OpEq, in.Project)
			}
			if len(in.Types) > 0 {
				add(domain.FieldType, domain.OpIn, toAnyList(in.Types))
			}
			if len(in.StateCategory) > 0 {
				add(domain.FieldStateCategory, domain.OpIn, toAnyList(in.StateCategory))
			}
			if in.Assignee != "" {
				add(domain.FieldAssignee, domain.OpEq, in.Assignee)
			}
			if in.Parent != "" {
				add(domain.FieldParent, domain.OpEq, in.Parent)
			}
			if in.Under != "" {
				add(domain.FieldAncestor, domain.OpEq, in.Under)
			}
			if in.Text != "" {
				add(domain.FieldText, domain.OpContains, in.Text)
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 50
			}
			res, err := svc.ListIssues(ctx, service.ListOptions{Filter: domain.Filter{And: and}, Sort: "updated", Limit: limit})
			return map[string]any{"issues": compact(res.Issues), "total": res.Total}, err
		})

	tool(s, h, "get_issue", "Get issue", "Issue details with ancestors, children, the stories it helps satisfy (why it exists: title, free-text body, persona, i_want/so_that, acceptance criteria — often just a one-line then, sometimes given/when/then), recent comments, checkpoints, (for stories and features) acceptance criteria and the latest acceptance review, and (for QA) questions and answers. When a story was sent back (rejected), the latest review explains why.", true,
		func(ctx context.Context, in keyIn) (any, error) {
			is, err := svc.GetIssue(ctx, in.Key)
			if err != nil {
				return nil, err
			}
			cps, err := svc.ListCheckpoints(ctx, in.Key)
			if err != nil {
				return nil, err
			}
			comments, err := svc.ListComments(ctx, in.Key)
			if err != nil {
				return nil, err
			}
			if len(comments) > 20 {
				comments = comments[len(comments)-20:]
			}
			out := map[string]any{"issue": is, "checkpoints": cps, "comments": comments}
			if is.Type == string(domain.TypeQA) {
				if q, err := svc.GetQA(ctx, in.Key); err == nil {
					out["qa"] = q
				}
			}
			if is.Type == string(domain.TypeStory) || is.Type == string(domain.TypeFeature) {
				if a, err := svc.GetAcceptance(ctx, in.Key); err == nil {
					acc := map[string]any{"criteria": a.Criteria, "rounds": len(a.Reviews), "latest_review": nil}
					if len(a.Reviews) > 0 {
						acc["latest_review"] = a.Reviews[len(a.Reviews)-1]
					}
					out["acceptance"] = acc
				}
			}
			return out, nil
		})

	tool(s, h, "get_issue_tree", "Get issue tree", "The issue and its descendants as a nested tree.", true,
		func(ctx context.Context, in treeIn) (any, error) {
			return svc.IssueTree(ctx, in.Key, in.MaxDepth)
		})

	tool(s, h, "create_issue", "Create issue", "Create an issue. Use parent_key to break work down (tasks can be nested under tasks). Ask questions with ask_question, not here.", false,
		func(ctx context.Context, in createIn) (any, error) {
			if in.Type == string(domain.TypeQA) {
				return nil, errors.New("use ask_question to create QA tickets")
			}
			ci := service.CreateIssueInput{ProjectKey: in.ProjectKey, Type: in.Type, Title: in.Title, Description: in.Description,
				Priority: &in.Priority, Labels: in.Labels}
			if in.ParentKey != "" {
				ci.ParentKey = &in.ParentKey
			}
			if in.Assignee != "" {
				ci.Assignee = &in.Assignee
			}
			is, err := svc.CreateIssue(ctx, ci)
			return compactOne(is), err
		})

	tool(s, h, "update_issue", "Update issue", "Change title, description, assignee, priority, labels, parent or due date. Use transition_issue for state.", false,
		func(ctx context.Context, in updateIn) (any, error) {
			var u service.UpdateIssueInput
			if in.Title != nil {
				u.Title = service.Some(*in.Title)
			}
			if in.Description != nil {
				u.Description = service.Some(*in.Description)
			}
			if in.Priority != nil {
				u.Priority = service.Some(*in.Priority)
			}
			if in.Labels != nil {
				u.Labels = service.Some(*in.Labels)
			}
			setNullable := func(v *string) service.Opt[string] {
				if *v == "" {
					return service.Null[string]()
				}
				return service.Some(*v)
			}
			if in.Assignee != nil {
				u.Assignee = setNullable(in.Assignee)
			}
			if in.ParentKey != nil {
				u.ParentKey = setNullable(in.ParentKey)
			}
			if in.DueDate != nil {
				u.DueDate = setNullable(in.DueDate)
			}
			is, err := svc.UpdateIssue(ctx, in.Key, u)
			return compactOne(is), err
		})

	tool(s, h, "transition_issue", "Change state", "Move an issue to another workflow state by name.", false,
		func(ctx context.Context, in transitionIn) (any, error) {
			is, err := svc.TransitionIssue(ctx, in.Key, in.State)
			return compactOne(is), err
		})

	tool(s, h, "add_comment", "Add comment", "Post a comment (Markdown). Prefer record_checkpoint for progress reports.", false,
		func(ctx context.Context, in commentIn) (any, error) {
			return svc.AddComment(ctx, in.Key, in.Body)
		})

	tool(s, h, "record_checkpoint", "Record checkpoint", "Record progress together with the decision and its rationale (判断基準). Call this at every meaningful step.", false,
		func(ctx context.Context, in checkpointIn) (any, error) {
			cp := service.CheckpointInput{Summary: in.Summary, Decision: in.Decision, Rationale: in.Rationale, Alternatives: in.Alternatives,
				Confidence: in.Confidence, Progress: in.Progress, NextSteps: in.NextSteps}
			if in.Artifacts != nil {
				b, _ := json.Marshal(in.Artifacts)
				_ = json.Unmarshal(b, &cp.Artifacts)
			}
			return svc.RecordCheckpoint(ctx, in.Key, cp)
		})

	tool(s, h, "ask_question", "Ask humans", "Ask a human one or more questions in a QA ticket. Choose the answer format that is quickest to answer.", false,
		func(ctx context.Context, in askIn) (any, error) {
			q := service.CreateQAInput{TargetKey: &in.TargetKey, Title: in.Title, Context: in.Context,
				ProvisionalDecision: in.ProvisionalDecision, IsBlocking: in.IsBlocking}
			if in.Assignee != "" {
				q.Assignee = &in.Assignee
			}
			for _, it := range in.Questions {
				item := service.QAItemInput{Question: it.Question, AnswerType: it.AnswerType, AllowOther: it.AllowOther, Required: it.Required}
				for _, o := range it.Options {
					item.Options = append(item.Options, service.QAOptionInput{ID: o.ID, Label: o.Label, Description: o.Description, Recommended: o.Recommended})
				}
				q.Items = append(q.Items, item)
			}
			return svc.CreateQA(ctx, q)
		})

	tool(s, h, "get_question_answers", "Get answers", "Read answers to your questions. With key: that QA ticket. Without: your open and answered questions.", true,
		func(ctx context.Context, in answersIn) (any, error) {
			if in.Key != "" {
				return svc.GetQA(ctx, in.Key)
			}
			if in.Status != "" {
				qs, err := svc.ListQA(ctx, service.ListQAOptions{AskedBy: "me", Status: in.Status})
				return map[string]any{"questions": qs}, err
			}
			open, err := svc.ListQA(ctx, service.ListQAOptions{AskedBy: "me", Status: service.QAStatusOpen})
			if err != nil {
				return nil, err
			}
			answered, err := svc.ListQA(ctx, service.ListQAOptions{AskedBy: "me", Status: service.QAStatusAnswered})
			return map[string]any{"answered": answered, "open": open}, err
		})

	tool(s, h, "close_question", "Close question", "Close a QA ticket after you have acted on the answers.", false,
		func(ctx context.Context, in keyIn) (any, error) {
			return svc.CloseQA(ctx, in.Key)
		})

	tool(s, h, "request_acceptance", "Request acceptance", "Move a finished story or feature to the acceptance state (受入待ち) for a human check.", false,
		func(ctx context.Context, in acceptanceIn) (any, error) {
			is, err := svc.RequestAcceptance(ctx, in.Key)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(in.Note) != "" {
				if _, err := svc.AddComment(ctx, in.Key, in.Note); err != nil {
					return nil, err
				}
			}
			return compactOne(is), nil
		})

	h.registerResources(s)
	h.registerPrompts(s)
}

// ---- resources and prompts ----

func (h *handlers) registerResources(s *mcp.Server) {
	s.AddResourceTemplate(&mcp.ResourceTemplate{Name: "issue", Title: "Issue", URITemplate: "taskapp://issue/{key}",
		Description: "An issue as Markdown (details, children, checkpoints)", MIMEType: "text/markdown"},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			key := strings.TrimPrefix(req.Params.URI, "taskapp://issue/")
			ctx, err := h.actorCtx(ctx, &mcp.CallToolRequest{Extra: req.Extra})
			if err != nil {
				return nil, err
			}
			is, err := h.svc.GetIssue(ctx, key)
			if err != nil {
				return nil, mcp.ResourceNotFoundError(req.Params.URI)
			}
			cps, _ := h.svc.ListCheckpoints(ctx, key)
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: "text/markdown", Text: issueMarkdown(is, cps)}}}, nil
		})
}

func (h *handlers) registerPrompts(s *mcp.Server) {
	s.AddPrompt(&mcp.Prompt{Name: "work_on_issue", Title: "Work on an issue",
		Description: "Step-by-step instructions for working on a taskapp issue with checkpoints and questions",
		Arguments:   []*mcp.PromptArgument{{Name: "issue_key", Description: "e.g. APP-12", Required: true}}},
		func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			key := req.Params.Arguments["issue_key"]
			text := fmt.Sprintf(`Work on taskapp issue %s.

1. Call get_issue for %s and read the description, parent chain and any existing checkpoints and QA.
2. transition_issue %s to "In Progress".
3. Do the work. After each meaningful step call record_checkpoint with summary, decision and rationale (判断基準).
   Use confidence "low" whenever a human should double-check.
4. If you need a decision, call ask_question (yes_no / ok_ng / single_choice / multi_choice / text; several
   questions per ticket are fine). Give your provisional decision. Set is_blocking if you must wait.
5. Check get_question_answers, apply the answers, then close_question.
6. Split large work with create_issue (parent_key = %s).
7. Finish with a final record_checkpoint, then transition_issue to "In Review"
   (or request_acceptance for a story/feature ready for acceptance).`, key, key, key, key)
			return &mcp.GetPromptResult{Description: "Agent workflow for " + key,
				Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}}}, nil
		})
}

// ---- formatting ----

type compactIssue struct {
	Key         string   `json:"key"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	State       string   `json:"state"`
	Priority    int      `json:"priority"`
	Assignee    string   `json:"assignee,omitempty"`
	ParentKey   string   `json:"parent_key,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	OpenQACount int      `json:"open_qa_count,omitempty"`
	Progress    string   `json:"progress,omitempty"`
	UpdatedAt   string   `json:"updated_at"`
}

func compactOne(is service.Issue) compactIssue {
	c := compactIssue{Key: is.Key, Type: is.Type, Title: is.Title, State: is.State.Name, Priority: is.Priority,
		ParentKey: deref(is.ParentKey), OpenQACount: is.OpenQACount, UpdatedAt: is.UpdatedAt}
	if is.Assignee != nil {
		c.Assignee = is.Assignee.Username
	}
	for _, l := range is.Labels {
		c.Labels = append(c.Labels, l.Name)
	}
	if is.Progress.Total > 0 {
		c.Progress = fmt.Sprintf("%d/%d", is.Progress.Done, is.Progress.Total)
	}
	return c
}

func compact(list []service.Issue) []compactIssue {
	out := make([]compactIssue, len(list))
	for i, is := range list {
		out[i] = compactOne(is)
	}
	return out
}

func issueMarkdown(is service.Issue, cps []service.Checkpoint) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s %s\n\n", is.Key, is.Title)
	fmt.Fprintf(&b, "- type: %s\n- state: %s\n", is.Type, is.State.Name)
	if is.Assignee != nil {
		fmt.Fprintf(&b, "- assignee: %s (%s)\n", is.Assignee.Username, is.Assignee.Kind)
	}
	for _, a := range is.Ancestors {
		fmt.Fprintf(&b, "- ancestor: %s %s (%s)\n", a.Key, a.Title, a.Type)
	}
	if is.Description != "" {
		b.WriteString("\n" + is.Description + "\n")
	}
	if len(is.Children) > 0 {
		b.WriteString("\n## Children\n")
		for _, c := range is.Children {
			fmt.Fprintf(&b, "- %s [%s] %s (%s)\n", c.Key, c.Type, c.Title, c.State.Name)
		}
	}
	if len(cps) > 0 {
		b.WriteString("\n## Checkpoints\n")
		for _, c := range cps {
			fmt.Fprintf(&b, "- #%d %s — %s / rationale: %s (confidence %s)\n", c.Seq, c.Summary, c.Decision, c.Rationale, c.Confidence)
		}
	}
	return b.String()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func toAnyList(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
