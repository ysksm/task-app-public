package store

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// LatestVersion is the newest migration embedded in this binary.
func LatestVersion() int64 {
	var latest int64
	entries, _ := fs.ReadDir(migrations, "migrations")
	for _, e := range entries {
		n, err := strconv.ParseInt(strings.SplitN(e.Name(), "_", 2)[0], 10, 64)
		if err == nil && n > latest {
			latest = n
		}
	}
	return latest
}

// Restore replaces the database in dataDir with a backup file. The current
// database (if any) is first copied aside as taskapp.db.before-restore-<time>,
// which is returned. The server must not be running.
func Restore(ctx context.Context, dataDir, from string) (aside string, err error) {
	if err := checkBackup(ctx, from); err != nil {
		return "", err
	}
	dbFile := filepath.Join(dataDir, DBFileName)
	if a, b := absPath(from), absPath(dbFile); a == b {
		return "", fmt.Errorf("%s is the database itself", from)
	}
	if _, err := os.Stat(dbFile); err == nil {
		// VACUUM INTO includes what is still in the WAL, so the copy aside is complete.
		cur, err := Open(dataDir)
		if err != nil {
			return "", err
		}
		aside = dbFile + ".before-restore-" + time.Now().Format("20060102-150405")
		_, err = cur.ExecContext(ctx, `VACUUM INTO ?`, aside)
		cur.Close()
		if err != nil {
			return "", fmt.Errorf("save the current database: %w", err)
		}
	}
	for _, p := range []string{dbFile, dbFile + "-wal", dbFile + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return aside, err
		}
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return aside, err
	}
	if err := copyFile(from, dbFile); err != nil {
		return aside, err
	}
	// An older backup is brought up to this binary's schema.
	db, err := Open(dataDir)
	if err != nil {
		return aside, err
	}
	defer db.Close()
	return aside, Migrate(ctx, db)
}

// checkBackup makes sure the file is a taskapp database this binary can use.
func checkBackup(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	v, err := SchemaVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("%s is not a taskapp database: %w", path, err)
	}
	if v > LatestVersion() {
		return fmt.Errorf("the backup has schema version %d, newer than this binary (%d); upgrade taskapp first", v, LatestVersion())
	}
	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func absPath(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}
