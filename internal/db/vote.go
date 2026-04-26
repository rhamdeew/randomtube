package db

import (
	"database/sql"
	"time"
)

func CanVote(db *sql.DB, videoID int64, ip string) (bool, error) {
	var lastVote sql.NullString
	err := db.QueryRow(
		`SELECT created_at FROM votes WHERE video_id = ? AND ip = ? ORDER BY created_at DESC LIMIT 1`,
		videoID, ip,
	).Scan(&lastVote)
	if err == sql.ErrNoRows || !lastVote.Valid {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	t, err := time.Parse("2006-01-02 15:04:05", lastVote.String)
	if err != nil {
		return true, nil
	}
	return time.Since(t) > 24*time.Hour, nil
}

func AddVote(db *sql.DB, videoID int64, ip, useragent string, vote int) error {
	_, err := db.Exec(
		`INSERT INTO votes (video_id, ip, useragent, vote) VALUES (?, ?, ?, ?)`,
		videoID, ip, useragent, vote,
	)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE videos SET rating = rating + ? WHERE id = ?", vote, videoID)
	return err
}
