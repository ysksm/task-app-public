package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ysksm/task-app/internal/service"
)

// ---- checkpoints ----

func (s *Server) listCheckpoints(w http.ResponseWriter, r *http.Request) {
	cps, err := s.svc.ListCheckpoints(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"checkpoints": cps})
}

func (s *Server) recordCheckpoint(w http.ResponseWriter, r *http.Request) {
	var in service.CheckpointInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	cp, err := s.svc.RecordCheckpoint(r.Context(), chi.URLParam(r, "key"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cp)
}

// ---- QA ----

// listQA: GET /qa?status=open|answered|closed&asked_by=me&assignee=me&target=APP-3&project=APP&limit=
func (s *Server) listQA(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := intParam(q.Get("limit"))
	if err != nil {
		writeError(w, err)
		return
	}
	list, err := s.svc.ListQA(r.Context(), service.ListQAOptions{Status: q.Get("status"), AskedBy: q.Get("asked_by"),
		Assignee: q.Get("assignee"), TargetKey: q.Get("target"), ProjectKey: q.Get("project"), Limit: limit})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"questions": list})
}

func (s *Server) createQA(w http.ResponseWriter, r *http.Request) {
	var in service.CreateQAInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	q, err := s.svc.CreateQA(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, q)
}

func (s *Server) getQA(w http.ResponseWriter, r *http.Request) {
	q, err := s.svc.GetQA(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (s *Server) answerQA(w http.ResponseWriter, r *http.Request) {
	var in service.AnswerInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	q, err := s.svc.AnswerQA(r.Context(), chi.URLParam(r, "key"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (s *Server) closeQA(w http.ResponseWriter, r *http.Request) {
	q, err := s.svc.CloseQA(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

// ---- agent activity ----

func (s *Server) agentActivity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := intParam(q.Get("limit"))
	if err != nil {
		writeError(w, err)
		return
	}
	feed, err := s.svc.AgentActivity(r.Context(), service.FeedOptions{Agent: q.Get("agent"), ProjectKey: q.Get("project"),
		LowConfidence: q.Get("low_confidence") == "true", Limit: limit})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": feed})
}

// ---- users and tokens ----

func (s *Server) addUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind        string `json:"kind"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		AgentModel  string `json:"agent_model"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	u, err := s.svc.AddUser(r.Context(), service.CreateUserInput{Kind: in.Kind, Username: in.Username, DisplayName: in.DisplayName,
		Email: in.Email, AgentModel: in.AgentModel})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) listTokens(w http.ResponseWriter, r *http.Request) {
	ts, err := s.svc.ListTokens(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": ts})
}

func (s *Server) issueToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		User string `json:"user"`
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	raw, t, err := s.svc.IssueToken(r.Context(), in.User, in.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": raw, "info": t})
}

func (s *Server) revealToken(w http.ResponseWriter, r *http.Request) {
	raw, err := s.svc.RevealToken(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": raw})
}

func (s *Server) revokeToken(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.RevokeToken(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
