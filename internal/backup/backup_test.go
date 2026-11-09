package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ysksm/task-app/internal/store"
)

func setup(t *testing.T, keep int) (*Manager, string, *time.Time) {
	t.Helper()
	data := t.TempDir()
	db, err := store.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE t (v TEXT); INSERT INTO t VALUES ('a')`); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(data, "backups")
	dbFile := filepath.Join(data, store.DBFileName)
	m := New(db, Config{Dir: dir, Interval: time.Hour, Keep: keep, DataFiles: []string{dbFile, dbFile + "-wal"}}, nil)
	clock := time.Date(2026, 10, 12, 9, 0, 0, 0, time.Local)
	m.now = func() time.Time { return clock }
	return m, dir, &clock
}

func names(t *testing.T, m *Manager) []string {
	t.Helper()
	files, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, f := range files {
		out = append(out, f.Name)
	}
	return out
}

func TestNowNamesAndPrunes(t *testing.T) {
	m, dir, clock := setup(t, 2)
	ctx := context.Background()
	// Files that are not ours are never listed or removed.
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("keep me"), 0o600)
	os.WriteFile(filepath.Join(dir, "taskapp-20261012-080000.db.tmp"), []byte("partial"), 0o600)

	for i := 0; i < 2; i++ {
		if _, err := m.Now(ctx); err != nil {
			t.Fatal(err)
		}
	}
	// Same second: the second file gets a suffix instead of failing.
	if got := names(t, m); len(got) != 2 || got[0] != "taskapp-20261012-090000-1.db" || got[1] != "taskapp-20261012-090000.db" {
		t.Fatalf("names = %v", got)
	}
	*clock = clock.Add(time.Hour)
	f, err := m.Now(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "taskapp-20261012-100000.db" || f.Size == 0 {
		t.Fatalf("file = %+v", f)
	}
	if got := names(t, m); len(got) != 2 || got[0] != f.Name || got[1] != "taskapp-20261012-090000-1.db" {
		t.Fatalf("after prune = %v", got)
	}
	for _, other := range []string{"notes.txt", "taskapp-20261012-080000.db.tmp"} {
		if _, err := os.Stat(filepath.Join(dir, other)); err != nil {
			t.Fatalf("%s was removed", other)
		}
	}
	// The copy is a working database.
	copyDB, err := sql.Open("sqlite", "file:"+filepath.Join(dir, f.Name)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var v string
	if err := copyDB.QueryRow(`SELECT v FROM t`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != "a" {
		t.Fatalf("v = %q", v)
	}
}

func TestDue(t *testing.T) {
	m, _, clock := setup(t, 7)
	if !m.due() {
		t.Fatal("first backup should be due")
	}
	if _, err := m.Now(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Database files are older than the backup: an idle database is not backed up again.
	for _, p := range m.cfg.DataFiles {
		os.Chtimes(p, clock.Add(-time.Minute), clock.Add(-time.Minute))
	}
	*clock = clock.Add(30 * time.Minute)
	if m.due() {
		t.Fatal("due before the interval")
	}
	*clock = clock.Add(2 * time.Hour)
	if m.due() {
		t.Fatal("due although nothing changed")
	}
	os.Chtimes(m.cfg.DataFiles[0], *clock, *clock)
	if !m.due() {
		t.Fatal("not due after a change")
	}
}

func TestStatus(t *testing.T) {
	m, _, clock := setup(t, 3)
	st, err := m.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || st.LastBackupAt != nil || !st.NextBackupAt.Equal(*clock) || st.IntervalSeconds != 3600 || len(st.Files) != 0 {
		t.Fatalf("status = %+v", st)
	}
	m.Now(context.Background())
	st, _ = m.Status()
	if !st.LastBackupAt.Equal(*clock) || !st.NextBackupAt.Equal(clock.Add(time.Hour)) || len(st.Files) != 1 {
		t.Fatalf("status = %+v", st)
	}
}
