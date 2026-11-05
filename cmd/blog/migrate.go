package main

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed schema.sql migrations/*.sql
var schemaFS embed.FS

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}
	entries, err := fs.Glob(schemaFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)
	for _, path := range entries {
		version := strings.TrimSuffix(path[strings.LastIndex(path, "/")+1:], ".sql")
		var applied int
		if err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version=?", version).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		script, readErr := schemaFS.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		tx, txErr := db.Begin()
		if txErr != nil {
			return txErr
		}
		if _, txErr = tx.Exec(string(script)); txErr == nil {
			_, txErr = tx.Exec("INSERT INTO schema_migrations(version,applied_at) VALUES(?,strftime('%Y-%m-%dT%H:%M:%SZ','now'))", version)
		}
		if txErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", version, txErr)
		}
		if txErr = tx.Commit(); txErr != nil {
			return fmt.Errorf("commit migration %s: %w", version, txErr)
		}
	}
	return nil
}
