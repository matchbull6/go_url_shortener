# go_url_shortener

A URL shortener in Go: takes a long URL, returns a short code, and redirects by it.

## API

| Method | Path           | Response |
|--------|----------------|----------|
| POST   | `/api/shorten` | `{"url": "https://..."}` → `201` `{"code": "aB3xK9z", "short_url": "<BASE_URL>/aB3xK9z"}` |
| GET    | `/{code}`      | `302` to the original URL or `404`; counts a click |
| GET    | `/api/links/{code}/stats` | `200` with link stats (below) or `404`; does not count a click |

```json
{
  "code": "aB3xK9z",
  "url": "https://go.dev",
  "short_url": "<BASE_URL>/aB3xK9z",
  "clicks": 42,
  "created_at": "2026-01-02T00:04:05Z"
}
```

Errors are JSON with a stable machine-readable `code` and a human-readable `message`:

```json
{"error": {"code": "invalid_url", "message": "shortener: invalid URL: missing host"}}
```

| Status | `code`           | When |
|--------|------------------|------|
| `400`  | `invalid_json`   | the body is not valid JSON |
| `400`  | `invalid_url`    | the URL is rejected (`message` explains why) |
| `404`  | `not_found`      | unknown short code |
| `413`  | `body_too_large` | request body is larger than 4 KiB |
| `503`  | `no_free_code`   | no free code found, retry the request (`Retry-After: 1`) |
| `500`  | `internal`       | internal error (details only in the log) |

Match on `code`; `message` may change.

## Configuration (environment variables)

| Variable       | Default                 | Description |
|----------------|-------------------------|-------------|
| `ADDR`         | `:8080`                 | `host:port` the server listens on |
| `BASE_URL`     | `http://localhost:8080` | public address used to build `short_url` |
| `CODE_LENGTH`  | `7`                     | code length, 4 to 32 |
| `STORAGE`      | `memory`                | `memory` (lost on restart) or `postgres` |
| `DATABASE_URL` | —                       | PostgreSQL connection string, required when `STORAGE=postgres` |

Invalid settings stop the service at startup. With `STORAGE=postgres`, migrations run on startup.

## Running locally

```sh
go run .
# or
go build -o go_url_shortener.exe .
./go_url_shortener.exe
```

With PostgreSQL:


```powershell
# sh
STORAGE=postgres DATABASE_URL="postgres://user:pass@localhost:5432/shortener?sslmode=disable" go run .

# PowerShell
$env:STORAGE = "postgres"
$env:DATABASE_URL = "postgres://user:pass@localhost:5432/shortener?sslmode=disable"
go run .
```

## Tests

```sh
go test ./...
go test -race ./...   # data race detector, needs cgo (gcc)
```

PostgreSQL tests run only when `TEST_DATABASE_URL` is set, otherwise they are skipped.
Use a separate database: the tests truncate its tables.

```powershell
# sh
TEST_DATABASE_URL="postgres://user:pass@localhost:5432/shortener_test?sslmode=disable" go test ./internal/storage/postgres/

# PowerShell
$env:TEST_DATABASE_URL = "postgres://user:pass@localhost:5432/shortener_test?sslmode=disable"
go test ./internal/storage/postgres/
```

## Layout

```
main.go                         — env config, dependency wiring, server start
internal/httpapi/               — handlers and routing
internal/shortener/             — business logic
    service.go                  — Service: Shorten, Resolve, Stats
    code.go                     — code generation (crypto/rand)
    validate.go                 — URL validation
internal/storage/               — contract: Storage interface, Link, ErrNotFound, ErrExists
internal/storage/storagetest/   — shared contract tests for all implementations
internal/storage/memory/        — in-memory implementation
internal/storage/postgres/      — PostgreSQL implementation (pgx)
    migrations/                 — SQL migrations (goose), embedded into the binary
docker/postgres/                — init script: creates the app's database role
```

## Docker

Two-stage build (`golang:*-trixie` → `distroless/static:nonroot`).

Set `BASE_URL` in `compose.yaml` to the address clients will use, then:

```sh
cp .env.example .env    # empty values: in-memory storage
docker compose up -d --build
docker compose logs -f
docker compose down
```

To use PostgreSQL, fill in `.env`:

```sh
COMPOSE_PROFILES=postgres
STORAGE=postgres
POSTGRES_PASSWORD=...
APP_DB_PASSWORD=...
```

The `postgres` service starts only with this profile. On first start it creates
the `shortener` role, a non-superuser that owns the database; the app connects as it.
Data lives in the `postgres-data` volume. The init script runs only on an empty volume,
so changing `APP_DB_PASSWORD` later requires `ALTER ROLE` or a fresh volume.

There is no HTTPS or rate limiting yet. The port is published on localhost only
(`127.0.0.1:8080:8080`). For public use, put it behind a reverse proxy with HTTPS.

## Stages

- [x] **1. MVP:** in-memory storage, `POST /api/shorten`, `GET /{code}`
- [x] **2. Tests:** unit tests for the generator and storage, `httptest` for handlers, `go test -race`
- [x] **3. Reliability:** URL validation, `MaxBytesReader`, `slog`, config via environment variables,
  graceful shutdown, server timeouts, JSON errors, `413` for oversized bodies
- [x] **4. Persistence:** PostgreSQL + migrations, the storage interface stays the same
- [x] **5. Analytics:** click counter, `GET /api/links/{code}/stats`
- [ ] **6. Ops:**
  - [x] Dockerfile, compose
  - [ ] rate limiting on link creation, reverse proxy + HTTPS
