package db

import (
	"database/sql"
	"fmt"
)

type ImportJob struct {
	ID         int64
	URL        string
	CategoryID sql.NullInt64
	Status     string
	Total      int
	Imported   int
	Error      string
	CreatedAt  string
	UpdatedAt  string
}

func CreateImportJob(db *sql.DB, url string, categoryID *int64) (int64, error) {
	var res sql.Result
	var err error
	if categoryID != nil {
		res, err = db.Exec(
			`INSERT INTO import_jobs (url, category_id) VALUES (?, ?)`,
			url, *categoryID,
		)
	} else {
		res, err = db.Exec(`INSERT INTO import_jobs (url) VALUES (?)`, url)
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetImportJob(db *sql.DB, id int64) (*ImportJob, error) {
	var j ImportJob
	err := db.QueryRow(
		`SELECT id, url, category_id, status, total, imported, error, created_at, updated_at
		 FROM import_jobs WHERE id = ?`, id,
	).Scan(&j.ID, &j.URL, &j.CategoryID, &j.Status, &j.Total, &j.Imported,
		&j.Error, &j.CreatedAt, &j.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &j, err
}

func ListImportJobs(db *sql.DB) ([]ImportJob, error) {
	rows, err := db.Query(
		`SELECT id, url, category_id, status, total, imported, error, created_at, updated_at
		 FROM import_jobs ORDER BY id DESC LIMIT 50`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []ImportJob
	for rows.Next() {
		var j ImportJob
		if err := rows.Scan(&j.ID, &j.URL, &j.CategoryID, &j.Status, &j.Total,
			&j.Imported, &j.Error, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func UpdateImportJobProgress(db *sql.DB, id int64, total, imported int) error {
	_, err := db.Exec(
		`UPDATE import_jobs SET total = ?, imported = ?, updated_at = datetime('now') WHERE id = ?`,
		total, imported, id,
	)
	return err
}

func FinishImportJob(db *sql.DB, id int64, errMsg string) error {
	status := "done"
	if errMsg != "" {
		status = "error"
	}
	_, err := db.Exec(
		`UPDATE import_jobs SET status = ?, error = ?, updated_at = datetime('now') WHERE id = ?`,
		status, errMsg, id,
	)
	return err
}

func SetImportJobRunning(db *sql.DB, id int64) error {
	_, err := db.Exec(
		`UPDATE import_jobs SET status = 'running', updated_at = datetime('now') WHERE id = ?`, id,
	)
	return err
}

func ImportJobProgress(j *ImportJob) string {
	if j.Total == 0 {
		return fmt.Sprintf("%d", j.Imported)
	}
	return fmt.Sprintf("%d / %d", j.Imported, j.Total)
}
