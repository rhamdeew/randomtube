package db

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(schema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	// Migrate existing videos.category_id → video_categories (idempotent).
	if _, err = db.Exec(`INSERT OR IGNORE INTO video_categories (video_id, category_id)
	                     SELECT id, category_id FROM videos WHERE category_id IS NOT NULL`); err != nil {
		return nil, fmt.Errorf("migrate video categories: %w", err)
	}
	return db, nil
}
