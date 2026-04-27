# RandomTube

A web app that serves random YouTube videos. Users can browse videos, vote (like/dislike), and report inappropriate content. Admins manage videos and import them from YouTube playlists or channels via a built-in interface.

## Stack

- **Go** — single binary, standard library `net/http`
- **SQLite** — database (`randomtube.db`)
- Templates and static files are embedded into the binary via `go:embed`

## Development

```bash
make run-dev        # build and run with dev defaults (ADMIN_PASSWORD=dev)
make test           # run tests
make build          # build binary only
```

## Configuration

Via flags or environment variables (env takes precedence):

| Flag | Env | Default |
|------|-----|---------|
| `-admin-password` | `ADMIN_PASSWORD` | *(required)* |
| `-db` | `DB_PATH` | `randomtube.db` |
| `-yt-api-key` | `YOUTUBE_API_KEY` | *(import disabled if unset)* |
| `-session-secret` | `SESSION_SECRET` | `change-me-in-production` |
| `-admin-user` | `ADMIN_USER` | `admin` |
| `-port` | `PORT` | `8080` |

## Production Deployment

### Directory layout

The service runs in its own directory and joins a shared Docker network where Caddy is already running.

```
~/
├── docker-compose-caddy/       # Caddy (separate project, already running)
│   ├── Caddyfile
│   └── docker-compose.yml
└── docker-compose-randomtube/  # RandomTube
    ├── docker-compose.yml
    ├── docker-compose.override.yml  # local overrides (not tracked by git)
    └── randomtube.db
```

### 1. Create the shared Docker network (once)

```bash
docker network create common
```

### 2. Configure Caddy

Add a site block to `Caddyfile`:

```caddyfile
https://randomtube.example.com:443 {
    encode gzip

    handle {
        reverse_proxy randomtube:8080 {
            header_up Host {host}
            header_up X-Real-IP {http.request.remote_host}
            header_up X-Forwarded-For {http.request.remote_host}
            header_up X-Forwarded-Proto {scheme}
            header_up X-Forwarded-Ssl on
        }
    }

    header {
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        X-XSS-Protection "1; mode=block"
        Referrer-Policy strict-origin-when-cross-origin
        Permissions-Policy "camera=(), microphone=(), geolocation=()"
    }
}
```

Reload Caddy:

```bash
cd ~/docker-compose-caddy
docker compose exec caddy caddy reload --config /etc/caddy/Caddyfile
```

### 3. Start RandomTube

```bash
mkdir ~/docker-compose-randomtube
cd ~/docker-compose-randomtube
touch randomtube.db
```

Copy the `cron` directory from this repo — it contains the Dockerfile and scripts needed to build the backup container:

```bash
cp -r /path/to/randomtube/cron ~/docker-compose-randomtube/cron
```

Or clone just that directory using sparse checkout:

```bash
git clone --filter=blob:none --sparse https://github.com/rhamdeew/randomtube.git _tmp
git -C _tmp sparse-checkout set cron
cp -r _tmp/cron ~/docker-compose-randomtube/cron
rm -rf _tmp
```

Create `docker-compose.yml`:

```yaml
services:
  app:
    image: ghcr.io/rhamdeew/randomtube:latest
    restart: unless-stopped
    networks:
      common:
        aliases:
          - randomtube
    volumes:
      - ./randomtube.db:/data/randomtube.db
    environment:
      DB_PATH: /data/randomtube.db
      ADMIN_PASSWORD: your-strong-password
      ADMIN_USER: admin
      SESSION_SECRET: your-long-random-secret
      YOUTUBE_API_KEY: ""   # optional
      PORT: "8080"

  cron:
    build:
      context: ./cron
      dockerfile: Dockerfile
    container_name: randomtube_cron
    volumes:
      - ./randomtube.db:/app/randomtube.db:ro
      - backup_data:/backups
    environment:
      S3_BUCKET: ""       # optional
      S3_ACCESS_KEY: ""
      S3_SECRET_KEY: ""
      S3_HOST: ""
      S3_REGION: ""
    restart: unless-stopped

networks:
  common:
    external: true

volumes:
  backup_data:
```

Start:

```bash
docker compose up -d
```

### Backups

The `cron` container backs up `randomtube.db` daily at 07:00 UTC and stores archives in the `backup_data` Docker volume. Rotation policy: 1 monthly, 2 weekly, 3 daily.

To enable S3 sync, set `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_HOST`, and `S3_REGION` via `docker-compose.override.yml` (not tracked by git):

```yaml
# docker-compose.override.yml
services:
  cron:
    environment:
      S3_BUCKET: "my-bucket"
      S3_ACCESS_KEY: "AKIAXXXXXXXX"
      S3_SECRET_KEY: "secret"
      S3_HOST: "s3.amazonaws.com"
      S3_REGION: "us-east-1"
```

View backup logs:

```bash
docker compose logs cron
```

Run a backup manually:

```bash
docker compose exec cron /usr/local/bin/backup.sh
```
