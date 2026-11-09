package jira

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/jira/adf"
	"github.com/ysksm/task-app/internal/service"
)

// Fixed ids (docs/02-architecture.md §7.1).

type issueTypeDef struct {
	ID, Name, Description string
	Type                  domain.IssueType
	HierarchyLevel        int
}

var issueTypes = []issueTypeDef{
	{"10000", "Epic", "大きな目標。ストーリーをまとめる", domain.TypeEpic, 1},
	{"10001", "Story", "利用者にとっての価値の単位", domain.TypeStory, 0},
	{"10002", "Feature", "ストーリーを実現する機能", domain.TypeFeature, 0},
	{"10003", "Task", "作業。タスクの下にタスクを置ける", domain.TypeTask, 0},
	{"10004", "Question", "エージェントから人間への確認事項 (QA)", domain.TypeQA, 0},
}

func issueTypeOf(t string) issueTypeDef {
	for _, d := range issueTypes {
		if string(d.Type) == t {
			return d
		}
	}
	return issueTypes[3]
}

type priorityDef struct {
	ID, Name, Color string
	Value           int
}

var priorities = []priorityDef{
	{"1", "Highest", "#e5484d", domain.PriorityUrgent},
	{"2", "High", "#f76b15", domain.PriorityHigh},
	{"3", "Medium", "#d9a21b", domain.PriorityMedium},
	{"4", "Low", "#3e63dd", domain.PriorityLow},
}

type categoryDef struct {
	ID               int
	Key, Name, Color string
}

var (
	catNew           = categoryDef{2, "new", "To Do", "blue-gray"}
	catIndeterminate = categoryDef{4, "indeterminate", "In Progress", "yellow"}
	catDone          = categoryDef{3, "done", "Done", "green"}
)

func categoryOf(c string) categoryDef {
	switch domain.StateCategory(c) {
	case domain.CategoryStarted:
		return catIndeterminate
	case domain.CategoryCompleted, domain.CategoryCanceled:
		return catDone
	}
	return catNew
}

// mapper renders Jira JSON for one request (it knows the base URL).
type mapper struct{ base string }

func (m mapper) api(path string, args ...any) string {
	return m.base + "/rest/api/3" + fmt.Sprintf(path, args...)
}

// jiraTime converts 2026-10-11T16:20:09.311Z to Jira's 2026-10-11T16:20:09.311+0000.
func jiraTime(ts string) string {
	if strings.HasSuffix(ts, "Z") {
		return strings.TrimSuffix(ts, "Z") + "+0000"
	}
	return ts
}

func jiraTimePtr(ts *string) any {
	if ts == nil {
		return nil
	}
	return jiraTime(*ts)
}

func (m mapper) avatarURLs(userID string) map[string]string {
	u := m.base + "/rest/api/3/avatar/user/" + userID + ".svg"
	return map[string]string{"48x48": u, "32x32": u, "24x24": u, "16x16": u}
}

func accountType(kind string) string {
	if kind == "agent" {
		return "app"
	}
	return "atlassian"
}

func (m mapper) user(u service.User) map[string]any {
	return map[string]any{
		"self":         m.api("/user?accountId=%s", u.ID),
		"accountId":    u.ID,
		"accountType":  accountType(u.Kind),
		"emailAddress": u.Email,
		"displayName":  u.DisplayName,
		"name":         u.Username,
		"active":       u.Active,
		"timeZone":     "Asia/Tokyo",
		"locale":       "ja_JP",
		"avatarUrls":   m.avatarURLs(u.ID),
	}
}

// userRef renders an embedded user. Emails are looked up from users.
func (m mapper) userRef(r *service.UserRef, users map[string]service.User) any {
	if r == nil {
		return nil
	}
	if u, ok := users[r.ID]; ok {
		return m.user(u)
	}
	return m.user(service.User{ID: r.ID, Kind: r.Kind, Username: r.Username, DisplayName: r.DisplayName, Active: true})
}

func (m mapper) issueType(d issueTypeDef) map[string]any {
	return map[string]any{
		"self":           m.api("/issuetype/%s", d.ID),
		"id":             d.ID,
		"name":           d.Name,
		"description":    d.Description,
		"iconUrl":        m.base + "/rest/api/3/avatar/issuetype/" + d.ID + ".svg",
		"subtask":        false,
		"hierarchyLevel": d.HierarchyLevel,
	}
}

