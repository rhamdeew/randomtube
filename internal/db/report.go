package db

import "database/sql"

// AddVideoReport records that ip reported video as broken/unplayable. A
// second report from the same IP for the same video is a no-op — one
// visitor's repeated player errors shouldn't count as multiple votes to
// disable it.
func AddVideoReport(db *sql.DB, videoID int64, ip string) error {
	_, err := db.Exec(
		`INSERT OR IGNORE INTO video_reports (video_id, ip) VALUES (?, ?)`,
		videoID, ip,
	)
	return err
}

// CountVideoReporters returns how many distinct IPs have reported video.
func CountVideoReporters(db *sql.DB, videoID int64) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM video_reports WHERE video_id = ?`, videoID).Scan(&count)
	return count, err
}

// ClearVideoReports wipes a video's report history. Called when a video is
// manually re-enabled, so it isn't one report away from being disabled
// again the moment a stale/geo-specific error resurfaces.
func ClearVideoReports(db *sql.DB, videoID int64) error {
	_, err := db.Exec(`DELETE FROM video_reports WHERE video_id = ?`, videoID)
	return err
}
