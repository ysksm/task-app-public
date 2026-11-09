package jira

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/jira/adf"
	"github.com/ysksm/task-app/internal/jira/jql"
	"github.com/ysksm/task-app/internal/service"
)

// ---- server / me ----

func (h *handler) myself(w http.ResponseWriter, r *http.Request) {
	a, _ := service.ActorFrom(r.Context())
	u := h.mapper(r).user(a.User)
	u["groups"] = map[string]any{"size": 0, "items": []any{}}
	u["applicationRoles"] = map[string]any{"size": 1, "items": []any{}}
	writeJSON(w, http.StatusOK, u)
}

func (h *handler) serverInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"baseUrl":        h.mapper(r).base,
		"displayUrl":     h.mapper(r).base,
		"version":        "1001.0.0-SNAPSHOT",
		"versionNumbers": []int{1001, 0, 0},
		"deploymentType": "Cloud",
		"buildNumber":    100000,
		"serverTime":     time.Now().Format("2006-01-02T15:04:05.000-0700"),
		"scmInfo":        "taskapp",
		"serverTitle":    "taskapp",
	})
}

// ---- projects ----

func (h *handler) listProjectsArray(w http.ResponseWriter, r *http.Request) {
	list, err := h.projects(r)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	m := h.mapper(r)
	out := make([]any, 0, len(list))
	for _, p := range list {
		out = append(out, m.project(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handler) searchProjects(w http.ResponseWriter, r *http.Request) {
	list, err := h.projects(r)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	if q := strings.ToLower(r.URL.Query().Get("query")); q != "" {
		filtered := list[:0]
		for _, p := range list {
			if strings.Contains(strings.ToLower(p.Key), q) || strings.Contains(strings.ToLower(p.Name), q) {
				filtered = append(filtered, p)
			}
		}
		list = filtered
	}
	if keys := r.URL.Query()["keys"]; len(keys) > 0 {
		var filtered []service.Project
		for _, k := range keys {
			if p, ok := findProject(list, k); ok {
				filtered = append(filtered, p)
			}
		}
		list = filtered
	}
	startAt := clamp(intParam(r, "startAt", 0), 0, 1<<30)
	maxResults := clamp(intParam(r, "maxResults", 50), 1, 100)
	m := h.mapper(r)
	values := []any{}
	for _, p := range page(list, startAt, maxResults) {
		values = append(values, m.project(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"self":       m.api("/project/search?startAt=%d&maxResults=%d", startAt, maxResults),
		"startAt":    startAt,
		"maxResults": maxResults,
		"total":      len(list),
		"isLast":     startAt+maxResults >= len(list),
		"values":     values,
	})
}

func (h *handler) projectByParam(w http.ResponseWriter, r *http.Request) (service.Project, bool) {
	list, err := h.projects(r)
	if err != nil {
		writeSvcErr(w, err)
		return service.Project{}, false
	}
	ref := chi.URLParam(r, "project")
	p, ok := findProject(list, ref)
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Sprintf("No project could be found with key '%s'.", ref))
	}
	return p, ok
}

func (h *handler) getProject(w http.ResponseWriter, r *http.Request) {
	p, ok := h.projectByParam(w, r)
	if !ok {
		return
	}
	m := h.mapper(r)
	out := m.project(p)
	if owner, err := h.svc.UserByID(r.Context(), p.Owner.ID); err == nil {
		out["lead"] = m.user(owner)
	}
	types := []any{}
	for _, d := range issueTypes {
		types = append(types, m.issueType(d))
	}
	out["issueTypes"] = types
	writeJSON(w, http.StatusOK, out)
}

func (h *handler) projectStatuses(w http.ResponseWriter, r *http.Request) {
	p, ok := h.projectByParam(w, r)
	if !ok {
		return
	}
	states, err := h.svc.ListStates(r.Context(), p.Key)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	m := h.mapper(r)
	out := []any{}
	for _, d := range issueTypes {
		sts := []any{}
		for _, st := range statesFor(states, d.Type) {
			sts = append(sts, m.status(st.ID, st.Name, st.Category))
		}
		t := m.issueType(d)
		t["statuses"] = sts
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, out)
}

// statesFor returns the workflow states an issue type can be in.
func statesFor(states []service.State, t domain.IssueType) []service.State {
	want := "all"
	if t == domain.TypeQA {
		want = "qa"
	}
	out := []service.State{}
	for _, st := range states {
		if st.AppliesTo == want {
			out = append(out, st)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out
}

// ---- search ----

type searchRequest struct {
	JQL           string          `json:"jql"`
	MaxResults    *int            `json:"maxResults"`
	StartAt       int             `json:"startAt"`
	NextPageToken string          `json:"nextPageToken"`
	Fields        json.RawMessage `json:"fields"`
}

func (sr searchRequest) fieldList() []string {
	var list []string
	if json.Unmarshal(sr.Fields, &list) == nil {
		return list
	}
	var one string
	if json.Unmarshal(sr.Fields, &one) == nil && one != "" {
		return []string{one}
	}
	return nil
}

func (h *handler) readSearch(w http.ResponseWriter, r *http.Request) (searchRequest, []string, bool) {
	var sr searchRequest
	var fields []string
	if r.Method == http.MethodPost {
		if err := decode(r, &sr); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return sr, nil, false
		}
		fields = sr.fieldList()
	} else {
		q := r.URL.Query()
		sr.JQL = q.Get("jql")
		if v, err := strconv.Atoi(q.Get("maxResults")); err == nil {
			sr.MaxResults = &v
		}
		sr.StartAt = intParam(r, "startAt", 0)
		sr.NextPageToken = q.Get("nextPageToken")
		fields = q["fields"]
	}
	return sr, fields, true
}

// runSearch executes a JQL query and renders the matching issues.
func (h *handler) runSearch(w http.ResponseWriter, r *http.Request, sr searchRequest, fields []string, offset, limit int) ([]any, int, bool) {
	projects, err := h.projects(r)
	if err != nil {
		writeSvcErr(w, err)
		return nil, 0, false
	}
	res, err := jql.Compile(sr.JQL, jql.Options{ProjectKey: func(ref string) string {
		if p, ok := findProject(projects, ref); ok {
			return p.Key
		}
		return ref
	}})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return nil, 0, false
	}
	list, err := h.svc.ListIssues(r.Context(), service.ListOptions{Filter: res.Filter, Sort: res.Sort, Limit: limit, Offset: offset})
	if err != nil {
		e := service.AsError(err)
		if e.Code == service.CodeInvalidArgument {
			writeErr(w, http.StatusBadRequest, jql.ErrorPrefix+e.Message)
		} else {
			writeSvcErr(w, err)
		}
		return nil, 0, false
	}
	out, err := h.renderIssues(r, list.Issues, parseFields(fields))
	if err != nil {
		writeSvcErr(w, err)
		return nil, 0, false
	}
	return out, list.Total, true
}

// renderIssues renders issues with the lookups they need.
func (h *handler) renderIssues(r *http.Request, issues []service.Issue, fs fieldSet) ([]any, error) {
	ic, err := h.issueContext(r, issues, fs)
	if err != nil {
		return nil, err
	}
	m := h.mapper(r)
	out := make([]any, 0, len(issues))
	for _, is := range issues {
		out = append(out, m.issue(is, fs, ic))
	}
	return out, nil
}

func (h *handler) issueContext(r *http.Request, issues []service.Issue, fs fieldSet) (issueCtx, error) {
	ic := issueCtx{projects: map[string]service.Project{}, parents: map[string]service.Issue{}}
	var err error
	if ic.users, err = h.users(r); err != nil {
		return ic, err
	}
	projects, err := h.svc.ListProjects(r.Context(), true)
	if err != nil {
		return ic, err
	}
	for _, p := range projects {
		ic.projects[p.Key] = p
	}
	if fs.has("parent") {
		var keys []any
		for _, is := range issues {
			if is.ParentKey != nil {
				keys = append(keys, *is.ParentKey)
			}
		}
		if len(keys) > 0 {
			parents, err := h.svc.ListIssues(r.Context(), service.ListOptions{
				Filter: domain.Filter{Field: domain.FieldKey, Op: domain.OpIn, Value: keys}, Limit: len(keys)})
			if err != nil {
				return ic, err
			}
			for _, p := range parents.Issues {
				ic.parents[p.Key] = p
			}
		}
	}
	return ic, nil
}

// searchJQL implements GET/POST /rest/api/3/search/jql (token paging).
func (h *handler) searchJQL(w http.ResponseWriter, r *http.Request) {
	sr, fields, ok := h.readSearch(w, r)
	if !ok {
		return
	}
	offset := 0
	if sr.NextPageToken != "" {
		var valid bool
		if offset, valid = decodeToken(sr.NextPageToken); !valid {
			writeErr(w, http.StatusBadRequest, "The nextPageToken is invalid.")
			return
		}
	}
	limit := 50
	if sr.MaxResults != nil {
		limit = clamp(*sr.MaxResults, 1, 100)
	}
	issues, total, ok := h.runSearch(w, r, sr, fields, offset, limit)
	if !ok {
		return
	}
	resp := map[string]any{"issues": issues, "isLast": offset+len(issues) >= total}
	if offset+len(issues) < total {
		resp["nextPageToken"] = encodeToken(offset + len(issues))
	}
	writeJSON(w, http.StatusOK, resp)
}

// searchLegacy implements the classic /search with startAt/total paging.
func (h *handler) searchLegacy(w http.ResponseWriter, r *http.Request) {
	sr, fields, ok := h.readSearch(w, r)
	if !ok {
		return
	}
	limit := 50
	if sr.MaxResults != nil {
		limit = clamp(*sr.MaxResults, 0, 100)
	}
	startAt := clamp(sr.StartAt, 0, 1<<30)
	var issues []any
	var total int
	if limit == 0 {
		// maxResults=0 only asks for the total.
		var okSearch bool
		if issues, total, okSearch = h.runSearch(w, r, sr, []string{"summary"}, startAt, 1); !okSearch {
			return
		}
		issues = []any{}
	} else if issues, total, ok = h.runSearch(w, r, sr, fields, startAt, limit); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"expand":     "schema,names",
		"startAt":    startAt,
		"maxResults": limit,
		"total":      total,
		"issues":     issues,
	})
}

