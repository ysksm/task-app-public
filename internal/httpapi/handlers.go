package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/service"
)

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	a, _ := service.ActorFrom(r.Context())
	writeJSON(w, http.StatusOK, a.User)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.svc.ListUsers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

// ---- projects ----

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	ps, err := s.svc.ListProjects(r.Context(), r.URL.Query().Get("include_archived") == "true")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": ps})
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var in service.CreateProjectInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	p, err := s.svc.CreateProject(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	p, err := s.svc.GetProject(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	var in service.UpdateProjectInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	p, err := s.svc.UpdateProject(r.Context(), chi.URLParam(r, "key"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) listStates(w http.ResponseWriter, r *http.Request) {
	states, err := s.svc.ListStates(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"states": states})
}

func (s *Server) listLabels(w http.ResponseWriter, r *http.Request) {
	labels, err := s.svc.ListLabels(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"labels": labels})
}

func (s *Server) createLabel(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	l, err := s.svc.CreateLabel(r.Context(), chi.URLParam(r, "key"), in.Name, in.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

// ---- issues ----

// listIssues: GET /issues?filter=<json>&project=APP&sort=updated&limit=&offset=&include_ancestors=true
// "project" is a shortcut that is ANDed with the filter.
func (s *Server) listIssues(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f, err := domain.ParseFilter(q.Get("filter"))
	if err != nil {
		writeError(w, service.Errorf(service.CodeInvalidArgument, "%s", err.Error()))
		return
	}
	if p := q.Get("project"); p != "" {
		cond := domain.Filter{Field: domain.FieldProject, Op: domain.OpEq, Value: p}
		if f.IsEmpty() {
			f = cond
		} else {
			f = domain.Filter{And: []domain.Filter{cond, f}}
		}
	}
	opt := service.ListOptions{Filter: f, Sort: q.Get("sort"), IncludeAncestors: q.Get("include_ancestors") == "true"}
	if opt.Limit, err = intParam(q.Get("limit")); err != nil {
		writeError(w, err)
		return
	}
	if opt.Offset, err = intParam(q.Get("offset")); err != nil {
		writeError(w, err)
		return
	}
	res, err := s.svc.ListIssues(r.Context(), opt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func intParam(v string) (int, error) {
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, service.Errorf(service.CodeInvalidArgument, "invalid number %q", v)
	}
	return n, nil
}

func (s *Server) createIssue(w http.ResponseWriter, r *http.Request) {
	var in service.CreateIssueInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	is, err := s.svc.CreateIssue(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, is)
}

func (s *Server) getIssue(w http.ResponseWriter, r *http.Request) {
	is, err := s.svc.GetIssue(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, is)
}

func (s *Server) updateIssue(w http.ResponseWriter, r *http.Request) {
	var in service.UpdateIssueInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	is, err := s.svc.UpdateIssue(r.Context(), chi.URLParam(r, "key"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, is)
}

func (s *Server) deleteIssue(w http.ResponseWriter, r *http.Request) {
	n, err := s.svc.DeleteIssue(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"deleted": n})
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	cs, err := s.svc.ListComments(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": cs})
}

func (s *Server) addComment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	c, err := s.svc.AddComment(r.Context(), chi.URLParam(r, "key"), in.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) listActivity(w http.ResponseWriter, r *http.Request) {
	as, err := s.svc.ListActivity(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"activity": as})
}

// ---- events ----

// streamEvents sends change notifications as server-sent events.
func (s *Server) streamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, service.Errorf(service.CodeInternal, "streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch, unsubscribe := s.svc.Bus().Subscribe()
	defer unsubscribe()
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case e, ok := <-ch:
			if !ok {
				return
			}
			b, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}
