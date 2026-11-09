package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ysksm/task-app/internal/service"
	"github.com/ysksm/task-app/internal/store"
)

// ---- acceptance ----

func (s *Server) getAcceptance(w http.ResponseWriter, r *http.Request) {
	a, err := s.svc.GetAcceptance(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) setCriteria(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Criteria []service.CriterionInput `json:"criteria"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	a, err := s.svc.SetCriteria(r.Context(), chi.URLParam(r, "key"), in.Criteria)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) reviewAcceptance(w http.ResponseWriter, r *http.Request) {
	var in service.ReviewInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	res, err := s.svc.ReviewAcceptance(r.Context(), chi.URLParam(r, "key"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) acceptanceQueue(w http.ResponseWriter, r *http.Request) {
	items, err := s.svc.AcceptanceQueue(r.Context(), r.URL.Query().Get("project"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ---- saved views ----

func (s *Server) listViews(w http.ResponseWriter, r *http.Request) {
	vs, err := s.svc.ListViews(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"views": vs})
}

func (s *Server) createView(w http.ResponseWriter, r *http.Request) {
	var in service.SavedViewInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	v, err := s.svc.CreateView(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (s *Server) getView(w http.ResponseWriter, r *http.Request) {
	v, err := s.svc.GetView(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) updateView(w http.ResponseWriter, r *http.Request) {
	var in service.SavedViewPatch
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	v, err := s.svc.UpdateView(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) deleteView(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteView(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- project settings ----

func stateID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return 0, service.Errorf(service.CodeNotFound, "state %q not found", chi.URLParam(r, "id"))
	}
	return id, nil
}

func (s *Server) createState(w http.ResponseWriter, r *http.Request) {
	var in service.StateInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	st, err := s.svc.CreateState(r.Context(), chi.URLParam(r, "key"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, st)
}

func (s *Server) updateState(w http.ResponseWriter, r *http.Request) {
	id, err := stateID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var in service.StatePatch
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	st, err := s.svc.UpdateState(r.Context(), chi.URLParam(r, "key"), id, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) deleteState(w http.ResponseWriter, r *http.Request) {
	id, err := stateID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteState(r.Context(), chi.URLParam(r, "key"), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) updateLabel(w http.ResponseWriter, r *http.Request) {
	var in service.LabelPatch
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	l, err := s.svc.UpdateLabel(r.Context(), chi.URLParam(r, "key"), chi.URLParam(r, "id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) deleteLabel(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteLabel(r.Context(), chi.URLParam(r, "key"), chi.URLParam(r, "id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- export ----

// exportData downloads the whole database as JSON (owner only).
func (s *Server) exportData(w http.ResponseWriter, r *http.Request) {
	a, _ := service.ActorFrom(r.Context())
	if !a.User.IsOwner {
		writeError(w, service.Errorf(service.CodeForbidden, "only the owner can export"))
		return
	}
	d, err := store.Export(r.Context(), s.svc.DB())
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="taskapp-export-%s.json"`, time.Now().Format("20060102-150405")))
	_ = store.WriteDump(w, d)
}
