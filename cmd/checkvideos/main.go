// checkvideos scans every enabled video in the database against the YouTube
// Data API and disables the ones YouTube no longer returns (deleted,
// private, region-blocked, copyright takedown, ...). One-off maintenance
// tool, not part of the running app.
//
// Usage:
//
//	go run ./cmd/checkvideos -db randomtube.db -yt-api-key AIza... [-apply]
//
// Without -apply it only prints what it would disable (dry run).
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"

	dbpkg "randomtube/internal/db"
	"randomtube/internal/youtube"
)

func main() {
	dbPath := flag.String("db", "randomtube.db", "SQLite database path")
	apiKey := flag.String("yt-api-key", "", "YouTube Data API v3 key (required)")
	apply := flag.Bool("apply", false, "actually disable unavailable videos (default is dry run)")
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("-yt-api-key is required")
	}

	database, err := dbpkg.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer func() { _ = database.Close() }()

	ids, err := enabledVideoIDs(database)
	if err != nil {
		log.Fatalf("list videos: %v", err)
	}
	log.Printf("checking %d enabled videos against YouTube...", len(ids))

	fetcher := youtube.New(*apiKey)
	ctx := context.Background()

	const chunkSize = 50
	disabled := 0
	for start := 0; start < len(ids); start += chunkSize {
		end := start + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]

		available, err := fetcher.FetchVideosInfo(ctx, chunk)
		if err != nil {
			log.Fatalf("check batch %d-%d: %v", start, end, err)
		}

		for _, id := range chunk {
			if _, ok := available[id]; ok {
				continue
			}
			if *apply {
				if err := dbpkg.DisableVideo(database, id); err != nil {
					log.Printf("disable %s: %v", id, err)
					continue
				}
				log.Printf("disabled: %s", id)
			} else {
				log.Printf("would disable: %s", id)
			}
			disabled++
		}
	}

	if *apply {
		log.Printf("done: disabled %d/%d videos", disabled, len(ids))
	} else {
		log.Printf("dry run: would disable %d/%d videos (rerun with -apply to do it)", disabled, len(ids))
	}
}

func enabledVideoIDs(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT youtube_id FROM videos WHERE enabled = 1`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
