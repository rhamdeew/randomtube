package main

import (
	"context"
	"embed"
	"flag"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	dbpkg "randomtube/internal/db"
	"randomtube/internal/handlers"
	"randomtube/internal/middleware"
	"randomtube/internal/youtube"
)

//go:embed all:templates
var templatesFS embed.FS

//go:embed all:static
var staticFS embed.FS

func main() {
	var (
		port      = flag.String("port", envOr("PORT", "8080"), "HTTP port")
		dbPath    = flag.String("db", envOr("DB_PATH", "randomtube.db"), "SQLite database path")
		apiKey    = flag.String("yt-api-key", envOr("YOUTUBE_API_KEY", ""), "YouTube Data API v3 key")
		adminPw   = flag.String("admin-password", envOr("ADMIN_PASSWORD", ""), "Admin password (required)")
		sessKey   = flag.String("session-secret", envOr("SESSION_SECRET", "change-me-in-production"), "Session secret")
		adminUser = flag.String("admin-user", envOr("ADMIN_USER", "admin"), "Admin username")
	)
	flag.Parse()

	if *adminPw == "" {
		log.Fatal("ADMIN_PASSWORD is required (set -admin-password or ADMIN_PASSWORD env var)")
	}

	database, err := dbpkg.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if err := dbpkg.CreateAdminUser(database, *adminUser, *adminPw); err != nil {
		log.Fatalf("create admin user: %v", err)
	}

	tmplFS, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		log.Fatal(err)
	}

	funcMap := template.FuncMap{
		"truncate": func(s string, n int) string {
			runes := []rune(s)
			if len(runes) <= n {
				return s
			}
			return string(runes[:n]) + "…"
		},
		"paginate": func(pages, current int) []int {
			start, end := current-3, current+3
			if start < 1 {
				start = 1
			}
			if end > pages {
				end = pages
			}
			out := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				out = append(out, i)
			}
			return out
		},
	}

	tmpl, err := handlers.NewTemplates(tmplFS, funcMap)
	if err != nil {
		log.Fatalf("load templates: %v", err)
	}

	var fetcher *youtube.Fetcher
	if *apiKey != "" {
		fetcher = youtube.New(*apiKey)
	} else {
		log.Println("WARNING: YOUTUBE_API_KEY not set — import will be disabled")
	}

	store := middleware.NewSessionStore(*sessKey)
	pub := handlers.NewPublicHandler(database, tmpl)
	adm := handlers.NewAdminHandler(database, tmpl, store, fetcher)

	mux := http.NewServeMux()

	// Public
	mux.HandleFunc("GET /{$}", pub.Index)
	mux.HandleFunc("GET /c/{code}", pub.Index)
	mux.HandleFunc("GET /categories", pub.Categories)
	mux.HandleFunc("POST /next", pub.Next)
	mux.HandleFunc("POST /report", pub.Report)
	mux.HandleFunc("POST /vote", pub.Vote)

	// Static files
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// Admin login (public)
	mux.HandleFunc("GET /admin/login", adm.Login)
	mux.HandleFunc("POST /admin/login", adm.Login)
	mux.HandleFunc("POST /admin/logout", adm.Logout)

	// Admin protected
	mux.Handle("/admin/", middleware.RequireAdmin(store, adminRouter(adm)))

	srv := &http.Server{
		Addr:         ":" + *port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Printf("listening on http://localhost:%s", *port)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func adminRouter(adm *handlers.AdminHandler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/", adm.Dashboard)
	mux.HandleFunc("GET /admin/videos", adm.Videos)
	mux.HandleFunc("POST /admin/videos/action", adm.VideoAction)
	mux.HandleFunc("POST /admin/videos/add", adm.VideoAdd)
	mux.HandleFunc("GET /admin/videos/{id}/edit", adm.VideoEdit)
	mux.HandleFunc("POST /admin/videos/{id}/edit", adm.VideoEdit)
	mux.HandleFunc("GET /admin/categories", adm.Categories)
	mux.HandleFunc("POST /admin/categories", adm.CategoryCreate)
	mux.HandleFunc("POST /admin/categories/{id}", adm.CategoryUpdate)
	mux.HandleFunc("POST /admin/categories/{id}/delete", adm.CategoryDelete)
	mux.HandleFunc("GET /admin/import", adm.ImportForm)
	mux.HandleFunc("POST /admin/import", adm.ImportSubmit)
	mux.HandleFunc("GET /admin/import/job/{id}", adm.ImportJobStatus)
	return mux
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
