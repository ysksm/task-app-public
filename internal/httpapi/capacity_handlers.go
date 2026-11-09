package httpapi

import (
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ysksm/task-app/internal/diskusage"
	"github.com/ysksm/task-app/internal/service"
	"github.com/ysksm/task-app/internal/store"
)

// capacity reports how big the data is and how much room is left, for the
// settings page to compare with the scale taskapp was checked at (owner only).
func (s *Server) capacity(w http.ResponseWriter, r *http.Request) {
	a, _ := service.ActorFrom(r.Context())
	if !a.User.IsOwner {
		writeError(w, service.Errorf(service.CodeForbidden, "only the owner can see the capacity"))
		return
	}
	counts, err := s.svc.Counts(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	storage := map[string]any{"db_bytes": nil, "backups_bytes": nil, "data_dir_bytes": nil, "disk_total_bytes": nil, "disk_free_bytes": nil}
	if dir := s.opts.DataDir; dir != "" {
		db := filepath.Join(dir, store.DBFileName)
		storage["db_bytes"] = diskusage.FileBytes(db) + diskusage.FileBytes(db+"-wal") + diskusage.FileBytes(db+"-shm")
		storage["data_dir_bytes"] = diskusage.DirBytes(dir)
		if total, free, ok := diskusage.Disk(dir); ok {
			storage["disk_total_bytes"], storage["disk_free_bytes"] = total, free
		}
	}
	if s.opts.Backups != nil {
		if st, err := s.opts.Backups.Status(); err == nil {
			storage["backups_bytes"] = diskusage.DirBytes(st.Dir)
		}
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	writeJSON(w, http.StatusOK, map[string]any{
		"counts":      counts,
		"storage":     storage,
		"memory":      map[string]uint64{"sys_bytes": m.Sys, "heap_bytes": m.HeapAlloc},
		"measured_at": time.Now().UTC().Format(time.RFC3339),
	})
}
