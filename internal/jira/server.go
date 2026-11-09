// Package jira serves a Jira REST API v3 compatible layer (/rest/api/3 and
// /rest/agile/1.0) on top of the service layer, so that Jira clients such as
// ORCA can work with taskapp. Jira concepts (issue type ids, status
// categories, resolutions, transitions, JQL, ADF) stay inside this package.
package jira

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/service"
)

// Handler returns the handler for everything under /rest.
func Handler(svc *service.Service) http.Handler {
	h := &handler{svc: svc}
	r := chi.NewRouter()
	r.Route("/api/3", func(r chi.Router) {
		// Avatars are plain images referenced from <img> tags: no auth.
		r.Get("/avatar/{kind}/{file}", h.avatar)
		r.Group(func(r chi.Router) {
			r.Use(h.authenticate)
			r.Get("/myself", h.myself)
			r.Get("/serverInfo", h.serverInfo)

			r.Get("/project", h.listProjectsArray)
			r.Get("/project/search", h.searchProjects)
			r.Get("/project/{project}", h.getProject)
			r.Get("/project/{project}/statuses", h.projectStatuses)

			r.Get("/search/jql", h.searchJQL)
			r.Post("/search/jql", h.searchJQL)
			r.Get("/search", h.searchLegacy)
			r.Post("/search", h.searchLegacy)

			r.Post("/issue", h.createIssue)
			r.Get("/issue/createmeta/{project}/issuetypes", h.createMetaTypes)
			r.Get("/issue/createmeta/{project}/issuetypes/{type}", h.createMetaFields)
			r.Get("/issue/{issue}", h.getIssue)
			r.Put("/issue/{issue}", h.editIssue)
			r.Put("/issue/{issue}/assignee", h.assign)
			r.Get("/issue/{issue}/transitions", h.listTransitions)
			r.Post("/issue/{issue}/transitions", h.doTransition)
			r.Get("/issue/{issue}/comment", h.listComments)
			r.Post("/issue/{issue}/comment", h.addComment)

			r.Get("/issuetype", h.listIssueTypes)
			r.Get("/priority", h.listPriorities)
			r.Get("/status", h.listStatuses)
			r.Get("/resolution", h.listResolutions)

			r.Get("/user", h.getUser)
			r.Get("/user/search", h.searchUsers)
			r.Get("/user/assignable/search", h.assignableUsers)

			r.Get("/attachment/content/{id}", func(w http.ResponseWriter, r *http.Request) {
				writeErr(w, http.StatusNotFound, "The attachment does not exist.")
			})
		})
	})
	r.Route("/agile/1.0", func(r chi.Router) {
		r.Use(h.authenticate)
		r.Get("/board", h.listBoards)
		r.Get("/board/{board}", h.getBoard)
		r.Get("/board/{board}/configuration", h.boardConfiguration)
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotFound, fmt.Sprintf("taskapp does not implement %s %s", r.Method, r.URL.Path))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Sprintf("taskapp does not implement %s %s", r.Method, r.URL.Path))
	})
	return r
}

// Browse redirects Jira issue links (/browse/APP-1) to the web UI.
func Browse(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := strings.ToUpper(chi.URLParam(r, "key"))
		project, _, ok := domain.SplitIssueKey(key)
		if !ok {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/p/"+project+"/issues/"+key, http.StatusFound)
	}
}

type handler struct{ svc *service.Service }

// ---- auth ----

