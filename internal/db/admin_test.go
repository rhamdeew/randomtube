package db_test

import (
	"testing"

	"randomtube/internal/db"
)

func TestCreateAndCheckAdminPassword(t *testing.T) {
	database := newTestDB(t)

	if err := db.CreateAdminUser(database, "admin", "secret"); err != nil {
		t.Fatalf("CreateAdminUser: %v", err)
	}

	ok, err := db.CheckAdminPassword(database, "admin", "secret")
	if err != nil {
		t.Fatalf("CheckAdminPassword: %v", err)
	}
	if !ok {
		t.Error("correct password should return true")
	}
}

func TestCheckAdminPassword_Wrong(t *testing.T) {
	database := newTestDB(t)
	db.CreateAdminUser(database, "admin", "secret")

	ok, _ := db.CheckAdminPassword(database, "admin", "wrong")
	if ok {
		t.Error("wrong password should return false")
	}
}

func TestCheckAdminPassword_UnknownUser(t *testing.T) {
	database := newTestDB(t)

	ok, err := db.CheckAdminPassword(database, "nobody", "pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("unknown user should return false")
	}
}

func TestCreateAdminUser_Upsert(t *testing.T) {
	database := newTestDB(t)
	db.CreateAdminUser(database, "admin", "old")
	db.CreateAdminUser(database, "admin", "new")

	ok, _ := db.CheckAdminPassword(database, "admin", "new")
	if !ok {
		t.Error("upserted password should work")
	}
	old, _ := db.CheckAdminPassword(database, "admin", "old")
	if old {
		t.Error("old password should no longer work after upsert")
	}
}