func (m mapper) statusCategory(c categoryDef) map[string]any {
	return map[string]any{
		"self":      m.api("/statuscategory/%d", c.ID),
		"id":        c.ID,
		"key":       c.Key,
		"colorName": c.Color,
		"name":      c.Name,
	}
}

func (m mapper) status(id int64, name, category string) map[string]any {
	return map[string]any{
		"self":           m.api("/status/%d", id),
		"id":             strconv.FormatInt(id, 10),
		"name":           name,
		"description":    "",
		"iconUrl":        "",
		"statusCategory": m.statusCategory(categoryOf(category)),
	}
}

func (m mapper) priority(p priorityDef) map[string]any {
	return map[string]any{
		"self":        m.api("/priority/%s", p.ID),
		"id":          p.ID,
		"name":        p.Name,
		"statusColor": p.Color,
		"iconUrl":     m.base + "/rest/api/3/avatar/priority/" + p.ID + ".svg",
	}
}

func (m mapper) priorityOf(v int) any {
	for _, p := range priorities {
		if p.Value == v {
			return m.priority(p)
		}
	}
	return nil
}

func (m mapper) project(p service.Project) map[string]any {
	return map[string]any{
		"self":           m.api("/project/%d", p.JiraID),
		"id":             strconv.FormatInt(p.JiraID, 10),
		"key":            p.Key,
		"name":           p.Name,
		"description":    p.Description,
		"projectTypeKey": "software",
		"simplified":     false,
		"style":          "classic",
		"isPrivate":      false,
		"avatarUrls":     map[string]string{},
	}
}

func (m mapper) resolution(category string) any {
	switch domain.StateCategory(category) {
	case domain.CategoryCompleted:
		return map[string]any{"self": m.api("/resolution/10000"), "id": "10000", "name": "Done", "description": "完了"}
	case domain.CategoryCanceled:
		return map[string]any{"self": m.api("/resolution/10001"), "id": "10001", "name": "Won't Do", "description": "中止"}
	}
	return nil
}

func adfOrNil(markdown string) any {
	if d := adf.FromMarkdown(markdown); d != nil {
		return d
	}
	return nil
}

// fieldSet decides which fields to render (?fields=...).
type fieldSet struct {
	all     bool
	include map[string]bool
	exclude map[string]bool
}

// alwaysFields are rendered whatever was asked for: ORCA (and other
// clients) substitute "now" or placeholders when they are missing.
var alwaysFields = []string{"summary", "status", "issuetype", "project", "created", "updated"}

func parseFields(list []string) fieldSet {
	fs := fieldSet{include: map[string]bool{}, exclude: map[string]bool{}}
	if len(list) == 0 {
		fs.all = true
	}
	for _, raw := range list {
		for _, f := range strings.Split(raw, ",") {
			f = strings.TrimSpace(f)
			switch {
			case f == "":
			case f == "*all" || f == "*navigable":
				fs.all = true
			case strings.HasPrefix(f, "-"):
				fs.exclude[f[1:]] = true
			default:
				fs.include[f] = true
			}
		}
	}
	for _, f := range alwaysFields {
		fs.include[f] = true
	}
	return fs
}

func (fs fieldSet) has(name string) bool {
	if fs.exclude[name] {
		return false
	}
	return fs.all || fs.include[name]
}

// issueCtx carries lookups needed while rendering issues.
type issueCtx struct {
	users    map[string]service.User
	projects map[string]service.Project // by key
	parents  map[string]service.Issue   // by key
	comments map[string]map[string]any  // by issue key, when the comment field was requested
}

