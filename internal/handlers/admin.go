package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"randomtube/internal/db"
	"randomtube/internal/i18n"
	"randomtube/internal/middleware"
	"randomtube/internal/youtube"

	"github.com/gorilla/sessions"
)

type AdminHandler struct {
	db      *sql.DB
	tmpl    *Templates
	store   *sessions.CookieStore
	fetcher *youtube.Fetcher
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
			h.tmpl.Render(w, r, "admin/login.html", map[string]any{"Error": i18n.T(i18n.Detect(r), "error.invalid_credentials")})
			return
		}

		if err := middleware.SetAuthenticated(h.store, w, r); err != nil {
			h.tmpl.Render(w, r, "admin/login.html", map[string]any{"Error": i18n.T(i18n.Detect(r), "error.session_error")})
			return
		}
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
		return
	}
	h.tmpl.Render(w, r, "admin/login.html", nil)
}

func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := middleware.Logout(h.store, w, r); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// --- Dashboard ---

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	total, enabled, disabled, _ := db.CountVideos(h.db)
	cats, _ := db.ListCategories(h.db)
	jobs, _ := db.ListImportJobs(h.db)

	h.tmpl.Render(w, r, "admin/dashboard.html", map[string]any{
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
	search := r.URL.Query().Get("search")
	sortBy := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")

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
		Search:       search,
		SortBy:       sortBy,
		SortDir:      sortDir,
		Page:         page,
		PerPage:      50,
	})
	cats, _ := db.ListCategories(h.db)
	pages := (total + 49) / 50

	h.tmpl.Render(w, r, "admin/videos.html", map[string]any{
		"Videos":        videos,
		"Total":         total,
		"Page":          page,
		"Pages":         pages,
		"CatCode":       catCode,
		"EnabledFilter": r.URL.Query().Get("enabled"),
		"Categories":    cats,
		"Search":        search,
		"SortBy":        sortBy,
		"SortDir":       sortDir,
		"Added":         r.URL.Query().Get("added"),
		"Skipped":       r.URL.Query().Get("skipped"),
		"AddError":      r.URL.Query().Get("error"),
	})
}

func (h *AdminHandler) VideoAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		jsonError(w, "invalid form", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")
	ids := parseIDs(r.Form["ids"])

	if len(ids) == 0 {
		jsonError(w, i18n.T(i18n.Detect(r), "error.no_videos_selected"), http.StatusBadRequest)
		return
	}

	var err error
	switch action {
	case "enable":
		err = db.BulkSetEnabled(h.db, ids, true)
		if err == nil {
			for _, id := range ids {
				_ = db.ClearVideoReports(h.db, id)
			}
		}
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
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func (h *AdminHandler) VideoAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/videos", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/videos", http.StatusSeeOther)
		return
	}

	rawText := r.FormValue("urls")
	catIDs := parseCategoryIDs(r.Form["category_ids"])

	var ytIDs []string
	for _, line := range strings.Split(rawText, "\n") {
		if ytID := extractYouTubeID(line); ytID != "" {
			ytIDs = append(ytIDs, ytID)
		}
	}

	titles := make(map[string]string)
	skipped := 0
	if h.fetcher != nil && len(ytIDs) > 0 {
		info, err := h.fetcher.FetchVideosInfo(r.Context(), ytIDs)
		if err != nil {
			http.Redirect(w, r, "/admin/videos?error=youtube", http.StatusSeeOther)
			return
		}
		titles = info
	}

	added := 0
	for _, ytID := range ytIDs {
		name, exists := titles[ytID]
		if h.fetcher != nil && !exists {
			// Video not returned by YouTube: private, deleted, or invalid ID.
			skipped++
			continue
		}
		if err := db.AddVideo(h.db, ytID, name, catIDs); err != nil {
			continue
		}
		added++
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/videos?added=%d&skipped=%d", added, skipped), http.StatusSeeOther)
}

func (h *AdminHandler) VideoEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Redirect(w, r, "/admin/videos", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/videos/%d/edit?error=db", id), http.StatusSeeOther)
			return
		}

		rawID := strings.TrimSpace(r.FormValue("youtube_id"))
		ytID := extractYouTubeID(rawID)
		if ytID == "" {
			http.Redirect(w, r, fmt.Sprintf("/admin/videos/%d/edit?error=invalid_id", id), http.StatusSeeOther)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		catIDs := parseCategoryIDs(r.Form["category_ids"])

		if err := db.UpdateVideo(h.db, id, ytID, name); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/videos/%d/edit?error=db", id), http.StatusSeeOther)
			return
		}
		if err := db.SetVideoCategories(h.db, id, catIDs); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/videos/%d/edit?error=db", id), http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/admin/videos", http.StatusSeeOther)
		return
	}

	video, err := db.GetVideoByID(h.db, id)
	if err != nil || video == nil {
		http.Redirect(w, r, "/admin/videos", http.StatusSeeOther)
		return
	}

	cats, _ := db.ListCategories(h.db)

	selectedCats := make(map[int64]bool)
	for _, c := range video.Categories {
		selectedCats[c.ID] = true
	}

	lang := i18n.Detect(r)
	var errMsg string
	switch r.URL.Query().Get("error") {
	case "invalid_id":
		errMsg = i18n.T(lang, "error.invalid_youtube_id")
	case "db":
		errMsg = i18n.T(lang, "error.db_error")
	}

	h.tmpl.Render(w, r, "admin/video_edit.html", map[string]any{
		"Video":         video,
		"AllCategories": cats,
		"SelectedCats":  selectedCats,
		"Error":         errMsg,
	})
}

// --- Categories ---

func (h *AdminHandler) Categories(w http.ResponseWriter, r *http.Request) {
	cats, _ := db.ListCategories(h.db)
	h.tmpl.Render(w, r, "admin/categories.html", map[string]any{"Categories": cats})
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
	if _, err := db.CreateCategory(h.db, name, code); err != nil {
		http.Redirect(w, r, "/admin/categories?error=db", http.StatusSeeOther)
		return
	}
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
	if err := db.UpdateCategory(h.db, id, name, code); err != nil {
		http.Redirect(w, r, "/admin/categories?error=db", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
}

func (h *AdminHandler) CategoryDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := db.DeleteCategory(h.db, id); err != nil {
		http.Redirect(w, r, "/admin/categories?error=db", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
}

// --- Import ---

func (h *AdminHandler) ImportForm(w http.ResponseWriter, r *http.Request) {
	cats, _ := db.ListCategories(h.db)
	jobs, _ := db.ListImportJobs(h.db)
	h.tmpl.Render(w, r, "admin/import.html", map[string]any{
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
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":       job.ID,
		"status":   job.Status,
		"total":    job.Total,
		"imported": job.Imported,
		"error":    job.Error,
	})
}

// --- helpers ---

var reYTID = regexp.MustCompile(`^[A-Za-z0-9_-]{6,15}$`)

func extractYouTubeID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "youtu") {
		if u, err := url.Parse(raw); err == nil {
			if v := u.Query().Get("v"); reYTID.MatchString(v) {
				return v
			}
			for _, part := range strings.Split(strings.TrimPrefix(u.Path, "/"), "/") {
				if reYTID.MatchString(part) {
					return part
				}
			}
		}
	}
	if reYTID.MatchString(raw) {
		return raw
	}
	return ""
}

func parseCategoryIDs(raw []string) []int64 {
	var ids []int64
	for _, s := range raw {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

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
