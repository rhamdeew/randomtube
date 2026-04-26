# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make run-dev          # Build and run with dev defaults (ADMIN_PASSWORD=dev)
make test             # Run all tests
make build            # Build binary
make test-coverage    # Run tests with HTML coverage report
```

Run a single test or package:
```bash
go test -v ./internal/db/ -run TestFunctionName
go test -v ./internal/handlers/
```

Migrate a MySQL dump to SQLite:
```bash
make migrate DUMP=/path/to/dump.sql DB=randomtube.db
```

## Runtime Configuration

All config is via flags or env vars (env takes precedence):

| Flag | Env | Default |
|------|-----|---------|
| `-admin-password` | `ADMIN_PASSWORD` | *(required)* |
| `-db` | `DB_PATH` | `randomtube.db` |
| `-yt-api-key` | `YOUTUBE_API_KEY` | *(import disabled if unset)* |
| `-session-secret` | `SESSION_SECRET` | `change-me-in-production` |
| `-admin-user` | `ADMIN_USER` | `admin` |
| `-port` | `PORT` | `8080` |

## Architecture

**Single-binary Go web app** — standard library `net/http` with Go 1.22+ method+path routing patterns. Templates and static files are embedded into the binary via `//go:embed`.

### Packages

- **`main.go`** — wires everything: opens DB, loads templates, creates handlers, registers routes, runs graceful shutdown.
- **`internal/db/`** — all database access. Schema is applied idempotently on every `db.Open()` call (`CREATE TABLE IF NOT EXISTS`). No separate migration step needed for schema changes — add them to `schema.sql`. The `migrate/` directory is a one-off tool for importing legacy MySQL dumps.
- **`internal/handlers/`** — HTTP handlers split into `public.go` (unauthenticated) and `admin.go` (protected). `templates.go` manages a `map[string]*template.Template` keyed by page name; each page is parsed with its layout at startup.
- **`internal/middleware/`** — session-based admin auth using `gorilla/sessions`. `RequireAdmin` wraps the admin sub-router in `main.go`.
- **`internal/youtube/`** — YouTube Data API v3 client. `ParseURL` resolves playlist/channel/handle URLs to a `Source`. `RunImport` runs as a background goroutine, writing progress to the `import_jobs` table.

### Database

SQLite via `modernc.org/sqlite` (pure Go, no CGo). WAL mode, foreign keys on, max 1 open connection.

Videos support multiple categories via the `video_categories` junction table. The `videos.category_id` column is a legacy field; on `db.Open()` existing rows are migrated to `video_categories` automatically.

### Request flow — public side

`GET /` and `GET /c/{code}` → `PublicHandler.Index` renders the first random video. Subsequent videos load via `POST /next` (JSON response). `POST /report` disables the current video and returns the next one as JSON. `POST /vote` records a like/dislike per IP.

### Request flow — admin side

All `/admin/*` routes (except login/logout) go through `middleware.RequireAdmin`. Import jobs are submitted via `POST /admin/import`, which launches `youtube.RunImport` in a goroutine; status is polled by the browser at `GET /admin/import/job/{id}`.

### Testing

Tests use in-memory SQLite (`db.Open(":memory:")`). Handler tests use `net/http/httptest` and stub templates via `testing/fstest.MapFS`. All test files are in external `_test` packages.