func (m mapper) issue(is service.Issue, fs fieldSet, ic issueCtx) map[string]any {
	f := map[string]any{}
	put := func(name string, v func() any) {
		if fs.has(name) {
			f[name] = v()
		}
	}
	put("summary", func() any { return is.Title })
	put("description", func() any { return adfOrNil(is.Description) })
	put("project", func() any { return m.project(ic.projects[is.ProjectKey]) })
	put("issuetype", func() any { return m.issueType(issueTypeOf(is.Type)) })
	put("status", func() any { return m.status(is.State.ID, is.State.Name, is.State.Category) })
	put("statusCategory", func() any { return m.statusCategory(categoryOf(is.State.Category)) })
	put("assignee", func() any { return m.userRef(is.Assignee, ic.users) })
	put("reporter", func() any { return m.userRef(is.Reporter, ic.users) })
	put("creator", func() any { return m.userRef(is.Reporter, ic.users) })
	put("priority", func() any { return m.priorityOf(is.Priority) })
	put("labels", func() any {
		names := make([]string, 0, len(is.Labels))
		for _, l := range is.Labels {
			names = append(names, l.Name)
		}
		return names
	})
	put("created", func() any { return jiraTime(is.CreatedAt) })
	put("updated", func() any { return jiraTime(is.UpdatedAt) })
	put("duedate", func() any { return is.DueDate })
	put("resolution", func() any { return m.resolution(is.State.Category) })
	put("resolutiondate", func() any {
		if is.CompletedAt != nil {
			return jiraTime(*is.CompletedAt)
		}
		return jiraTimePtr(is.CanceledAt)
	})
	put("subtasks", func() any { return []any{} })
	put("attachment", func() any { return []any{} })
	put("issuelinks", func() any { return []any{} })
	put("timeestimate", func() any { return nil })
	if is.ParentKey != nil && fs.has("parent") {
		if p, ok := ic.parents[*is.ParentKey]; ok {
			f["parent"] = map[string]any{
				"id":   strconv.FormatInt(p.JiraID, 10),
				"key":  p.Key,
				"self": m.api("/issue/%d", p.JiraID),
				"fields": map[string]any{
					"summary":   p.Title,
					"status":    m.status(p.State.ID, p.State.Name, p.State.Category),
					"priority":  m.priorityOf(p.Priority),
					"issuetype": m.issueType(issueTypeOf(p.Type)),
				},
			}
		}
	}
	if c, ok := ic.comments[is.Key]; ok && fs.has("comment") {
		f["comment"] = c
	}
	return map[string]any{
		"expand": "renderedFields,names,schema,operations,editmeta,changelog,versionedRepresentations",
		"id":     strconv.FormatInt(is.JiraID, 10),
		"key":    is.Key,
		"self":   m.api("/issue/%d", is.JiraID),
		"fields": f,
	}
}

// comment renders a comment.
func (m mapper) comment(issueID string, c service.Comment, users map[string]service.User) map[string]any {
	author := m.userRef(c.Author, users)
	body := adfOrNil(c.Body)
	if body == nil {
		body = adf.Doc()
	}
	id := c.ID
	if c.JiraID != 0 {
		id = strconv.FormatInt(c.JiraID, 10)
	}
	return map[string]any{
		"self":         m.api("/issue/%s/comment/%s", issueID, id),
		"id":           id,
		"author":       author,
		"updateAuthor": author,
		"body":         body,
		"created":      jiraTime(c.CreatedAt),
		"updated":      jiraTime(c.UpdatedAt),
		"jsdPublic":    true,
	}
}

var confidenceLabel = map[string]string{"high": "高", "medium": "中", "low": "低（要確認）"}

// checkpointMarkdown is how a checkpoint reads as a Jira comment.
func checkpointMarkdown(cp service.Checkpoint) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s #%d %s\n\n", checkpointPrefix, cp.Seq, cp.Summary)
	line := func(label, v string) {
		if v != "" {
			fmt.Fprintf(&b, "- **%s**: %s\n", label, v)
		}
	}
	line("判断", cp.Decision)
	line("判断基準", cp.Rationale)
	if len(cp.Alternatives) > 0 {
		line("代替案", strings.Join(cp.Alternatives, " / "))
	}
	line("確信度", confidenceLabel[cp.Confidence])
	if cp.Progress != nil {
		line("進捗", fmt.Sprintf("%d%%", *cp.Progress))
	}
	line("次", cp.NextSteps)
	return strings.TrimSpace(b.String())
}

func (m mapper) checkpointComment(issueID string, cp service.Checkpoint, users map[string]service.User) map[string]any {
	c := service.Comment{ID: "checkpoint-" + cp.ID, Author: cp.Author, Body: checkpointMarkdown(cp), CreatedAt: cp.CreatedAt, UpdatedAt: cp.CreatedAt}
	return m.comment(issueID, c, users)
}
