package db_test

import (
	"testing"

	"randomtube/internal/db"
)

func TestCanVote_FirstTime(t *testing.T) {
	database := newTestDB(t)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, enabled) VALUES ('aaa111', 'V', 1)`)

	ok, err := db.CanVote(database, 1, "1.2.3.4")
	if err != nil {
		t.Fatalf("CanVote: %v", err)
	}
	if !ok {
		t.Error("first vote should be allowed")
	}
}

func TestCanVote_TwiceInRow(t *testing.T) {
	database := newTestDB(t)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, enabled) VALUES ('aaa111', 'V', 1)`)

	db.AddVote(database, 1, "1.2.3.4", "UA", 1)

	ok, err := db.CanVote(database, 1, "1.2.3.4")
	if err != nil {
		t.Fatalf("CanVote: %v", err)
	}
	if ok {
		t.Error("second vote within 24h should be blocked")
	}
}

func TestCanVote_DifferentIP(t *testing.T) {
	database := newTestDB(t)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, enabled) VALUES ('aaa111', 'V', 1)`)

	db.AddVote(database, 1, "1.2.3.4", "UA", 1)

	ok, _ := db.CanVote(database, 1, "9.9.9.9")
	if !ok {
		t.Error("different IP should be allowed to vote")
	}
}

func TestAddVote_UpdatesRating(t *testing.T) {
	database := newTestDB(t)
	mustExec(t, database, `INSERT INTO videos (youtube_id, name, enabled) VALUES ('aaa111', 'V', 1)`)

	db.AddVote(database, 1, "1.1.1.1", "UA", 1)
	db.AddVote(database, 1, "2.2.2.2", "UA", 1)
	db.AddVote(database, 1, "3.3.3.3", "UA", -1)

	v, _ := db.GetVideoByYoutubeID(database, "aaa111")
	if v.Rating != 1 {
		t.Errorf("expected rating 1, got %d", v.Rating)
	}
}