func (h *handler) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		auth := r.Header.Get("Authorization")
		var user service.User
		var err error
		switch {
		case strings.HasPrefix(auth, "Bearer "):
			user, err = h.svc.Authenticate(ctx, strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")))
		case strings.HasPrefix(auth, "Basic "):
			// Cloud mode: Basic base64(email:api_token). The user part must
			// name the token's owner (email, username or account id).
			name, token, ok := r.BasicAuth()
			if !ok {
				err = errors.New("malformed basic credentials")
				break
			}
			user, err = h.svc.Authenticate(ctx, token)
			if err == nil && !strings.EqualFold(name, user.Email) && name != user.Username && name != user.ID {
				err = errors.New("the user does not own this token")
			}
		default:
			err = errors.New("no credentials")
		}
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="taskapp"`)
			writeErr(w, http.StatusUnauthorized, "You are not authenticated. Authentication required to perform this operation.")
			return
		}
		next.ServeHTTP(w, r.WithContext(service.WithActor(ctx, service.Actor{User: user, Source: domain.SourceJira})))
	})
}

// ---- responses ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr writes Jira's error shape.
func writeErr(w http.ResponseWriter, status int, messages ...string) {
	writeJSON(w, status, map[string]any{"errorMessages": messages, "errors": map[string]string{}})
}

func writeFieldErrs(w http.ResponseWriter, errs map[string]string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{"errorMessages": []string{}, "errors": errs})
}

// writeSvcErr maps a service error to a Jira error response.
func writeSvcErr(w http.ResponseWriter, err error) {
	e := service.AsError(err)
	status := e.HTTPStatus()
	switch e.Code {
	case service.CodeHierarchyViolation:
		status = http.StatusBadRequest
	case service.CodeConflict:
		status = http.StatusBadRequest
	}
	writeErr(w, status, e.Message)
}

const issueNotFound = "Issue does not exist or you do not have permission to see it."

func (h *handler) mapper(r *http.Request) mapper {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	}
	return mapper{base: scheme + "://" + r.Host}
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("The request body is not valid JSON: %v", err)
	}
	return nil
}

// ---- lookups ----

func (h *handler) users(r *http.Request) (map[string]service.User, error) {
	list, err := h.svc.ListUsers(r.Context())
	if err != nil {
		return nil, err
	}
	m := make(map[string]service.User, len(list))
	for _, u := range list {
		m[u.ID] = u
	}
	return m, nil
}

// resolveUser finds a user by account id, email or username.
func resolveUser(users map[string]service.User, ref string) (service.User, bool) {
	if u, ok := users[ref]; ok {
		return u, true
	}
	for _, u := range users {
		if (u.Email != "" && strings.EqualFold(u.Email, ref)) || u.Username == ref {
			return u, true
		}
	}
	return service.User{}, false
}

func (h *handler) projects(r *http.Request) ([]service.Project, error) {
	return h.svc.ListProjects(r.Context(), false)
}

// findProject accepts a key, a Jira numeric id or an internal id.
func findProject(list []service.Project, ref string) (service.Project, bool) {
	for _, p := range list {
		if strings.EqualFold(p.Key, ref) || p.ID == ref || strconv.FormatInt(p.JiraID, 10) == ref {
			return p, true
		}
	}
	return service.Project{}, false
}

// issueKey resolves an issue path parameter (key or numeric id).
func (h *handler) issueKey(r *http.Request, ref string) (string, error) {
	if n, err := strconv.ParseInt(ref, 10, 64); err == nil {
		return h.svc.IssueKeyByJiraID(r.Context(), n)
	}
	return strings.ToUpper(ref), nil
}

func (h *handler) loadIssue(w http.ResponseWriter, r *http.Request) (service.Issue, bool) {
	key, err := h.issueKey(r, chi.URLParam(r, "issue"))
	if err == nil {
		var is service.Issue
		if is, err = h.svc.GetIssue(r.Context(), key); err == nil {
			return is, true
		}
	}
	if service.AsError(err).Code == service.CodeNotFound {
		writeErr(w, http.StatusNotFound, issueNotFound)
	} else {
		writeSvcErr(w, err)
	}
	return service.Issue{}, false
}

// ---- paging ----

func intParam(r *http.Request, name string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil {
		return v
	}
	return def
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func encodeToken(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("o:" + strconv.Itoa(offset)))
}

func decodeToken(tok string) (int, bool) {
	b, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil || !strings.HasPrefix(string(b), "o:") {
		return 0, false
	}
	n, err := strconv.Atoi(string(b[2:]))
	return n, err == nil && n >= 0
}

// page applies startAt/maxResults to a slice.
func page[T any](items []T, startAt, maxResults int) []T {
	if startAt >= len(items) {
		return []T{}
	}
	end := startAt + maxResults
	if end > len(items) {
		end = len(items)
	}
	return items[startAt:end]
}
