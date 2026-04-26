package db_test

import (
	"testing"

	"randomtube/internal/db"
)

func seedData(t *testing.T, database interface {
	Exec(string, ...any) (interface{ LastInsertId() (int64, error) }, error)
}) {
	t.Helper()
}

func TestRandomVideo_NoFilter(t *testing.T) {
	database := newTestDB(t)
	mustExec(t, database, `INSERT INTO categories (id, name, code) VALUES (1, 'Music', 'music')`)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, category_id, enabled) VALUES ('aaa111', 'Video A', 1, 1)`)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, category_id, enabled) VALUES ('bbb222', 'Video B', 1, 1)`)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, category_id, enabled) VALUES ('ccc333', 'Video C', 1, 0)`)

	v, err := db.RandomVideo(database, "", "")
	if err != nil {
		t.Fatalf("RandomVideo: %v", err)
	}
	if v == nil {
		t.Fatal("expected a video, got nil")
	}
	if v.YoutubeID == "ccc333" {
		t.Error("disabled video should not be returned")
	}
}

func TestRandomVideo_ByCategory(t *testing.T) {
	database := newTestDB(t)
	mustExec(t, database, `INSERT INTO categories (id, name, code) VALUES (1, 'Music', 'music')`)
	mustExec(t, database, `INSERT INTO categories (id, name, code) VALUES (2, 'Other', 'other')`)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, category_id, enabled) VALUES ('aaa111', 'Music Video', 1, 1)`)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, category_id, enabled) VALUES ('bbb222', 'Other Video', 2, 1)`)

	v, err := db.RandomVideo(database, "music", "")
	if err != nil {
		t.Fatalf("RandomVideo by category: %v", err)
	}
	if v == nil {
		t.Fatal("expected a video, got nil")
	}
	if v.YoutubeID != "aaa111" {
		t.Errorf("expected aaa111, got %s", v.YoutubeID)
	}
}

func TestRandomVideo_Exclude(t *testing.T) {
	database := newTestDB(t)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, enabled) VALUES ('aaa111', 'Video A', 1)`)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, enabled) VALUES ('bbb222', 'Video B', 1)`)

	v, err := db.RandomVideo(database, "", "aaa111")
	if err != nil {
		t.Fatalf("RandomVideo with exclude: %v", err)
	}
	if v == nil {
		t.Fatal("expected a video, got nil")
	}
	if v.YoutubeID == "aaa111" {
		t.Error("excluded video should not be returned")
	}
}

func TestRandomVideo_NoResults(t *testing.T) {
	database := newTestDB(t)

	v, err := db.RandomVideo(database, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != nil {
		t.Errorf("expected nil, got %+v", v)
	}
}

func TestDisableVideo(t *testing.T) {
	database := newTestDB(t)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, enabled) VALUES ('aaa111', 'Video A', 1)`)

	if err := db.DisableVideo(database, "aaa111"); err != nil {
		t.Fatalf("DisableVideo: %v", err)
	}

	v, _ := db.RandomVideo(database, "", "")
	if v != nil {
		t.Error("disabled video should not appear in random query")
	}
}

func TestIncrementViews(t *testing.T) {
	database := newTestDB(t)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, enabled) VALUES ('aaa111', 'Video A', 1)`)

	v, _ := db.RandomVideo(database, "", "")
	before := v.Views

	if err := db.IncrementViews(database, v.ID); err != nil {
		t.Fatalf("IncrementViews: %v", err)
	}

	v2, _ := db.GetVideoByYoutubeID(database, "aaa111")
	if v2.Views != before+1 {
		t.Errorf("expected views %d, got %d", before+1, v2.Views)
	}
}

func TestUpsertVideo_Dedup(t *testing.T) {
	database := newTestDB(t)

	if err := db.UpsertVideo(database, "aaa111", "First Title", nil); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := db.UpsertVideo(database, "aaa111", "Duplicate", nil); err != nil {
		t.Fatalf("duplicate upsert: %v", err)
	}

	_, total, _ := db.ListVideos(database, db.VideoFilter{})
	if total != 1 {
		t.Errorf("expected 1 video after dedup, got %d", total)
	}
}

func TestListVideos_Pagination(t *testing.T) {
	database := newTestDB(t)
	for i := 0; i < 10; i++ {
		mustExec(t, database,
			`INSERT INTO videos (youtube_id, name, enabled) VALUES (?, ?, 1)`,
			"id"+string(rune('a'+i)), "Video",
		)
	}

	page1, total, err := db.ListVideos(database, db.VideoFilter{Page: 1, PerPage: 4})
	if err != nil {
		t.Fatalf("ListVideos: %v", err)
	}
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}
	if len(page1) != 4 {
		t.Errorf("expected 4 results on page 1, got %d", len(page1))
	}

	page3, _, _ := db.ListVideos(database, db.VideoFilter{Page: 3, PerPage: 4})
	if len(page3) != 2 {
		t.Errorf("expected 2 results on page 3, got %d", len(page3))
	}
}

func TestListVideos_EnabledFilter(t *testing.T) {
	database := newTestDB(t)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, enabled) VALUES ('on1', 'On', 1)`)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, enabled) VALUES ('off1', 'Off', 0)`)

	enabled := true
	videos, _, _ := db.ListVideos(database, db.VideoFilter{Enabled: &enabled})
	if len(videos) != 1 || videos[0].YoutubeID != "on1" {
		t.Errorf("enabled filter failed: %+v", videos)
	}

	disabled := false
	videos, _, _ = db.ListVideos(database, db.VideoFilter{Enabled: &disabled})
	if len(videos) != 1 || videos[0].YoutubeID != "off1" {
		t.Errorf("disabled filter failed: %+v", videos)
	}
}

func TestBulkSetEnabled(t *testing.T) {
	database := newTestDB(t)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, enabled) VALUES ('v1', 'V1', 1)`)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, enabled) VALUES ('v2', 'V2', 1)`)

	var ids []int64
	rows, _ := database.Query("SELECT id FROM videos")
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()

	if err := db.BulkSetEnabled(database, ids, false); err != nil {
		t.Fatalf("BulkSetEnabled: %v", err)
	}

	v, _ := db.RandomVideo(database, "", "")
	if v != nil {
		t.Error("all videos disabled, RandomVideo should return nil")
	}
}