// ---- issues ----

func (h *handler) getIssue(w http.ResponseWriter, r *http.Request) {
	is, ok := h.loadIssue(w, r)
	if !ok {
		return
	}
	fs := parseFields(r.URL.Query()["fields"])
	ic, err := h.issueContext(r, []service.Issue{is}, fs)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	m := h.mapper(r)
	if fs.has("comment") {
		comments, err := h.commentList(r, is, ic.users, false)
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		ic.comments = map[string]map[string]any{is.Key: {
			"comments": comments, "maxResults": len(comments), "total": len(comments), "startAt": 0,
			"self": m.api("/issue/%d/comment", is.JiraID),
		}}
	}
	writeJSON(w, http.StatusOK, m.issue(is, fs, ic))
}

// issueFields is the "fields" object of create and edit requests.
type issueFields map[string]json.RawMessage

func (f issueFields) has(k string) bool { _, ok := f[k]; return ok }

func (f issueFields) isNull(k string) bool {
	return strings.TrimSpace(string(f[k])) == "null"
}

func (f issueFields) str(k string) (string, error) {
	var s string
	if err := json.Unmarshal(f[k], &s); err != nil {
		return "", fmt.Errorf("Field '%s' must be a string.", k)
	}
	return s, nil
}

// ref reads {"id": ...} / {"key": ...} / {"name": ...} / {"accountId": ...} or a bare string.
func (f issueFields) ref(k string) (map[string]string, error) {
	raw := f[k]
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return map[string]string{"id": s, "key": s, "name": s}, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, fmt.Errorf("Field '%s' must be an object.", k)
	}
	out := map[string]string{}
	for _, name := range []string{"id", "key", "name", "accountId", "emailAddress", "value"} {
		switch v := obj[name].(type) {
		case string:
			out[name] = v
		case float64:
			out[name] = strconv.FormatInt(int64(v), 10)
		}
	}
	return out, nil
}

