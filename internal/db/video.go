package db

import (
	"database/sql"
	"fmt"
	"strings"
)

type Video struct {
	ID         int64
	YoutubeID  string
	Name       string
	Categories []Category
	Enabled    bool
	Views      int64
	Rating     int64
	CreatedAt  string
}

type VideoFilter struct {
	CategoryCode string
	Enabled      *bool
	Search       string
	SortBy       string
	SortDir      string
	Page         int
	PerPage      int
}

func RandomVideo(db *sql.DB, categoryCode string, excludeYoutubeID string) (*Video, error) {
	args := []any{}
	where := " WHERE v.enabled = 1"

	join := ""
	if categoryCode != "" {
		join = " JOIN video_categories vc ON vc.video_id = v.id JOIN categories c ON c.id = vc.category_id"
		where += " AND c.code = ?"
		args = append(args, categoryCode)
	}
	if excludeYoutubeID != "" {
		where += " AND v.youtube_id != ?"
		args = append(args, excludeYoutubeID)
	}

	q := `SELECT v.id, v.youtube_id, v.name, v.enabled, v.views, v.rating, v.created_at
	      FROM videos v` + join + where + " ORDER BY RANDOM() LIMIT 1"

	row := db.QueryRow(q, args...)
	v := &Video{}
	var enabled int
	err := row.Scan(&v.ID, &v.YoutubeID, &v.Name, &enabled, &v.Views, &v.Rating, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v.Enabled = enabled == 1
	return v, nil
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

	if f.CategoryCode != "" {
		where += ` AND EXISTS (
			SELECT 1 FROM video_categories vc
			JOIN categories c ON c.id = vc.category_id
			WHERE vc.video_id = v.id AND c.code = ?
		)`
		args = append(args, f.CategoryCode)
	}
	if f.Search != "" {
		where += " AND (v.youtube_id LIKE ? OR v.name LIKE ?)"
		p := "%" + f.Search + "%"
		args = append(args, p, p)
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
	if err := db.QueryRow("SELECT COUNT(*) FROM videos v"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	validSort := map[string]string{
		"id": "v.id", "name": "v.name", "views": "v.views", "rating": "v.rating",
	}
	sortCol, ok := validSort[f.SortBy]
	if !ok {
		sortCol = "v.id"
	}
	sortDir := "DESC"
	if strings.ToLower(f.SortDir) == "asc" {
		sortDir = "ASC"
	}

	offset := (f.Page - 1) * f.PerPage
	q := `SELECT v.id, v.youtube_id, v.name, v.enabled, v.views, v.rating, v.created_at
	      FROM videos v` + where +
		fmt.Sprintf(" ORDER BY %s %s LIMIT ? OFFSET ?", sortCol, sortDir)
	args = append(args, f.PerPage, offset)

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var videos []Video
	var ids []int64
	for rows.Next() {
		var v Video
		var enabled int
		if err := rows.Scan(&v.ID, &v.YoutubeID, &v.Name, &enabled, &v.Views, &v.Rating, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		v.Enabled = enabled == 1
		videos = append(videos, v)
		ids = append(ids, v.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	catsMap, err := loadCategoriesForVideos(db, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range videos {
		if cats, ok := catsMap[videos[i].ID]; ok {
			videos[i].Categories = cats
		}
	}

	return videos, total, nil
}

func GetVideoByID(db *sql.DB, id int64) (*Video, error) {
	v := &Video{}
	var enabled int
	err := db.QueryRow(`SELECT id, youtube_id, name, enabled, views, rating, created_at
	                    FROM videos WHERE id = ?`, id).
		Scan(&v.ID, &v.YoutubeID, &v.Name, &enabled, &v.Views, &v.Rating, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v.Enabled = enabled == 1

	cats, err := getVideoCategories(db, id)
	if err != nil {
		return nil, err
	}
	v.Categories = cats
	return v, nil
}

func GetVideoByYoutubeID(db *sql.DB, youtubeID string) (*Video, error) {
	v := &Video{}
	var enabled int
	err := db.QueryRow(`SELECT id, youtube_id, name, enabled, views, rating, created_at
	                    FROM videos WHERE youtube_id = ?`, youtubeID).
		Scan(&v.ID, &v.YoutubeID, &v.Name, &enabled, &v.Views, &v.Rating, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v.Enabled = enabled == 1
	return v, nil
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

func UpdateVideo(db *sql.DB, id int64, youtubeID, name string) error {
	_, err := db.Exec("UPDATE videos SET youtube_id = ?, name = ? WHERE id = ?", youtubeID, name, id)
	return err
}

func SetVideoCategories(db *sql.DB, videoID int64, categoryIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM video_categories WHERE video_id = ?", videoID); err != nil {
		return err
	}
	for _, catID := range categoryIDs {
		if _, err = tx.Exec("INSERT INTO video_categories (video_id, category_id) VALUES (?, ?)", videoID, catID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AddVideo inserts a video by YouTube ID and assigns categories.
// If the video already exists, only new category associations are added.
func AddVideo(db *sql.DB, youtubeID string, categoryIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.Exec(`INSERT INTO videos (youtube_id, name) VALUES (?, '')
	                     ON CONFLICT(youtube_id) DO NOTHING`, youtubeID); err != nil {
		return err
	}

	var videoID int64
	if err = tx.QueryRow("SELECT id FROM videos WHERE youtube_id = ?", youtubeID).Scan(&videoID); err != nil {
		return err
	}

	for _, catID := range categoryIDs {
		if _, err = tx.Exec("INSERT OR IGNORE INTO video_categories (video_id, category_id) VALUES (?, ?)", videoID, catID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func UpsertVideo(db *sql.DB, youtubeID, name string, categoryID *int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.Exec(`INSERT INTO videos (youtube_id, name) VALUES (?, ?)
	                     ON CONFLICT(youtube_id) DO NOTHING`, youtubeID, name); err != nil {
		return err
	}

	if categoryID != nil {
		if _, err = tx.Exec(`INSERT OR IGNORE INTO video_categories (video_id, category_id)
		                     SELECT id, ? FROM videos WHERE youtube_id = ?`, *categoryID, youtubeID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func CountVideos(db *sql.DB) (total, enabled, disabled int, err error) {
	err = db.QueryRow(`SELECT
		COUNT(*),
		SUM(CASE WHEN enabled=1 THEN 1 ELSE 0 END),
		SUM(CASE WHEN enabled=0 THEN 1 ELSE 0 END)
		FROM videos`).Scan(&total, &enabled, &disabled)
	return
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

func getVideoCategories(db *sql.DB, videoID int64) ([]Category, error) {
	rows, err := db.Query(`SELECT c.id, c.name, c.code
	                       FROM video_categories vc
	                       JOIN categories c ON c.id = vc.category_id
	                       WHERE vc.video_id = ?
	                       ORDER BY c.name`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cats []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Code); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func loadCategoriesForVideos(db *sql.DB, videoIDs []int64) (map[int64][]Category, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}

	placeholders := strings.Repeat("?,", len(videoIDs))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]any, len(videoIDs))
	for i, id := range videoIDs {
		args[i] = id
	}

	rows, err := db.Query(
		`SELECT vc.video_id, c.id, c.name, c.code
		 FROM video_categories vc
		 JOIN categories c ON c.id = vc.category_id
		 WHERE vc.video_id IN (`+placeholders+`)
		 ORDER BY vc.video_id, c.name`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]Category)
	for rows.Next() {
		var videoID int64
		var c Category
		if err := rows.Scan(&videoID, &c.ID, &c.Name, &c.Code); err != nil {
			return nil, err
		}
		result[videoID] = append(result[videoID], c)
	}
	return result, rows.Err()
}
