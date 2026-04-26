package handlers_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"randomtube/internal/db"
	"randomtube/internal/handlers"
	"randomtube/internal/middleware"
)

func newTestTemplates(t *testing.T) *handlers.Templates {
	t.Helper()
	fsys := make(fstest.MapFS)
	for name, content := range testTemplates {
		fsys[name] = &fstest.MapFile{Data: []byte(content)}
	}
	tmpl, err := handlers.NewTemplates(fsys, template.FuncMap{
		"truncate": func(s string, n int) string { return s },
		"paginate": func(pages, current int) []int { return nil },
	})
	if err != nil {
		t.Fatalf("NewTemplates: %v", err)
	}
	return tmpl
}

var testTemplates = map[string]string{
	"public/layout.html": `{{define "layout"}}{{template "content" .}}{{end}}`,
	"public/index.html": `{{define "content"}}` +
		`{{if .Video}}<div id="player" data-id="{{.Video.YoutubeID}}"></div>{{else}}<p>no video</p>{{end}}` +
		`{{end}}`,
	"public/categories.html": `{{define "content"}}` +
		`{{range .Categories}}<a href="/c/{{.Code}}">{{.Name}}</a>{{end}}` +
		`{{end}}`,
	"public/error.html": `{{define "content"}}<p class="error">{{.Code}}: {{.Message}}</p>{{end}}`,
	"admin/layout.html": `{{define "layout"}}{{template "content" .}}{{end}}`,
	"admin/login.html":  `{{define "layout"}}<form>{{if .Error}}<p>{{.Error}}</p>{{end}}</form>{{end}}`,
	"admin/dashboard.html": `{{define "content"}}` +
		`<p>total:{{.Total}} enabled:{{.Enabled}} disabled:{{.Disabled}}</p>` +
		`{{end}}`,
	"admin/videos.html":     `{{define "content"}}{{range .Videos}}<tr>{{.YoutubeID}}</tr>{{end}}{{end}}`,
	"admin/categories.html": `{{define "content"}}{{range .Categories}}<li>{{.Name}}</li>{{end}}{{end}}`,
	"admin/import.html":     `{{define "content"}}import form{{end}}`,
	"admin/video_edit.html": `{{define "content"}}edit {{.Video.ID}}{{end}}`,
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func seedVideo(t *testing.T, database *sql.DB, ytID, name string, enabled int) {
	t.Helper()
	if _, err := database.Exec(
		`INSERT INTO videos (youtube_id, name, enabled) VALUES (?, ?, ?)`,
		ytID, name, enabled,
	); err != nil {
		t.Fatalf("seed video %s: %v", ytID, err)
	}
}

func seedCategory(t *testing.T, database *sql.DB, id int, name, code string) {
	t.Helper()
	if _, err := database.Exec(
		`INSERT INTO categories (id, name, code) VALUES (?, ?, ?)`,
		id, name, code,
	); err != nil {
		t.Fatalf("seed category %s: %v", code, err)
	}
}

// --- Public handler tests ---

func TestPublicIndex_ReturnsVideo(t *testing.T) {
	database := newTestDB(t)
	seedVideo(t, database, "abc123", "Test Video", 1)

	tmpl := newTestTemplates(t)
	h := handlers.NewPublicHandler(database, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.Index(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "abc123") {
		t.Errorf("expected youtube ID in response, body: %s", body)
	}
}

func TestPublicIndex_EmptyDB(t *testing.T) {
	database := newTestDB(t)
	tmpl := newTestTemplates(t)
	h := handlers.NewPublicHandler(database, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.Index(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for empty db, got %d", w.Result().StatusCode)
	}
}

func TestPublicNext_ReturnsJSON(t *testing.T) {
	database := newTestDB(t)
	seedVideo(t, database, "vid1", "Video 1", 1)
	seedVideo(t, database, "vid2", "Video 2", 1)

	tmpl := newTestTemplates(t)
	h := handlers.NewPublicHandler(database, tmpl)

	form := url.Values{"current": {"vid1"}, "cat": {""}}
	req := httptest.NewRequest(http.MethodPost, "/next", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Next(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if _, ok := data["id"]; !ok {
		t.Error("response missing 'id' field")
	}
	if data["id"] == "vid1" {
		t.Error("excluded video returned")
	}
}

func TestPublicReport_DisablesVideo(t *testing.T) {
	database := newTestDB(t)
	seedVideo(t, database, "dead1", "Dead Video", 1)
	seedVideo(t, database, "live1", "Live Video", 1)

	tmpl := newTestTemplates(t)
	h := handlers.NewPublicHandler(database, tmpl)

	form := url.Values{"id": {"dead1"}, "cat": {""}}
	req := httptest.NewRequest(http.MethodPost, "/report", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Report(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}

	// dead1 should now be disabled
	v, _ := db.GetVideoByYoutubeID(database, "dead1")
	if v == nil {
		t.Fatal("video should still exist")
	}
	if v.Enabled {
		t.Error("reported video should be disabled")
	}

	// response should contain next video
	var data map[string]any
	json.NewDecoder(w.Body).Decode(&data)
	if id, _ := data["id"].(string); id != "live1" {
		t.Errorf("expected next video live1, got %q", id)
	}
}

func TestPublicReport_OnlyVideo_Returns404(t *testing.T) {
	database := newTestDB(t)
	seedVideo(t, database, "only1", "Only Video", 1)

	tmpl := newTestTemplates(t)
	h := handlers.NewPublicHandler(database, tmpl)

	form := url.Values{"id": {"only1"}, "cat": {""}}
	req := httptest.NewRequest(http.MethodPost, "/report", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Report(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 when no next video, got %d", w.Result().StatusCode)
	}
}

func TestPublicCategories(t *testing.T) {
	database := newTestDB(t)
	seedCategory(t, database, 1, "Music", "music")
	seedCategory(t, database, 2, "Other", "other")

	tmpl := newTestTemplates(t)
	h := handlers.NewPublicHandler(database, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/categories", nil)
	w := httptest.NewRecorder()
	h.Categories(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "music") || !strings.Contains(body, "other") {
		t.Errorf("expected categories in response: %s", body)
	}
}

func TestPublicVote(t *testing.T) {
	database := newTestDB(t)
	seedVideo(t, database, "vid1", "Video 1", 1)

	tmpl := newTestTemplates(t)
	h := handlers.NewPublicHandler(database, tmpl)

	form := url.Values{"id": {"vid1"}, "button": {"like"}}
	req := httptest.NewRequest(http.MethodPost, "/vote", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "1.2.3.4:0"
	w := httptest.NewRecorder()
	h.Vote(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}

	v, _ := db.GetVideoByYoutubeID(database, "vid1")
	if v.Rating != 1 {
		t.Errorf("expected rating 1, got %d", v.Rating)
	}
}

// --- Admin handler tests ---

func TestAdminDashboard(t *testing.T) {
	database := newTestDB(t)
	seedVideo(t, database, "v1", "V1", 1)
	seedVideo(t, database, "v2", "V2", 0)

	tmpl := newTestTemplates(t)
	store := middleware.NewSessionStore("test-secret")
	h := handlers.NewAdminHandler(database, tmpl, store, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	w := httptest.NewRecorder()
	h.Dashboard(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "total:2") {
		t.Errorf("expected total:2 in body, got: %s", body)
	}
	if !strings.Contains(body, "enabled:1") {
		t.Errorf("expected enabled:1 in body, got: %s", body)
	}
	if !strings.Contains(body, "disabled:1") {
		t.Errorf("expected disabled:1 in body, got: %s", body)
	}
}

func TestAdminLogin_WrongPassword(t *testing.T) {
	database := newTestDB(t)
	db.CreateAdminUser(database, "admin", "secret")

	tmpl := newTestTemplates(t)
	store := middleware.NewSessionStore("test-secret")
	h := handlers.NewAdminHandler(database, tmpl, store, nil)

	form := url.Values{"username": {"admin"}, "password": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Login(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Неверный") {
		t.Errorf("expected error message in body, got: %s", body)
	}
}

func TestAdminLogin_CorrectPassword_Redirects(t *testing.T) {
	database := newTestDB(t)
	db.CreateAdminUser(database, "admin", "secret")

	tmpl := newTestTemplates(t)
	store := middleware.NewSessionStore("test-secret")
	h := handlers.NewAdminHandler(database, tmpl, store, nil)

	form := url.Values{"username": {"admin"}, "password": {"secret"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Login(w, req)

	if w.Result().StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", w.Result().StatusCode)
	}
	if loc := w.Result().Header.Get("Location"); loc != "/admin/" {
		t.Errorf("expected redirect to /admin/, got %q", loc)
	}
}

func TestAdminVideoAction_Disable(t *testing.T) {
	database := newTestDB(t)
	seedVideo(t, database, "v1", "V1", 1)

	var videoID int64
	database.QueryRow("SELECT id FROM videos WHERE youtube_id = 'v1'").Scan(&videoID)

	tmpl := newTestTemplates(t)
	store := middleware.NewSessionStore("test-secret")
	h := handlers.NewAdminHandler(database, tmpl, store, nil)

	form := url.Values{
		"action": {"disable"},
		"ids":    {fmt.Sprint(videoID)},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/videos/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "/admin/videos")
	w := httptest.NewRecorder()
	h.VideoAction(w, req)

	v, _ := db.GetVideoByYoutubeID(database, "v1")
	if v.Enabled {
		t.Error("video should be disabled after action")
	}
}