func (f issueFields) labels() ([]string, error) {
	var l []string
	if err := json.Unmarshal(f["labels"], &l); err != nil {
		return nil, fmt.Errorf("Field 'labels' must be an array of strings.")
	}
	return l, nil
}

func (h *handler) priorityValue(f issueFields) (int, error) {
	if f.isNull("priority") {
		return domain.PriorityNone, nil
	}
	ref, err := f.ref("priority")
	if err != nil {
		return 0, err
	}
	for _, k := range []string{"id", "name"} {
		if p, ok := jql.Priorities[strings.ToLower(ref[k])]; ok && ref[k] != "" {
			return p, nil
		}
	}
	return 0, fmt.Errorf("The priority selected is invalid.")
}

func userFromRef(users map[string]service.User, ref map[string]string) (service.User, bool) {
	for _, k := range []string{"accountId", "id", "emailAddress", "name", "key"} {
		if v := ref[k]; v != "" {
			if u, ok := resolveUser(users, v); ok {
				return u, true
			}
		}
	}
	return service.User{}, false
}

// parentKey resolves fields.parent ({key}, {id} or a bare key).
func (h *handler) parentKey(r *http.Request, f issueFields) (*string, error) {
	if f.isNull("parent") {
		return nil, nil
	}
	ref, err := f.ref("parent")
	if err != nil {
		return nil, err
	}
	if k := ref["key"]; k != "" && !isDigits(k) {
		k = strings.ToUpper(k)
		return &k, nil
	}
	if id := ref["id"]; id != "" {
		key, err := h.issueKey(r, id)
		if err != nil {
			return nil, fmt.Errorf("The parent issue %s does not exist.", id)
		}
		return &key, nil
	}
	return nil, fmt.Errorf("Field 'parent' needs a key or an id.")
}

