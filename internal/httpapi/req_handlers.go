package httpapi

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ysksm/task-app/internal/service"
)

// Requirements layer: story map, personas, jobs, activities, steps, stories,
// acceptance criteria (docs/08-devcycle.md §5.4).

func (s *Server) reqRoutes(r chi.Router) {
	r.Get("/projects/{key}/storymap", s.getStoryMap)
	r.Get("/projects/{key}/drafts", s.listDrafts)
	r.Post("/projects/{key}/drafts", reqCreate(s, (*service.Service).CreateDraft))
	r.Get("/projects/{key}/import-plan", s.getImportPlan)
	r.Post("/projects/{key}/import", s.applyImport)
	r.Get("/projects/{key}/releases", s.listReleases)
	r.Post("/projects/{key}/releases", reqCreate(s, (*service.Service).CreateRelease))
	r.Patch("/projects/{key}/releases/{rkey}", reqUpdate(s, (*service.Service).UpdateRelease))
	r.Delete("/projects/{key}/releases/{rkey}", reqArchive(s, (*service.Service).DeleteRelease))
	r.Get("/projects/{key}/personas", s.listPersonas)
	r.Post("/projects/{key}/personas", reqCreate(s, (*service.Service).CreatePersona))
	r.Patch("/projects/{key}/personas/{rkey}", reqUpdate(s, (*service.Service).UpdatePersona))
	r.Delete("/projects/{key}/personas/{rkey}", reqArchive(s, (*service.Service).ArchivePersona))
	r.Get("/projects/{key}/jobs", s.listJobs)
	r.Post("/projects/{key}/jobs", reqCreate(s, (*service.Service).CreateJob))
	r.Patch("/projects/{key}/jobs/{rkey}", reqUpdate(s, (*service.Service).UpdateJob))
	r.Delete("/projects/{key}/jobs/{rkey}", reqArchive(s, (*service.Service).ArchiveJob))
	r.Post("/projects/{key}/activities", reqCreate(s, (*service.Service).CreateActivity))
	r.Patch("/projects/{key}/activities/{rkey}", reqUpdate(s, (*service.Service).UpdateActivity))
	r.Delete("/projects/{key}/activities/{rkey}", reqArchive(s, (*service.Service).ArchiveActivity))
	r.Post("/projects/{key}/activities/{rkey}/steps", reqCreateUnder(s, (*service.Service).CreateStep))
	r.Patch("/projects/{key}/steps/{rkey}", reqUpdate(s, (*service.Service).UpdateStep))
	r.Delete("/projects/{key}/steps/{rkey}", reqArchive(s, (*service.Service).ArchiveStep))
	r.Get("/projects/{key}/stories", s.listStories)
	r.Post("/projects/{key}/stories", reqCreate(s, (*service.Service).CreateStory))
	r.Get("/projects/{key}/stories/{rkey}", s.getStory)
	r.Patch("/projects/{key}/stories/{rkey}", reqUpdate(s, (*service.Service).UpdateStory))
	r.Delete("/projects/{key}/stories/{rkey}", reqArchive(s, (*service.Service).ArchiveStory))
	r.Post("/projects/{key}/stories/{rkey}/criteria", reqCreateUnder(s, (*service.Service).CreateCriterion))
	r.Patch("/projects/{key}/criteria/{rkey}", reqUpdate(s, (*service.Service).UpdateCriterion))
	r.Delete("/projects/{key}/criteria/{rkey}", reqArchive(s, (*service.Service).ArchiveCriterion))
	r.Put("/projects/{key}/stories/{rkey}/issues/{issueKey}", s.linkStoryIssue)
	r.Delete("/projects/{key}/stories/{rkey}/issues/{issueKey}", s.unlinkStoryIssue)
}

type ctxT = context.Context

func reqCreate[In, Out any](s *Server, fn func(*service.Service, ctxT, string, In) (Out, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in In
		if err := decodeJSON(r, &in); err != nil {
			writeError(w, err)
			return
		}
		out, err := fn(s.svc, r.Context(), chi.URLParam(r, "key"), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

func reqCreateUnder[In, Out any](s *Server, fn func(*service.Service, ctxT, string, string, In) (Out, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in In
		if err := decodeJSON(r, &in); err != nil {
			writeError(w, err)
			return
		}
		out, err := fn(s.svc, r.Context(), chi.URLParam(r, "key"), chi.URLParam(r, "rkey"), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

func reqUpdate[In, Out any](s *Server, fn func(*service.Service, ctxT, string, string, In) (Out, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in In
		if err := decodeJSON(r, &in); err != nil {
			writeError(w, err)
			return
		}
		out, err := fn(s.svc, r.Context(), chi.URLParam(r, "key"), chi.URLParam(r, "rkey"), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func reqArchive(s *Server, fn func(*service.Service, ctxT, string, string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(s.svc, r.Context(), chi.URLParam(r, "key"), chi.URLParam(r, "rkey")); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) getStoryMap(w http.ResponseWriter, r *http.Request) {
	m, err := s.svc.GetStoryMap(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) listReleases(w http.ResponseWriter, r *http.Request) {
	rs, err := s.svc.ListReleases(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"releases": rs})
}

func (s *Server) listPersonas(w http.ResponseWriter, r *http.Request) {
	ps, err := s.svc.ListPersonas(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"personas": ps})
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	js, err := s.svc.ListJobs(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": js})
}

func (s *Server) listStories(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.ListStories(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stories": st})
}

func (s *Server) getStory(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.GetStory(r.Context(), chi.URLParam(r, "key"), chi.URLParam(r, "rkey"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) linkStoryIssue(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.LinkStoryIssue(r.Context(), chi.URLParam(r, "key"), chi.URLParam(r, "rkey"), chi.URLParam(r, "issueKey"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) unlinkStoryIssue(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.UnlinkStoryIssue(r.Context(), chi.URLParam(r, "key"), chi.URLParam(r, "rkey"), chi.URLParam(r, "issueKey"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// ---- drafts (§5.9) ----

func (s *Server) listDrafts(w http.ResponseWriter, r *http.Request) {
	ps, err := s.svc.ListDrafts(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"drafts": ps})
}

func (s *Server) getImportPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := s.svc.PlanImport(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) applyImport(w http.ResponseWriter, r *http.Request) {
	var in service.ImportInput
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &in); err != nil {
			writeError(w, err)
			return
		}
	}
	res, err := s.svc.ApplyImport(r.Context(), chi.URLParam(r, "key"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
