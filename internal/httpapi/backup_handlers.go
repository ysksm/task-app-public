package httpapi

import (
	"net/http"

	"github.com/ysksm/task-app/internal/service"
)

// ---- automatic backups (NFR-02) ----

func (s *Server) ownerOnly(w http.ResponseWriter, r *http.Request, what string) bool {
	a, _ := service.ActorFrom(r.Context())
	if !a.User.IsOwner {
		writeError(w, service.Errorf(service.CodeForbidden, "only the owner can %s", what))
		return false
	}
	if s.opts.Backups == nil {
		writeError(w, service.Errorf(service.CodeNotFound, "backups are not available"))
		return false
	}
	return true
}

// backupStatus returns the schedule and the backups in the directory.
func (s *Server) backupStatus(w http.ResponseWriter, r *http.Request) {
	if !s.ownerOnly(w, r, "see backups") {
		return
	}
	st, err := s.opts.Backups.Status()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// createBackup takes a backup now.
func (s *Server) createBackup(w http.ResponseWriter, r *http.Request) {
	if !s.ownerOnly(w, r, "take backups") {
		return
	}
	f, err := s.opts.Backups.Now(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}
