// Package i18n provides minimal string translation for the public and admin
// UIs. The active language is detected from the browser's Accept-Language
// header; English is the default when no supported language matches.
package i18n

import (
	"fmt"
	"net/http"
	"strings"
)

// Default is the language used when detection fails or a key/language is missing.
const Default = "en"

var supported = map[string]bool{"en": true, "ru": true}

// Detect picks a supported language from the request's Accept-Language header.
func Detect(r *http.Request) string {
	header := r.Header.Get("Accept-Language")
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		tag = strings.ToLower(tag)
		if idx := strings.IndexAny(tag, "-_"); idx > 0 {
			tag = tag[:idx]
		}
		if supported[tag] {
			return tag
		}
	}
	return Default
}

// T returns the translated string for key in lang, falling back to Default,
// then to the key itself if no translation exists. Extra args are applied
// with fmt.Sprintf when the message contains verbs.
func T(lang, key string, args ...any) string {
	msg, ok := catalog[lang][key]
	if !ok {
		msg, ok = catalog[Default][key]
		if !ok {
			return key
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

var catalog = map[string]map[string]string{
	"en": {
		"nav.categories": "Categories",

		"index.no_videos": "No videos available right now",
		"like_title":      "Like",
		"dislike_title":   "Dislike",
		"next_button":     "More",
		"share_button":    "Share",

		"categories.none": "No categories available right now",

		"error.home_link": "Back to home",

		"admin.nav.home":       "Home",
		"admin.nav.videos":     "Videos",
		"admin.nav.categories": "Categories",
		"admin.nav.import":     "Import",
		"admin.nav.site":       "Site ↗",
		"admin.nav.logout":     "Logout",

		"admin.login.title":    "Login — RandomTube Admin",
		"admin.login.heading":  "Admin Login",
		"admin.login.username": "Username",
		"admin.login.password": "Password",
		"admin.login.submit":   "Login",

		"admin.dashboard.title":            "Dashboard",
		"admin.dashboard.total":            "Total videos",
		"admin.dashboard.enabled":          "Active",
		"admin.dashboard.disabled":         "Disabled",
		"admin.dashboard.categories_count": "Categories",
		"admin.dashboard.recent_jobs":      "Recent import jobs",
		"admin.dashboard.progress":         "Progress",
		"admin.dashboard.no_jobs":          "No jobs yet.",
		"admin.dashboard.import_link":      "Import a channel or playlist",

		"admin.videos.title":              "Videos",
		"admin.videos.import_button":      "+ Import",
		"admin.videos.add_error":          "Failed to verify videos via the YouTube API. Please try again.",
		"admin.videos.added":              "Added",
		"admin.videos.skipped":            "skipped (not found on YouTube)",
		"admin.videos.search":             "Search",
		"admin.videos.search_placeholder": "YouTube ID or title",
		"admin.videos.all_categories":     "All categories",
		"admin.videos.filter_button":      "Filter",
		"admin.videos.add_manual":         "+ Add video manually",
		"admin.videos.urls_label":         "YouTube links (one per line)",
		"admin.videos.categories_label":   "Categories",
		"admin.videos.views":              "Views",
		"admin.videos.rating":             "Rating",

		"admin.categories.add_heading":      "Add category",
		"admin.categories.name_placeholder": "Music",
		"admin.categories.code_label":       "Code (slug)",
		"admin.categories.confirm_delete":   "Delete category",

		"admin.import.title":          "Import channel / playlist",
		"admin.import.description":    "Paste a link to a YouTube channel or playlist.",
		"admin.import.examples":       "Examples:",
		"admin.import.url_label":      "Channel or playlist URL",
		"admin.import.category_label": "Category (optional)",
		"admin.import.no_category":    "No category",
		"admin.import.submit":         "Start import",
		"admin.import.history":        "Import history",
		"admin.import.imported":       "Imported",
		"admin.import.none":           "No imports yet.",

		"admin.video_edit.title":               "Edit video",
		"admin.video_edit.youtube_label":       "YouTube ID or link",
		"admin.video_edit.youtube_placeholder": "dQw4w9WgXcQ or https://youtu.be/...",
		"admin.video_edit.name_placeholder":    "Leave empty if unknown",
		"admin.video_edit.categories_hint":     "(optional)",
		"admin.video_edit.no_categories":       "No categories",

		"admin.common.status":     "Status",
		"admin.common.error":      "Error",
		"admin.common.date":       "Date",
		"admin.common.category":   "Category",
		"admin.common.categories": "Categories",
		"admin.common.all":        "All",
		"admin.common.optional":   "optional",
		"admin.common.add":        "Add",
		"admin.common.total":      "Total",
		"admin.common.enable":     "Enable",
		"admin.common.disable":    "Disable",
		"admin.common.delete":     "Delete",
		"admin.common.name":       "Name",
		"admin.common.code":       "Code",
		"admin.common.actions":    "Actions",
		"admin.common.on":         "on",
		"admin.common.off":        "off",
		"admin.common.edit":       "Edit",
		"admin.common.back":       "← Back",
		"admin.common.save":       "Save",
		"admin.common.cancel":     "Cancel",

		"error.video_not_found":   "Video not found",
		"error.server_error":      "Server error",
		"error.no_more_videos":    "No more videos",
		"error.vote_once_per_day": "You can vote once per day",
		"error.save_error":        "Save error",
		"vote.recorded":           "Your vote has been recorded",

		"error.invalid_credentials": "Invalid username or password",
		"error.session_error":       "Failed to create session",
		"error.no_videos_selected":  "no videos selected",
		"error.invalid_youtube_id":  "Invalid YouTube ID or link",
		"error.db_error":            "Database error",
	},
	"ru": {
		"nav.categories": "Категории",

		"index.no_videos": "Видео временно отсутствуют",
		"like_title":      "Нравится",
		"dislike_title":   "Не нравится",
		"next_button":     "Ещё",
		"share_button":    "Поделиться",

		"categories.none": "Категории временно отсутствуют",

		"error.home_link": "На главную",

		"admin.nav.home":       "Главная",
		"admin.nav.videos":     "Видео",
		"admin.nav.categories": "Категории",
		"admin.nav.import":     "Импорт",
		"admin.nav.site":       "Сайт ↗",
		"admin.nav.logout":     "Выйти",

		"admin.login.title":    "Вход — RandomTube Admin",
		"admin.login.heading":  "Вход в админку",
		"admin.login.username": "Логин",
		"admin.login.password": "Пароль",
		"admin.login.submit":   "Войти",

		"admin.dashboard.title":            "Панель управления",
		"admin.dashboard.total":            "Всего видео",
		"admin.dashboard.enabled":          "Активных",
		"admin.dashboard.disabled":         "Отключённых",
		"admin.dashboard.categories_count": "Категорий",
		"admin.dashboard.recent_jobs":      "Последние задачи импорта",
		"admin.dashboard.progress":         "Прогресс",
		"admin.dashboard.no_jobs":          "Задач нет.",
		"admin.dashboard.import_link":      "Импортировать канал или плейлист",

		"admin.videos.title":              "Видео",
		"admin.videos.import_button":      "+ Импортировать",
		"admin.videos.add_error":          "Не удалось проверить видео через YouTube API. Попробуйте ещё раз.",
		"admin.videos.added":              "Добавлено",
		"admin.videos.skipped":            "пропущено (не найдено на YouTube)",
		"admin.videos.search":             "Поиск",
		"admin.videos.search_placeholder": "YouTube ID или название",
		"admin.videos.all_categories":     "Все категории",
		"admin.videos.filter_button":      "Фильтр",
		"admin.videos.add_manual":         "+ Добавить видео вручную",
		"admin.videos.urls_label":         "Ссылки на YouTube (по одной на строку)",
		"admin.videos.categories_label":   "Категории",
		"admin.videos.views":              "Просмотры",
		"admin.videos.rating":             "Рейтинг",

		"admin.categories.add_heading":      "Добавить категорию",
		"admin.categories.name_placeholder": "Музыка",
		"admin.categories.code_label":       "Код (slug)",
		"admin.categories.confirm_delete":   "Удалить категорию",

		"admin.import.title":          "Импорт канала / плейлиста",
		"admin.import.description":    "Вставьте ссылку на YouTube-канал или плейлист.",
		"admin.import.examples":       "Примеры:",
		"admin.import.url_label":      "URL канала или плейлиста",
		"admin.import.category_label": "Категория (необязательно)",
		"admin.import.no_category":    "Без категории",
		"admin.import.submit":         "Начать импорт",
		"admin.import.history":        "История импортов",
		"admin.import.imported":       "Импортировано",
		"admin.import.none":           "Импортов ещё не было.",

		"admin.video_edit.title":               "Редактировать видео",
		"admin.video_edit.youtube_label":       "YouTube ID или ссылка",
		"admin.video_edit.youtube_placeholder": "dQw4w9WgXcQ или https://youtu.be/...",
		"admin.video_edit.name_placeholder":    "Оставьте пустым, если неизвестно",
		"admin.video_edit.categories_hint":     "(можно не выбирать)",
		"admin.video_edit.no_categories":       "Нет категорий",

		"admin.common.status":     "Статус",
		"admin.common.error":      "Ошибка",
		"admin.common.date":       "Дата",
		"admin.common.category":   "Категория",
		"admin.common.categories": "Категории",
		"admin.common.all":        "Все",
		"admin.common.optional":   "необязательно",
		"admin.common.add":        "Добавить",
		"admin.common.total":      "Всего",
		"admin.common.enable":     "Включить",
		"admin.common.disable":    "Отключить",
		"admin.common.delete":     "Удалить",
		"admin.common.name":       "Название",
		"admin.common.code":       "Код",
		"admin.common.actions":    "Действия",
		"admin.common.on":         "вкл",
		"admin.common.off":        "откл",
		"admin.common.edit":       "Ред.",
		"admin.common.back":       "← Назад",
		"admin.common.save":       "Сохранить",
		"admin.common.cancel":     "Отмена",

		"error.video_not_found":   "Видео не найдено",
		"error.server_error":      "Ошибка сервера",
		"error.no_more_videos":    "Нет больше видео",
		"error.vote_once_per_day": "Голосовать можно раз в сутки",
		"error.save_error":        "Ошибка сохранения",
		"vote.recorded":           "Ваш голос учтён",

		"error.invalid_credentials": "Неверный логин или пароль",
		"error.session_error":       "Не удалось создать сессию",
		"error.no_videos_selected":  "нет выбранных видео",
		"error.invalid_youtube_id":  "Неверный YouTube ID или ссылка",
		"error.db_error":            "Ошибка базы данных",
	},
}
