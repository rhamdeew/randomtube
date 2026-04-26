package db

import (
	"database/sql"
	"fmt"
)

type Video struct {
	ID         int64
	YoutubeID  string
	Name       string
	CategoryID sql.NullInt64
	Enabled    bool
	Views      int64
	Rating     int64
	CreatedAt  string
}

type VideoFilter struct {
	CategoryCode string
	Enabled      *bool
	Page         int
	PerPage      int
}

func RandomVideo(db *sql.DB, categoryCode string, excludeYoutubeID string) (*Video, error) {
	args := []any{}
	q := `SELECT v.id, v.youtube_id, v.name, v.category_id, v.enabled, v.views, v.rating, v.created_at
	      FROM videos v`

	where := " WHERE v.enabled = 1"
	if categoryCode != "" {
		q += " JOIN categories c ON c.id = v.category_id"
		where += " AND c.code = ?"
		args = append(args, categoryCode)
	}
	if excludeYoutubeID != "" {
		where += " AND v.youtube_id != ?"
		args = append(args, excludeYoutubeID)
	}
	q += where + " ORDER BY RANDOM() LIMIT 1"

	row := db.QueryRow(q, args...)
	return scanVideo(row)
}

func ListVideos(db *sql.DB, f VideoFilter) ([]Video, int, error) {
	if f.PerPage <= 0 {
		f.PerPage = 50
	}
	if f.Page <= 0 {
		f.Page = 1
	}

	args := []any{}
	where := " WHERE 1=1"
	join := ""

	if f.CategoryCode != "" {
		join = " JOIN categories c ON c.id = v.category_id"
		where += " AND c.code = ?"
		args = append(args, f.CategoryCode)
	}
	if f.Enabled != nil {
		where += " AND v.enabled = ?"
		if *f.Enabled {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}

	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM videos v"+join+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (f.Page - 1) * f.PerPage
	q := `SELECT v.id, v.youtube_id, v.name, v.category_id, v.enabled, v.views, v.rating, v.created_at
	      FROM videos v` + join + where +
		" ORDER BY v.id DESC LIMIT ? OFFSET ?"
	args = append(args, f.PerPage, offset)

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var videos []Video
	for rows.Next() {
		v, err := scanVideoRow(rows)
		if err != nil {
			return nil, 0, err
		}
		videos = append(videos, *v)
	}
	return videos, total, rows.Err()
}

func GetVideoByYoutubeID(db *sql.DB, youtubeID string) (*Video, error) {
	row := db.QueryRow(`SELECT id, youtube_id, name, category_id, enabled, views, rating, created_at
	                    FROM videos WHERE youtube_id = ?`, youtubeID)
	return scanVideo(row)
}

func IncrementViews(db *sql.DB, id int64) error {
	_, err := db.Exec("UPDATE videos SET views = views + 1 WHERE id = ?", id)
	return err
}

func DisableVideo(db *sql.DB, youtubeID string) error {
	_, err := db.Exec("UPDATE videos SET enabled = 0 WHERE youtube_id = ?", youtubeID)
	return err
}

func SetVideoEnabled(db *sql.DB, id int64, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := db.Exec("UPDATE videos SET enabled = ? WHERE id = ?", v, id)
	return err
}

func DeleteVideo(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM videos WHERE id = ?", id)
	return err
}

func UpsertVideo(db *sql.DB, youtubeID, name string, categoryID *int64) error {
	if categoryID != nil {
		_, err := db.Exec(`INSERT INTO videos (youtube_id, name, category_id)
		                   VALUES (?, ?, ?)
		                   ON CONFLICT(youtube_id) DO NOTHING`,
			youtubeID, name, *categoryID)
		return err
	}
	_, err := db.Exec(`INSERT INTO videos (youtube_id, name)
	                   VALUES (?, ?)
	                   ON CONFLICT(youtube_id) DO NOTHING`,
		youtubeID, name)
	return err
}

func CountVideos(db *sql.DB) (total, enabled, disabled int, err error) {
	err = db.QueryRow(`SELECT
		COUNT(*),
		SUM(CASE WHEN enabled=1 THEN 1 ELSE 0 END),
		SUM(CASE WHEN enabled=0 THEN 1 ELSE 0 END)
		FROM videos`).Scan(&total, &enabled, &disabled)
	return
}

func scanVideo(row *sql.Row) (*Video, error) {
	v := &Video{}
	err := row.Scan(&v.ID, &v.YoutubeID, &v.Name, &v.CategoryID,
		&v.Enabled, &v.Views, &v.Rating, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return v, err
}

func scanVideoRow(rows *sql.Rows) (*Video, error) {
	v := &Video{}
	var enabled int
	err := rows.Scan(&v.ID, &v.YoutubeID, &v.Name, &v.CategoryID,
		&enabled, &v.Views, &v.Rating, &v.CreatedAt)
	v.Enabled = enabled == 1
	return v, err
}

func VideoExists(db *sql.DB, youtubeID string) (bool, error) {
	var n int
	err := db.QueryRow("SELECT COUNT(*) FROM videos WHERE youtube_id = ?", youtubeID).Scan(&n)
	return n > 0, err
}

func BulkSetEnabled(db *sql.DB, ids []int64, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err = tx.Exec("UPDATE videos SET enabled = ? WHERE id = ?", v, id); err != nil {
			return fmt.Errorf("update video %d: %w", id, err)
		}
	}
	return tx.Commit()
}

func BulkDelete(db *sql.DB, ids []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err = tx.Exec("DELETE FROM videos WHERE id = ?", id); err != nil {
			return fmt.Errorf("delete video %d: %w", id, err)
		}
	}
	return tx.Commit()
}
