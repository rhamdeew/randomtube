package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"randomtube/internal/db"
	"randomtube/internal/i18n"
	"randomtube/internal/youtube"
)

type PublicHandler struct {
	db        *sql.DB
	templates *Templates
	fetcher   *youtube.Fetcher

	addMu       sync.Mutex
	addLastByIP map[string]time.Time
}

const addRateLimit = 10 * time.Second

func NewPublicHandler(database *sql.DB, tmpl *Templates, fetcher *youtube.Fetcher) *PublicHandler {
	return &PublicHandler{db: database, templates: tmpl, fetcher: fetcher, addLastByIP: map[string]time.Time{}}
}

func (h *PublicHandler) Index(w http.ResponseWriter, r *http.Request) {
	catCode := r.PathValue("code")

	if youtubeID := r.URL.Query().Get("v"); youtubeID != "" {
		h.serveSpecificVideo(w, r, catCode, youtubeID)
		return
	}

	h.serveRandomVideo(w, r, catCode)
}

func (h *PublicHandler) serveRandomVideo(w http.ResponseWriter, r *http.Request, catCode string) {
	video, err := db.RandomVideo(h.db, catCode, "")
	if err != nil || video == nil {
		h.templates.Error(w, r, http.StatusNotFound, "error.video_not_found")
		return
	}

	h.renderVideo(w, r, catCode, video)
}

func (h *PublicHandler) serveSpecificVideo(w http.ResponseWriter, r *http.Request, catCode string, youtubeID string) {
	video, err := db.GetVideoByYoutubeID(h.db, youtubeID)
	if err != nil || video == nil || !video.Enabled {
		basePath := "/"
		if catCode != "" {
			basePath = "/c/" + catCode
		}
		http.Redirect(w, r, basePath, http.StatusFound)
		return
	}

	h.renderVideo(w, r, catCode, video)
}

func (h *PublicHandler) renderVideo(w http.ResponseWriter, r *http.Request, catCode string, video *db.Video) {
	_ = db.IncrementViews(h.db, video.ID)

	cats, _ := db.ListCategories(h.db)
	h.templates.Render(w, r, "public/index.html", map[string]any{
		"Video":        video,
		"CategoryCode": catCode,
		"Categories":   cats,
	})
}

func (h *PublicHandler) Categories(w http.ResponseWriter, r *http.Request) {
	cats, err := db.ListCategories(h.db)
	if err != nil {
		h.templates.Error(w, r, http.StatusInternalServerError, "error.server_error")
		return
	}
	h.templates.Render(w, r, "public/categories.html", map[string]any{
		"Categories": cats,
	})
}

func (h *PublicHandler) Next(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	catCode := r.FormValue("cat")
	currentID := r.FormValue("current")

	video, err := db.RandomVideo(h.db, catCode, currentID)
	if err != nil || video == nil {
		jsonError(w, i18n.T(i18n.Detect(r), "error.no_more_videos"), http.StatusNotFound)
		return
	}

	_ = db.IncrementViews(h.db, video.ID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":    video.YoutubeID,
		"name":  video.Name,
		"views": video.Views + 1,
	})
}

// reportThreshold is how many distinct IPs must report a video as broken
// before it's disabled site-wide. A single report can be a fluke (bad
// connection, a one-off YouTube bot check, or a video that's merely
// region-blocked for that one visitor) — the Player API gives no reliable
// way to tell those apart from a genuinely dead video, so we wait for
// several independent reporters instead of trusting the first one.
const reportThreshold = 5

func (h *PublicHandler) Report(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	youtubeID := r.FormValue("id")
	if youtubeID == "" {
		jsonError(w, "missing id", http.StatusBadRequest)
		return
	}

	lang := i18n.Detect(r)

	if video, err := db.GetVideoByYoutubeID(h.db, youtubeID); err == nil && video != nil {
		if err := db.AddVideoReport(h.db, video.ID, realIP(r)); err == nil {
			if count, err := db.CountVideoReporters(h.db, video.ID); err == nil && count >= reportThreshold {
				_ = db.DisableVideo(h.db, youtubeID)
			}
		}
	}

	// Return next video automatically — regardless of whether the report
	// threshold was reached, this visitor's player still errored and
	// shouldn't be handed the same broken-for-them video again.
	catCode := r.FormValue("cat")
	video, err := db.RandomVideo(h.db, catCode, youtubeID)
	if err != nil || video == nil {
		jsonError(w, i18n.T(lang, "error.no_more_videos"), http.StatusNotFound)
		return
	}

	_ = db.IncrementViews(h.db, video.ID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":   video.YoutubeID,
		"name": video.Name,
	})
}

