package db_test

import (
	"testing"

	"randomtube/internal/db"
)

func TestCreateAndListCategories(t *testing.T) {
	database := newTestDB(t)

	id, err := db.CreateCategory(database, "Music", "music")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	db.CreateCategory(database, "Other", "other")

	cats, err := db.ListCategories(database)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(cats) != 2 {
		t.Errorf("expected 2 categories, got %d", len(cats))
	}
}

func TestGetCategoryByCode(t *testing.T) {
	database := newTestDB(t)
	db.CreateCategory(database, "Music", "music")

	cat, err := db.GetCategoryByCode(database, "music")
	if err != nil {
		t.Fatalf("GetCategoryByCode: %v", err)
	}
	if cat == nil || cat.Name != "Music" {
		t.Errorf("unexpected category: %+v", cat)
	}

	missing, _ := db.GetCategoryByCode(database, "nope")
	if missing != nil {
		t.Error("expected nil for missing category")
	}
}

func TestUpdateCategory(t *testing.T) {
	database := newTestDB(t)
	id, _ := db.CreateCategory(database, "Old Name", "old")

	if err := db.UpdateCategory(database, id, "New Name", "new"); err != nil {
		t.Fatalf("UpdateCategory: %v", err)
	}

	cat, _ := db.GetCategoryByCode(database, "new")
	if cat == nil || cat.Name != "New Name" {
		t.Errorf("update didn't work: %+v", cat)
	}
}

func TestDeleteCategory(t *testing.T) {
	database := newTestDB(t)
	id, _ := db.CreateCategory(database, "Music", "music")

	if err := db.DeleteCategory(database, id); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}

	cats, _ := db.ListCategories(database)
	if len(cats) != 0 {
		t.Errorf("expected 0 categories after delete, got %d", len(cats))
	}
}

func TestCategoryUniqueCode(t *testing.T) {
	database := newTestDB(t)
	db.CreateCategory(database, "Music", "music")

	_, err := db.CreateCategory(database, "Music 2", "music")
	if err == nil {
		t.Error("expected error on duplicate code, got nil")
	}
}