func isDigits(s string) bool {
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

func markdownOf(raw json.RawMessage) (string, error) {
	md, err := adf.ToMarkdown(raw)
	if err != nil {
		return "", fmt.Errorf("Field 'description' must be an Atlassian Document: %v", err)
	}
	return md, nil
}

type issueRequest struct {
	Fields     issueFields                 `json:"fields"`
	Update     map[string][]map[string]any `json:"update"`
	Transition map[string]json.RawMessage  `json:"transition"`
}

func (h *handler) createIssue(w http.ResponseWriter, r *http.Request) {
	var req issueRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	f := req.Fields
	if f == nil {
		f = issueFields{}
	}
	errs := map[string]string{}
	in := service.CreateIssueInput{}

	projects, err := h.projects(r)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	if !f.has("project") {
		errs["project"] = "Specify a valid project ID or key"
	} else if ref, err := f.ref("project"); err != nil {
		errs["project"] = err.Error()
	} else {
		var found bool
		for _, k := range []string{"id", "key"} {
			if p, ok := findProject(projects, ref[k]); ok && ref[k] != "" {
				in.ProjectKey, found = p.Key, true
				break
			}
		}
		if !found {
			errs["project"] = "Specify a valid project ID or key"
		}
	}
	if !f.has("issuetype") {
		errs["issuetype"] = "Specify an issue type"
	} else if ref, err := f.ref("issuetype"); err != nil {
		errs["issuetype"] = err.Error()
	} else {
		for _, k := range []string{"id", "name"} {
			if t, ok := jql.IssueTypes[strings.ToLower(ref[k])]; ok && ref[k] != "" {
				in.Type = string(t)
				break
			}
		}
		if in.Type == "" {
			errs["issuetype"] = "Specify a valid issue type"
		} else if in.Type == string(domain.TypeQA) {
			errs["issuetype"] = questionViaMCP
		}
	}
	if s, err := f.str("summary"); err != nil || strings.TrimSpace(s) == "" {
		errs["summary"] = "You must specify a summary of the issue."
	} else {
		in.Title = strings.TrimSpace(s)
	}
	if f.has("description") {
		if in.Description, err = markdownOf(f["description"]); err != nil {
			errs["description"] = err.Error()
		}
	}
	if f.has("parent") {
		if in.ParentKey, err = h.parentKey(r, f); err != nil {
			errs["parent"] = err.Error()
		}
	}
	users, err := h.users(r)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	if f.has("assignee") && !f.isNull("assignee") {
		ref, err := f.ref("assignee")
		if u, ok := userFromRef(users, ref); err == nil && ok {
			in.Assignee = &u.Username
		} else {
			errs["assignee"] = "The assignee does not exist."
		}
	}
	if f.has("priority") {
		p, err := h.priorityValue(f)
		if err != nil {
			errs["priority"] = err.Error()
		}
		in.Priority = &p
	}
	if f.has("labels") {
		if in.Labels, err = f.labels(); err != nil {
			errs["labels"] = err.Error()
		}
	}
	if f.has("duedate") && !f.isNull("duedate") {
		d, err := f.str("duedate")
		if err != nil {
			errs["duedate"] = err.Error()
		}
		in.DueDate = &d
	}
	if len(errs) > 0 {
		writeFieldErrs(w, errs)
		return
	}
	is, err := h.svc.CreateIssue(r.Context(), in)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":   strconv.FormatInt(is.JiraID, 10),
		"key":  is.Key,
		"self": h.mapper(r).api("/issue/%d", is.JiraID),
	})
}

