package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"randomtube/internal/db"
)

type PublicHandler struct {
	db        *sql.DB
	templates *Templates
}

func NewPublicHandler(database *sql.DB, tmpl *Templates) *PublicHandler {
	return &PublicHandler{db: database, templates: tmpl}
}

func (h *PublicHandler) Index(w http.ResponseWriter, r *http.Request) {
	catCode := r.PathValue("code")
	h.serveRandomVideo(w, r, catCode)
}

func (h *PublicHandler) serveRandomVideo(w http.ResponseWriter, r *http.Request, catCode string) {
	video, err := db.RandomVideo(h.db, catCode, "")
	if err != nil || video == nil {
		h.templates.Error(w, r, http.StatusNotFound, "Видео не найдено")
		return
	}

	db.IncrementViews(h.db, video.ID)

	cats, _ := db.ListCategories(h.db)
	h.templates.Render(w, "public/index.html", map[string]any{
		"Video":      video,
		"CategoryCode": catCode,
		"Categories": cats,
	})
}

func (h *PublicHandler) Categories(w http.ResponseWriter, r *http.Request) {
	cats, err := db.ListCategories(h.db)
	if err != nil {
		h.templates.Error(w, r, http.StatusInternalServerError, "Ошибка сервера")
		return
	}
	h.templates.Render(w, "public/categories.html", map[string]any{
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
		jsonError(w, "Нет больше видео", http.StatusNotFound)
		return
	}

	db.IncrementViews(h.db, video.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":    video.YoutubeID,
		"name":  video.Name,
		"views": video.Views + 1,
	})
}

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

	db.DisableVideo(h.db, youtubeID)

	// Return next video automatically
	catCode := r.FormValue("cat")
	video, err := db.RandomVideo(h.db, catCode, youtubeID)
	if err != nil || video == nil {
		jsonError(w, "Нет больше видео", http.StatusNotFound)
		return
	}

	db.IncrementViews(h.db, video.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
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

	video, err := db.GetVideoByYoutubeID(h.db, youtubeID)
	if err != nil || video == nil {
		jsonError(w, "Видео не найдено", http.StatusNotFound)
		return
	}

	ip := realIP(r)
	ok, err := db.CanVote(h.db, video.ID, ip)
	if err != nil {
		jsonError(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}
	if !ok {
		jsonOK(w, "Голосовать можно раз в сутки")
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
		jsonError(w, "Ошибка сохранения", http.StatusInternalServerError)
		return
	}
	jsonOK(w, "Ваш голос учтён")
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

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonOK(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}
