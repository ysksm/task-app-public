package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// Export / import of the whole database as JSON (NFR-02).
//
// The dump is table-level: every row of every application table, keyed by
// column name, so ids (including the numeric Jira ids) survive a round trip
// and Jira clients, MCP agents and saved links keep working after a restore.
// API tokens are included as SHA-256 hashes only. Password hashes are left
// out (humans set their passwords again after an import).

// DumpFormat is the version of the JSON envelope.
const DumpFormat = 1

// tableOrder lists tables parents first, so foreign keys resolve on import.
var tableOrder = []string{
	"users", "api_tokens", "projects", "workflow_states", "labels",
	"issues", "issue_labels", "issue_links", "comments", "checkpoints",
	"qa_details", "qa_items", "acceptance_criteria", "acceptance_reviews", "acceptance_review_items",
	"req_personas", "req_jobs", "req_activities", "req_steps", "req_job_activities", "req_stories",
	"req_story_steps", "req_criteria", "req_story_issues", "req_counters", "req_changes", "req_releases", "req_draft_links",
	"saved_views", "activities", "attachments",
}

// Dump is the exported database.
type Dump struct {
	Format        int                         `json:"format"`
	SchemaVersion int64                       `json:"schema_version"`
	ExportedAt    string                      `json:"exported_at"`
	Tables        map[string][]map[string]any `json:"tables"`
}

// SchemaVersion returns the applied migration version.
func SchemaVersion(ctx context.Context, db *sql.DB) (int64, error) {
	var v sql.NullInt64
	err := db.QueryRowContext(ctx, `SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1`).Scan(&v)
	return v.Int64, err
}

func appTables(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table'
		AND name NOT LIKE 'sqlite_%' AND name NOT IN ('goose_db_version', 'sessions')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	present := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		present[n] = true
	}
	out := []string{}
	for _, t := range tableOrder {
		if present[t] {
			out = append(out, t)
			delete(present, t)
		}
	}
	// Tables added later without updating tableOrder still get exported.
	rest := make([]string, 0, len(present))
	for t := range present {
		rest = append(rest, t)
	}
	sort.Strings(rest)
	return append(out, rest...), rows.Err()
}

// Export reads every application table.
func Export(ctx context.Context, db *sql.DB) (*Dump, error) {
	version, err := SchemaVersion(ctx, db)
	if err != nil {
		return nil, err
	}
	d := &Dump{Format: DumpFormat, SchemaVersion: version, ExportedAt: time.Now().UTC().Format(time.RFC3339), Tables: map[string][]map[string]any{}}
	// One read transaction gives a consistent snapshot.
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	tables, err := appTables(ctx, tx)
	if err != nil {
		return nil, err
	}
	for _, t := range tables {
		rows, err := tx.QueryContext(ctx, `SELECT * FROM "`+t+`" ORDER BY rowid`)
		if err != nil {
			// WITHOUT ROWID tables have no rowid; fall back to the natural order.
			if rows, err = tx.QueryContext(ctx, `SELECT * FROM "`+t+`"`); err != nil {
				return nil, err
			}
		}
		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			return nil, err
		}
		list := []map[string]any{}
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				return nil, err
			}
			row := make(map[string]any, len(cols))
			for i, c := range cols {
				if b, ok := vals[i].([]byte); ok {
					vals[i] = string(b)
				}
				row[c] = vals[i]
			}
			if _, ok := row["password_hash"]; ok && t == "users" {
				row["password_hash"] = ""
			}
			list = append(list, row)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		d.Tables[t] = list
	}
	return d, nil
}

// WriteDump writes a dump as indented JSON.
func WriteDump(w io.Writer, d *Dump) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", " ")
	return enc.Encode(d)
}

// ReadDump parses a dump, keeping numbers exact.
func ReadDump(r io.Reader) (*Dump, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	var d Dump
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("not a taskapp export: %w", err)
	}
	if d.Format != DumpFormat {
		return nil, fmt.Errorf("unsupported export format %d (this binary reads format %d)", d.Format, DumpFormat)
	}
	return &d, nil
}

// ErrNotEmpty is returned when importing into a database that has data.
var ErrNotEmpty = errors.New("the database is not empty; import into a new data directory")

// Import loads a dump into an empty, migrated database.
func Import(ctx context.Context, db *sql.DB, d *Dump) (map[string]int, error) {
	version, err := SchemaVersion(ctx, db)
	if err != nil {
		return nil, err
	}
	if d.SchemaVersion > version {
		return nil, fmt.Errorf("the export has schema version %d, newer than this binary (%d); upgrade taskapp first", d.SchemaVersion, version)
	}
	var users int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		return nil, err
	}
	if users > 0 {
		return nil, ErrNotEmpty
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Rows reference each other (issues.parent_id); check keys at commit.
	if _, err := tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
		return nil, err
	}
	tables, err := appTables(ctx, tx)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, t := range tables {
		rows := d.Tables[t]
		if len(rows) == 0 {
			continue
		}
		cols, err := tableColumns(ctx, tx, t)
		if err != nil {
			return nil, err
		}
		for i, row := range rows {
			names, args := []string{}, []any{}
			for _, c := range cols {
				v, ok := row[c]
				if !ok {
					continue // columns added after the export keep their defaults
				}
				if n, ok := v.(json.Number); ok {
					if iv, err := n.Int64(); err == nil {
						v = iv
					} else if fv, err := n.Float64(); err == nil {
						v = fv
					}
				}
				names, args = append(names, `"`+c+`"`), append(args, v)
			}
			q := `INSERT INTO "` + t + `" (` + strings.Join(names, ",") + `) VALUES (` + strings.TrimSuffix(strings.Repeat("?,", len(args)), ",") + `)`
			if _, err := tx.ExecContext(ctx, q, args...); err != nil {
				return nil, fmt.Errorf("%s row %d: %w", t, i+1, err)
			}
		}
		counts[t] = len(rows)
	}
	for t := range d.Tables {
		if _, ok := counts[t]; !ok && len(d.Tables[t]) > 0 && !contains(tables, t) {
			return nil, fmt.Errorf("the export has a table %q this binary does not know", t)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("import: %w", err)
	}
	return counts, nil
}

func tableColumns(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Backup writes a consistent copy of the database file (works while the
// server is running).
func Backup(ctx context.Context, db *sql.DB, path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	_, err := db.ExecContext(ctx, `VACUUM INTO ?`, path)
	return err
}
