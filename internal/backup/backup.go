// Package backup takes automatic backups of the SQLite database while the
// server runs (NFR-02): a consistent copy (VACUUM INTO) every interval, kept
// as taskapp-YYYYMMDD-HHMMSS.db in a directory, oldest removed beyond Keep.
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Config controls automatic backups. Interval 0 disables the schedule
// (backups can still be taken on demand).
type Config struct {
	Dir      string
	Interval time.Duration
	Keep     int
	// DataFiles are the database files whose modification time tells whether
	// anything changed since the last backup (taskapp.db and its WAL).
	DataFiles []string
}

// File is one backup in the directory.
type File struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// Status is what the settings page shows.
type Status struct {
	Enabled         bool       `json:"enabled"`
	Dir             string     `json:"dir"`
	IntervalSeconds int64      `json:"interval_seconds"`
	Keep            int        `json:"keep"`
	LastBackupAt    *time.Time `json:"last_backup_at"`
	NextBackupAt    *time.Time `json:"next_backup_at"`
	LastError       string     `json:"last_error"`
	Files           []File     `json:"files"`
}

// Manager takes and lists backups. Safe for concurrent use.
type Manager struct {
	db     *sql.DB
	cfg    Config
	logger *slog.Logger
	now    func() time.Time

	mu      sync.Mutex
	lastErr string
}

func New(db *sql.DB, cfg Config, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Manager{db: db, cfg: cfg, logger: logger, now: time.Now}
}

const layout = "20060102-150405"

var namePattern = regexp.MustCompile(`^taskapp-(\d{8}-\d{6})(?:-(\d+))?\.db$`)

// Dir is the backup directory.
func (m *Manager) Dir() string { return m.cfg.Dir }

// Now takes a backup immediately and removes the oldest beyond Keep.
func (m *Manager) Now(ctx context.Context) (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.take(ctx)
	if err != nil {
		m.lastErr = err.Error()
		return File{}, err
	}
	m.lastErr = ""
	if err := m.prune(); err != nil {
		m.logger.Warn("backup: prune failed", "err", err)
	}
	return f, nil
}

func (m *Manager) take(ctx context.Context) (File, error) {
	if err := os.MkdirAll(m.cfg.Dir, 0o700); err != nil {
		return File{}, err
	}
	at := m.now()
	base := "taskapp-" + at.Format(layout)
	name := base + ".db"
	for i := 1; exists(filepath.Join(m.cfg.Dir, name)); i++ {
		name = fmt.Sprintf("%s-%d.db", base, i)
	}
	// Written under a temporary name and renamed, so an interrupted backup
	// never looks like a complete one.
	final := filepath.Join(m.cfg.Dir, name)
	tmp := final + ".tmp"
	os.Remove(tmp)
	if _, err := m.db.ExecContext(ctx, `VACUUM INTO ?`, tmp); err != nil {
		os.Remove(tmp)
		return File{}, fmt.Errorf("backup: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return File{}, err
	}
	st, err := os.Stat(final)
	if err != nil {
		return File{}, err
	}
	return File{Name: name, Size: st.Size(), CreatedAt: at}, nil
}

// prune removes our own backup files beyond Keep (newest kept). Other files
// in the directory are never touched.
func (m *Manager) prune() error {
	if m.cfg.Keep <= 0 {
		return nil
	}
	files, err := m.List()
	if err != nil {
		return err
	}
	for _, f := range files[min(len(files), m.cfg.Keep):] {
		if err := os.Remove(filepath.Join(m.cfg.Dir, f.Name)); err != nil {
			return err
		}
	}
	return nil
}

// List returns the backups, newest first.
func (m *Manager) List() ([]File, error) {
	entries, err := os.ReadDir(m.cfg.Dir)
	if os.IsNotExist(err) {
		return []File{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := []File{}
	seq := map[string]int{} // same-second suffix, for ordering
	for _, e := range entries {
		match := namePattern.FindStringSubmatch(e.Name())
		if match == nil || e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		at, _ := time.ParseInLocation(layout, match[1], time.Local)
		seq[e.Name()], _ = strconv.Atoi(match[2])
		files = append(files, File{Name: e.Name(), Size: info.Size(), CreatedAt: at})
	}
	sort.Slice(files, func(i, j int) bool {
		if !files[i].CreatedAt.Equal(files[j].CreatedAt) {
			return files[i].CreatedAt.After(files[j].CreatedAt)
		}
		return seq[files[i].Name] > seq[files[j].Name]
	})
	return files, nil
}

// Status reports the configuration, the backups and when the next one is due.
func (m *Manager) Status() (Status, error) {
	files, err := m.List()
	if err != nil {
		return Status{}, err
	}
	m.mu.Lock()
	lastErr := m.lastErr
	m.mu.Unlock()
	st := Status{
		Enabled: m.cfg.Interval > 0, Dir: m.cfg.Dir, IntervalSeconds: int64(m.cfg.Interval / time.Second),
		Keep: m.cfg.Keep, LastError: lastErr, Files: files,
	}
	if len(files) > 0 {
		last := files[0].CreatedAt
		st.LastBackupAt = &last
	}
	if st.Enabled {
		next := m.now()
		if st.LastBackupAt != nil && st.LastBackupAt.Add(m.cfg.Interval).After(next) {
			next = st.LastBackupAt.Add(m.cfg.Interval)
		}
		st.NextBackupAt = &next
	}
	return st, nil
}

// due reports whether a scheduled backup should run now: none yet, or the
// newest is older than Interval and the database changed since then (an idle
// database does not rotate its history away).
func (m *Manager) due() bool {
	files, err := m.List()
	if err != nil || len(files) == 0 {
		return err == nil
	}
	last := files[0].CreatedAt
	if m.now().Sub(last) < m.cfg.Interval {
		return false
	}
	for _, p := range m.cfg.DataFiles {
		if st, err := os.Stat(p); err == nil && st.ModTime().After(last) {
			return true
		}
	}
	return false
}

// Run checks every tick and takes a backup when one is due, until ctx ends.
// A short ticker (rather than one long timer) keeps the schedule right after
// the machine sleeps.
func (m *Manager) Run(ctx context.Context, tick time.Duration) {
	if m.cfg.Interval <= 0 {
		return
	}
	check := func() {
		if !m.due() {
			return
		}
		if f, err := m.Now(ctx); err != nil {
			if ctx.Err() == nil {
				m.logger.Error("automatic backup failed", "err", err)
			}
		} else {
			m.logger.Info("automatic backup", "file", filepath.Join(m.cfg.Dir, f.Name), "size", f.Size)
		}
	}
	check()
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			check()
		}
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
