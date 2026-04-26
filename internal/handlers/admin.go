package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"randomtube/internal/db"
	"randomtube/internal/middleware"
	"randomtube/internal/youtube"

	"github.com/gorilla/sessions"
)

type AdminHandler struct {
	db       *sql.DB
	tmpl     *Templates
	store    *sessions.CookieStore
	fetcher  *youtube.Fetcher
}

func NewAdminHandler(database *sql.DB, tmpl *Templates, store *sessions.CookieStore, fetcher *youtube.Fetcher) *AdminHandler {
	return &AdminHandler{db: database, tmpl: tmpl, store: store, fetcher: fetcher}
}

// --- Auth ---

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		ok, err := db.CheckAdminPassword(h.db, username, password)
		if err != nil || !ok {
			h.tmpl.Render(w, "admin/login.html", map[string]any{"Error": "Неверный логин или пароль"})
			return
		}

		middleware.SetAuthenticated(h.store, w, r)
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
		return
	}
	h.tmpl.Render(w, "admin/login.html", nil)
}

func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	middleware.Logout(h.store, w, r)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// --- Dashboard ---

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	total, enabled, disabled, _ := db.CountVideos(h.db)
	cats, _ := db.ListCategories(h.db)
	jobs, _ := db.ListImportJobs(h.db)

	h.tmpl.Render(w, "admin/dashboard.html", map[string]any{
		"Total":      total,
		"Enabled":    enabled,
		"Disabled":   disabled,
		"Categories": cats,
		"Jobs":       jobs,
	})
}

// --- Videos ---

func (h *AdminHandler) Videos(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	catCode := r.URL.Query().Get("cat")

	var enabledFilter *bool
	switch r.URL.Query().Get("enabled") {
	case "1":
		t := true
		enabledFilter = &t
	case "0":
		f := false
		enabledFilter = &f
	}

	videos, total, _ := db.ListVideos(h.db, db.VideoFilter{
		CategoryCode: catCode,
		Enabled:      enabledFilter,
		Page:         page,
		PerPage:      50,
	})
	cats, _ := db.ListCategories(h.db)
	pages := (total + 49) / 50

	h.tmpl.Render(w, "admin/videos.html", map[string]any{
		"Videos":     videos,
		"Total":      total,
		"Page":       page,
		"Pages":      pages,
		"CatCode":    catCode,
		"EnabledFilter": r.URL.Query().Get("enabled"),
		"Categories": cats,
	})
}

func (h *AdminHandler) VideoAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()

	action := r.FormValue("action")
	ids := parseIDs(r.Form["ids"])

	if len(ids) == 0 {
		jsonError(w, "нет выбранных видео", http.StatusBadRequest)
		return
	}

	var err error
	switch action {
	case "enable":
		err = db.BulkSetEnabled(h.db, ids, true)
	case "disable":
		err = db.BulkSetEnabled(h.db, ids, false)
	case "delete":
		err = db.BulkDelete(h.db, ids)
	default:
		jsonError(w, "unknown action", http.StatusBadRequest)
		return
	}

	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isAJAX(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

// --- Categories ---

func (h *AdminHandler) Categories(w http.ResponseWriter, r *http.Request) {
	cats, _ := db.ListCategories(h.db)
	h.tmpl.Render(w, "admin/categories.html", map[string]any{"Categories": cats})
}

func (h *AdminHandler) CategoryCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	code := strings.TrimSpace(r.FormValue("code"))
	if name == "" || code == "" {
		http.Redirect(w, r, "/admin/categories?error=empty", http.StatusSeeOther)
		return
	}
	db.CreateCategory(h.db, name, code)
	http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
}

func (h *AdminHandler) CategoryUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	name := strings.TrimSpace(r.FormValue("name"))
	code := strings.TrimSpace(r.FormValue("code"))
	db.UpdateCategory(h.db, id, name, code)
	http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
}

func (h *AdminHandler) CategoryDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	db.DeleteCategory(h.db, id)
	http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
}

// --- Import ---

func (h *AdminHandler) ImportForm(w http.ResponseWriter, r *http.Request) {
	cats, _ := db.ListCategories(h.db)
	jobs, _ := db.ListImportJobs(h.db)
	h.tmpl.Render(w, "admin/import.html", map[string]any{
		"Categories": cats,
		"Jobs":       jobs,
	})
}

func (h *AdminHandler) ImportSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/import", http.StatusSeeOther)
		return
	}

	rawURL := strings.TrimSpace(r.FormValue("url"))
	if rawURL == "" {
		http.Redirect(w, r, "/admin/import?error=empty_url", http.StatusSeeOther)
		return
	}

	var catID *int64
	if s := r.FormValue("category_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			catID = &id
		}
	}

	if h.fetcher == nil {
		http.Redirect(w, r, "/admin/import?error=no_api_key", http.StatusSeeOther)
		return
	}

	jobID, err := db.CreateImportJob(h.db, rawURL, catID)
	if err != nil {
		http.Redirect(w, r, "/admin/import?error=db", http.StatusSeeOther)
		return
	}

	go youtube.RunImport(context.Background(), h.db, h.fetcher, jobID, rawURL, catID)

	http.Redirect(w, r, fmt.Sprintf("/admin/import?job=%d", jobID), http.StatusSeeOther)
}

func (h *AdminHandler) ImportJobStatus(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	job, err := db.GetImportJob(h.db, id)
	if err != nil || job == nil {
		jsonError(w, "job not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":       job.ID,
		"status":   job.Status,
		"total":    job.Total,
		"imported": job.Imported,
		"error":    job.Error,
	})
}

// --- helpers ---

func parseIDs(raw []string) []int64 {
	var ids []int64
	for _, s := range raw {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func isAJAX(r *http.Request) bool {
	return r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(r.Header.Get("Accept"), "application/json")
}