func (h *handler) editIssue(w http.ResponseWriter, r *http.Request) {
	is, ok := h.loadIssue(w, r)
	if !ok {
		return
	}
	var req issueRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	f := req.Fields
	if f == nil {
		f = issueFields{}
	}
	in := service.UpdateIssueInput{}
	errs := map[string]string{}
	var err error
	if f.has("summary") {
		if s, err := f.str("summary"); err != nil || strings.TrimSpace(s) == "" {
			errs["summary"] = "You must specify a summary of the issue."
		} else {
			in.Title = service.Some(strings.TrimSpace(s))
		}
	}
	if f.has("description") {
		md, err := markdownOf(f["description"])
		if err != nil {
			errs["description"] = err.Error()
		}
		in.Description = service.Some(md)
	}
	if f.has("labels") {
		l, err := f.labels()
		if err != nil {
			errs["labels"] = err.Error()
		}
		in.Labels = service.Some(l)
	}
	if f.has("priority") {
		p, err := h.priorityValue(f)
		if err != nil {
			errs["priority"] = err.Error()
		}
		in.Priority = service.Some(p)
	}
	if f.has("duedate") {
		if f.isNull("duedate") {
			in.DueDate = service.Null[string]()
		} else if d, err := f.str("duedate"); err != nil {
			errs["duedate"] = err.Error()
		} else {
			in.DueDate = service.Some(d)
		}
	}
	if f.has("parent") {
		pk, err := h.parentKey(r, f)
		if err != nil {
			errs["parent"] = err.Error()
		} else if pk == nil {
			in.ParentKey = service.Null[string]()
		} else {
			in.ParentKey = service.Some(*pk)
		}
	}
	if f.has("issuetype") {
		ref, err := f.ref("issuetype")
		var t domain.IssueType
		if err == nil {
			for _, k := range []string{"id", "name"} {
				if v, ok := jql.IssueTypes[strings.ToLower(ref[k])]; ok && ref[k] != "" {
					t = v
				}
			}
		}
		if t == "" {
			errs["issuetype"] = "Specify a valid issue type"
		} else if t == domain.TypeQA && is.Type != string(domain.TypeQA) {
			errs["issuetype"] = questionViaMCP
		} else {
			in.Type = service.Some(string(t))
		}
	}
	if f.has("assignee") {
		if f.isNull("assignee") {
			in.Assignee = service.Null[string]()
		} else {
			users, err := h.users(r)
			if err != nil {
				writeSvcErr(w, err)
				return
			}
			ref, err := f.ref("assignee")
			if u, ok := userFromRef(users, ref); err == nil && ok {
				in.Assignee = service.Some(u.Username)
			} else {
				errs["assignee"] = "The assignee does not exist."
			}
		}
	}
	// "update" operations: labels add/remove/set, summary set, comment add.
	var comments []string
	for field, ops := range req.Update {
		for _, op := range ops {
			switch field {
			case "labels":
				cur := in.Labels.Value
				if !in.Labels.Set {
					for _, l := range is.Labels {
						cur = append(cur, l.Name)
					}
				}
				if v, ok := op["add"].(string); ok && !containsStr(cur, v) {
					cur = append(cur, v)
				}
				if v, ok := op["remove"].(string); ok {
					cur = removeStr(cur, v)
				}
				if v, ok := op["set"].([]any); ok {
					cur = nil
					for _, x := range v {
						if s, ok := x.(string); ok {
							cur = append(cur, s)
						}
					}
				}
				in.Labels = service.Some(cur)
			case "summary":
				if v, ok := op["set"].(string); ok {
					in.Title = service.Some(v)
				}
			case "comment":
				if add, ok := op["add"].(map[string]any); ok {
					raw, _ := json.Marshal(add["body"])
					md, err := adf.ToMarkdown(raw)
					if err != nil {
						errs["comment"] = err.Error()
					}
					comments = append(comments, md)
				}
			default:
				errs[field] = fmt.Sprintf("Field '%s' cannot be set. It is not on the appropriate screen, or unknown.", field)
			}
		}
	}
	if len(errs) > 0 {
		writeFieldErrs(w, errs)
		return
	}
	if _, err = h.svc.UpdateIssue(r.Context(), is.Key, in); err != nil {
		writeSvcErr(w, err)
		return
	}
	for _, c := range comments {
		if _, err := h.svc.AddComment(r.Context(), is.Key, c); err != nil {
			writeSvcErr(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func removeStr(list []string, s string) []string {
	out := []string{}
	for _, v := range list {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

func (h *handler) assign(w http.ResponseWriter, r *http.Request) {
	is, ok := h.loadIssue(w, r)
	if !ok {
		return
	}
	var body map[string]any
	if err := decode(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in := service.UpdateIssueInput{}
	var ref string
	for _, k := range []string{"accountId", "name", "emailAddress", "key"} {
		if v, ok := body[k].(string); ok {
			ref = v
			break
		}
	}
	switch ref {
	case "":
		in.Assignee = service.Null[string]()
	case "-1": // automatic: the project owner
		p, err := h.svc.GetProject(r.Context(), is.ProjectKey)
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		in.Assignee = service.Some(p.Owner.Username)
	default:
		users, err := h.users(r)
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		u, ok := resolveUser(users, ref)
		if !ok {
			writeFieldErrs(w, map[string]string{"assignee": fmt.Sprintf("User '%s' does not exist.", ref)})
			return
		}
		in.Assignee = service.Some(u.Username)
	}
	if _, err := h.svc.UpdateIssue(r.Context(), is.Key, in); err != nil {
		writeSvcErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- transitions ----

// Transitions are global: every state the issue type can be in is a
// transition whose id equals the target status id.
func (h *handler) listTransitions(w http.ResponseWriter, r *http.Request) {
	is, ok := h.loadIssue(w, r)
	if !ok {
		return
	}
	states, err := h.svc.ListStates(r.Context(), is.ProjectKey)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	m := h.mapper(r)
	out := []any{}
	for _, st := range statesFor(states, domain.IssueType(is.Type)) {
		out = append(out, map[string]any{
			"id":            strconv.FormatInt(st.ID, 10),
			"name":          st.Name,
			"to":            m.status(st.ID, st.Name, st.Category),
			"hasScreen":     false,
			"isGlobal":      true,
			"isInitial":     st.IsDefault,
			"isAvailable":   true,
			"isConditional": false,
			"isLooped":      false,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"expand": "transitions", "transitions": out})
}

func (h *handler) doTransition(w http.ResponseWriter, r *http.Request) {
	is, ok := h.loadIssue(w, r)
	if !ok {
		return
	}
	var req issueRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var idStr string
	if raw, ok := req.Transition["id"]; ok {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			s = strings.TrimSpace(string(raw))
		}
		idStr = s
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "Missing 'transition' identifier")
		return
	}
	states, err := h.svc.ListStates(r.Context(), is.ProjectKey)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	valid := false
	for _, st := range statesFor(states, domain.IssueType(is.Type)) {
		valid = valid || st.ID == id
	}
	if !valid {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("Transition id '%s' is not valid for this issue.", idStr))
		return
	}
	if _, err := h.svc.UpdateIssue(r.Context(), is.Key, service.UpdateIssueInput{StateID: service.Some(id)}); err != nil {
		writeSvcErr(w, err)
		return
	}
	for _, op := range req.Update["comment"] {
		if add, ok := op["add"].(map[string]any); ok {
			raw, _ := json.Marshal(add["body"])
			if md, err := adf.ToMarkdown(raw); err == nil && md != "" {
				if _, err := h.svc.AddComment(r.Context(), is.Key, md); err != nil {
					writeSvcErr(w, err)
					return
				}
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- comments ----

// commentList merges comments and checkpoints (shown as [Checkpoint]
// comments) in creation order.
func (h *handler) commentList(r *http.Request, is service.Issue, users map[string]service.User, newestFirst bool) ([]any, error) {
	comments, err := h.svc.ListComments(r.Context(), is.Key)
	if err != nil {
		return nil, err
	}
	cps, err := h.svc.ListCheckpoints(r.Context(), is.Key)
	if err != nil {
		return nil, err
	}
	type entry struct {
		at   string
		body map[string]any
	}
	m := h.mapper(r)
	issueID := strconv.FormatInt(is.JiraID, 10)
	var all []entry
	for _, c := range comments {
		all = append(all, entry{c.CreatedAt, m.comment(issueID, c, users)})
	}
	for _, cp := range cps {
		all = append(all, entry{cp.CreatedAt, m.checkpointComment(issueID, cp, users)})
	}
	sort.SliceStable(all, func(i, j int) bool {
		if newestFirst {
			return all[i].at > all[j].at
		}
		return all[i].at < all[j].at
	})
	out := make([]any, len(all))
	for i, e := range all {
		out[i] = e.body
	}
	return out, nil
}

func (h *handler) listComments(w http.ResponseWriter, r *http.Request) {
	is, ok := h.loadIssue(w, r)
	if !ok {
		return
	}
	users, err := h.users(r)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	all, err := h.commentList(r, is, users, strings.HasPrefix(r.URL.Query().Get("orderBy"), "-"))
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	startAt := clamp(intParam(r, "startAt", 0), 0, 1<<30)
	maxResults := clamp(intParam(r, "maxResults", 100), 1, 5000)
	pg := page(all, startAt, maxResults)
	writeJSON(w, http.StatusOK, map[string]any{
		"self":       h.mapper(r).api("/issue/%d/comment", is.JiraID),
		"startAt":    startAt,
		"maxResults": maxResults,
		"total":      len(all),
		"isLast":     startAt+len(pg) >= len(all),
		"comments":   pg,
	})
}

func (h *handler) addComment(w http.ResponseWriter, r *http.Request) {
	is, ok := h.loadIssue(w, r)
	if !ok {
		return
	}
	var req struct {
		Body json.RawMessage `json:"body"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	md, err := adf.ToMarkdown(req.Body)
	if err != nil {
		writeFieldErrs(w, map[string]string{"comment": "Comment body must be an Atlassian Document: " + err.Error()})
		return
	}
	if strings.TrimSpace(md) == "" {
		writeFieldErrs(w, map[string]string{"comment": "Comment body can not be empty!"})
		return
	}
	users, err := h.users(r)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	m := h.mapper(r)
	issueID := strconv.FormatInt(is.JiraID, 10)
	if in, isCheckpoint, err := parseCheckpoint(md); isCheckpoint {
		if err != nil {
			writeFieldErrs(w, map[string]string{"comment": err.Error()})
			return
		}
		cp, err := h.svc.RecordCheckpoint(r.Context(), is.Key, in)
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, m.checkpointComment(issueID, cp, users))
		return
	}
	c, err := h.svc.AddComment(r.Context(), is.Key, md)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m.comment(issueID, c, users))
}

// questionViaMCP explains why Questions cannot be created through Jira.
const questionViaMCP = "Questions (QA) carry structured answer options and are created by agents with the MCP tool ask_question."

// ---- create metadata ----

func (h *handler) creatableTypes() []issueTypeDef {
	// Questions are asked by agents through MCP (they carry structured
	// answer options), so they are not offered for creation here.
	out := []issueTypeDef{}
	for _, d := range issueTypes {
		if d.Type != domain.TypeQA {
			out = append(out, d)
		}
	}
	return out
}

func (h *handler) createMetaTypes(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.projectByParam(w, r); !ok {
		return
	}
	m := h.mapper(r)
	types := []any{}
	for _, d := range h.creatableTypes() {
		types = append(types, m.issueType(d))
	}
	startAt := clamp(intParam(r, "startAt", 0), 0, 1<<30)
	maxResults := clamp(intParam(r, "maxResults", 50), 1, 200)
	pg := page(types, startAt, maxResults)
	writeJSON(w, http.StatusOK, map[string]any{
		"startAt": startAt, "maxResults": maxResults, "total": len(types), "isLast": startAt+len(pg) >= len(types),
		"issueTypes": pg,
	})
}

func (h *handler) createMetaFields(w http.ResponseWriter, r *http.Request) {
	p, ok := h.projectByParam(w, r)
	if !ok {
		return
	}
	typeRef := strings.ToLower(chi.URLParam(r, "type"))
	t, ok := jql.IssueTypes[typeRef]
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Sprintf("Issue type '%s' does not exist.", typeRef))
		return
	}
	m := h.mapper(r)
	field := func(key, name string, required bool, schema map[string]any, allowed []any) map[string]any {
		f := map[string]any{
			"fieldId": key, "key": key, "name": name, "required": required,
			"schema": schema, "hasDefaultValue": false, "operations": []string{"set"},
		}
		if allowed != nil {
			f["allowedValues"] = allowed
		}
		return f
	}
	types := []any{}
	for _, d := range h.creatableTypes() {
		types = append(types, m.issueType(d))
	}
	prios := []any{}
	for _, pr := range priorities {
		prios = append(prios, m.priority(pr))
	}
	fields := []any{
		field("summary", "要約", true, map[string]any{"type": "string", "system": "summary"}, nil),
		field("issuetype", "課題タイプ", true, map[string]any{"type": "issuetype", "system": "issuetype"}, types),
		field("project", "プロジェクト", true, map[string]any{"type": "project", "system": "project"}, []any{m.project(p)}),
		field("description", "説明", false, map[string]any{"type": "string", "system": "description"}, nil),
		field("assignee", "担当者", false, map[string]any{"type": "user", "system": "assignee"}, nil),
		field("priority", "優先度", false, map[string]any{"type": "priority", "system": "priority"}, prios),
		field("labels", "ラベル", false, map[string]any{"type": "array", "items": "string", "system": "labels"}, nil),
		field("duedate", "期日", false, map[string]any{"type": "date", "system": "duedate"}, nil),
	}
	if t != domain.TypeEpic {
		fields = append(fields, field("parent", "親", false, map[string]any{"type": "issuelink", "system": "parent"}, nil))
	}
	startAt := clamp(intParam(r, "startAt", 0), 0, 1<<30)
	maxResults := clamp(intParam(r, "maxResults", 50), 1, 200)
	pg := page(fields, startAt, maxResults)
	writeJSON(w, http.StatusOK, map[string]any{
		"startAt": startAt, "maxResults": maxResults, "total": len(fields), "isLast": startAt+len(pg) >= len(fields),
		"fields": pg,
	})
}

// ---- reference data ----

func (h *handler) listIssueTypes(w http.ResponseWriter, r *http.Request) {
	m := h.mapper(r)
	out := []any{}
	for _, d := range issueTypes {
		out = append(out, m.issueType(d))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handler) listPriorities(w http.ResponseWriter, r *http.Request) {
	m := h.mapper(r)
	out := []any{}
	for _, p := range priorities {
		out = append(out, m.priority(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handler) listStatuses(w http.ResponseWriter, r *http.Request) {
	projects, err := h.projects(r)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	m := h.mapper(r)
	out := []any{}
	for _, p := range projects {
		states, err := h.svc.ListStates(r.Context(), p.Key)
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		for _, st := range states {
			s := m.status(st.ID, st.Name, st.Category)
			s["scope"] = map[string]any{"type": "PROJECT", "project": map[string]any{"id": strconv.FormatInt(p.JiraID, 10)}}
			out = append(out, s)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handler) listResolutions(w http.ResponseWriter, r *http.Request) {
	m := h.mapper(r)
	writeJSON(w, http.StatusOK, []any{m.resolution(string(domain.CategoryCompleted)), m.resolution(string(domain.CategoryCanceled))})
}

// ---- users ----

func (h *handler) getUser(w http.ResponseWriter, r *http.Request) {
	users, err := h.users(r)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	q := r.URL.Query()
	ref := q.Get("accountId")
	if ref == "" {
		ref = q.Get("username")
	}
	u, ok := resolveUser(users, ref)
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Sprintf("Specified user does not exist or you do not have required permissions"))
		return
	}
	writeJSON(w, http.StatusOK, h.mapper(r).user(u))
}

func (h *handler) matchUsers(r *http.Request) ([]any, error) {
	list, err := h.svc.ListUsers(r.Context())
	if err != nil {
		return nil, err
	}
	q := r.URL.Query()
	query := strings.ToLower(strings.TrimSpace(q.Get("query")))
	if query == "" {
		query = strings.ToLower(strings.TrimSpace(q.Get("username")))
	}
	if query == "." {
		query = ""
	}
	accountID := q.Get("accountId")
	var matched []service.User
	for _, u := range list {
		if !u.Active {
			continue
		}
		if accountID != "" && u.ID != accountID {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(u.DisplayName), query) &&
			!strings.Contains(strings.ToLower(u.Username), query) && !strings.Contains(strings.ToLower(u.Email), query) {
			continue
		}
		matched = append(matched, u)
	}
	m := h.mapper(r)
	out := []any{}
	for _, u := range page(matched, clamp(intParam(r, "startAt", 0), 0, 1<<30), clamp(intParam(r, "maxResults", 50), 1, 1000)) {
		out = append(out, m.user(u))
	}
	return out, nil
}

func (h *handler) searchUsers(w http.ResponseWriter, r *http.Request) {
	out, err := h.matchUsers(r)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handler) assignableUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("issueKey") == "" && q.Get("project") == "" && q.Get("issueId") == "" {
		writeErr(w, http.StatusBadRequest, "The issue key, issue id or project must be given.")
		return
	}
	if k := q.Get("issueKey"); k != "" {
		if _, err := h.svc.GetIssue(r.Context(), k); err != nil {
			writeErr(w, http.StatusNotFound, issueNotFound)
			return
		}
	}
	h.searchUsers(w, r)
}

// ---- agile ----

func (h *handler) board(m mapper, p service.Project) map[string]any {
	return map[string]any{
		"id":   p.JiraID,
		"self": m.base + fmt.Sprintf("/rest/agile/1.0/board/%d", p.JiraID),
		"name": p.Key + " ボード",
		"type": "kanban",
		"location": map[string]any{
			"projectId": p.JiraID, "projectKey": p.Key, "projectName": p.Name,
			"displayName": fmt.Sprintf("%s (%s)", p.Name, p.Key), "projectTypeKey": "software", "name": p.Name,
		},
	}
}

func (h *handler) listBoards(w http.ResponseWriter, r *http.Request) {
	projects, err := h.projects(r)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	if ref := r.URL.Query().Get("projectKeyOrId"); ref != "" {
		p, ok := findProject(projects, ref)
		projects = nil
		if ok {
			projects = []service.Project{p}
		}
	}
	m := h.mapper(r)
	startAt := clamp(intParam(r, "startAt", 0), 0, 1<<30)
	maxResults := clamp(intParam(r, "maxResults", 50), 1, 50)
	values := []any{}
	for _, p := range page(projects, startAt, maxResults) {
		values = append(values, h.board(m, p))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"maxResults": maxResults, "startAt": startAt, "total": len(projects),
		"isLast": startAt+len(values) >= len(projects), "values": values,
	})
}

func (h *handler) boardProject(w http.ResponseWriter, r *http.Request) (service.Project, bool) {
	projects, err := h.projects(r)
	if err != nil {
		writeSvcErr(w, err)
		return service.Project{}, false
	}
	id := chi.URLParam(r, "board")
	for _, p := range projects {
		if strconv.FormatInt(p.JiraID, 10) == id {
			return p, true
		}
	}
	writeErr(w, http.StatusNotFound, fmt.Sprintf("Board %s does not exist or you do not have permission to see it.", id))
	return service.Project{}, false
}

func (h *handler) getBoard(w http.ResponseWriter, r *http.Request) {
	p, ok := h.boardProject(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.board(h.mapper(r), p))
}

func (h *handler) boardConfiguration(w http.ResponseWriter, r *http.Request) {
	p, ok := h.boardProject(w, r)
	if !ok {
		return
	}
	states, err := h.svc.ListStates(r.Context(), p.Key)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	m := h.mapper(r)
	columns := []any{}
	for _, st := range statesFor(states, domain.TypeTask) {
		columns = append(columns, map[string]any{
			"name":     st.Name,
			"statuses": []any{map[string]any{"id": strconv.FormatInt(st.ID, 10), "self": m.api("/status/%d", st.ID)}},
		})
	}
	b := h.board(m, p)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": b["id"], "name": b["name"], "self": b["self"].(string) + "/configuration", "type": "kanban",
		"location":     map[string]any{"type": "project", "key": p.Key, "id": strconv.FormatInt(p.JiraID, 10)},
		"filter":       map[string]any{"id": strconv.FormatInt(p.JiraID, 10), "self": m.api("/filter/%d", p.JiraID)},
		"columnConfig": map[string]any{"columns": columns, "constraintType": "none"},
		"ranking":      map[string]any{"rankCustomFieldId": 10019},
	})
}