func (h *PublicHandler) Vote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	youtubeID := r.FormValue("id")
	button := r.FormValue("button") // "like" | "dislike"

	lang := i18n.Detect(r)

	video, err := db.GetVideoByYoutubeID(h.db, youtubeID)
	if err != nil || video == nil {
		jsonError(w, i18n.T(lang, "error.video_not_found"), http.StatusNotFound)
		return
	}

	ip := realIP(r)
	ok, err := db.CanVote(h.db, video.ID, ip)
	if err != nil {
		jsonError(w, i18n.T(lang, "error.server_error"), http.StatusInternalServerError)
		return
	}
	if !ok {
		jsonOK(w, i18n.T(lang, "error.vote_once_per_day"))
		return
	}

	vote := 0
	switch button {
	case "like":
		vote = 1
	case "dislike":
		vote = -1
	}

	if err := db.AddVote(h.db, video.ID, ip, r.UserAgent(), vote); err != nil {
		jsonError(w, i18n.T(lang, "error.save_error"), http.StatusInternalServerError)
		return
	}
	jsonOK(w, i18n.T(lang, "vote.recorded"))
}

func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

// --- Public add page (no admin login required) ---

func (h *PublicHandler) AddForm(w http.ResponseWriter, r *http.Request) {
	cats, _ := db.ListCategories(h.db)
	jobs, _ := db.ListImportJobs(h.db)

	lang := i18n.Detect(r)
	var errMsg string
	switch r.URL.Query().Get("error") {
	case "invalid_url":
		errMsg = i18n.T(lang, "public.add.error.invalid_url")
	case "youtube":
		errMsg = i18n.T(lang, "admin.videos.add_error")
	case "no_api_key":
		errMsg = i18n.T(lang, "public.add.error.no_api_key")
	case "rate_limit":
		errMsg = i18n.T(lang, "public.add.error.rate_limit")
	case "db":
		errMsg = i18n.T(lang, "error.db_error")
	}

	var job *db.ImportJob
	if s := r.URL.Query().Get("job"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			job, _ = db.GetImportJob(h.db, id)
		}
	}

	h.templates.Render(w, r, "public/add.html", map[string]any{
		"Categories": cats,
		"Jobs":       jobs,
		"Error":      errMsg,
		"Added":      r.URL.Query().Get("added") == "1",
		"Job":        job,
	})
}

func (h *PublicHandler) AddSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/add", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/add", http.StatusSeeOther)
		return
	}

	if !h.allowAdd(realIP(r)) {
		http.Redirect(w, r, "/add?error=rate_limit", http.StatusSeeOther)
		return
	}

	if h.fetcher == nil {
		http.Redirect(w, r, "/add?error=no_api_key", http.StatusSeeOther)
		return
	}

	input := strings.TrimSpace(r.FormValue("url"))

	var catID *int64
	if s := r.FormValue("category_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			catID = &id
		}
	}

	if src, err := youtube.ParseURL(input); err == nil && src != nil {
		jobID, err := db.CreateImportJob(h.db, input, catID)
		if err != nil {
			http.Redirect(w, r, "/add?error=db", http.StatusSeeOther)
			return
		}
		go youtube.RunImport(context.Background(), h.db, h.fetcher, jobID, input, catID)
		http.Redirect(w, r, "/add?job="+strconv.FormatInt(jobID, 10), http.StatusSeeOther)
		return
	}

	ytID := extractYouTubeID(input)
	if ytID == "" {
		http.Redirect(w, r, "/add?error=invalid_url", http.StatusSeeOther)
		return
	}

	info, err := h.fetcher.FetchVideosInfo(r.Context(), []string{ytID})
	if err != nil {
		http.Redirect(w, r, "/add?error=youtube", http.StatusSeeOther)
		return
	}
	name, exists := info[ytID]
	if !exists {
		http.Redirect(w, r, "/add?error=youtube", http.StatusSeeOther)
		return
	}

	var catIDs []int64
	if catID != nil {
		catIDs = []int64{*catID}
	}
	if err := db.AddVideo(h.db, ytID, name, catIDs); err != nil {
		http.Redirect(w, r, "/add?error=db", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/add?added=1", http.StatusSeeOther)
}

func (h *PublicHandler) AddJobStatus(w http.ResponseWriter, r *http.Request) {
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

// allowAdd reports whether ip may submit /add now, updating its last-submit
// timestamp if so. Simple in-memory per-IP throttle to deter accidental or
// scripted spam on the public form; resets on process restart.
func (h *PublicHandler) allowAdd(ip string) bool {
	h.addMu.Lock()
	defer h.addMu.Unlock()

	now := time.Now()
	if last, ok := h.addLastByIP[ip]; ok && now.Sub(last) < addRateLimit {
		return false
	}
	h.addLastByIP[ip] = now
	return true
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonOK(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": msg})
}
