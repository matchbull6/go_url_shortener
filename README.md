# go_url_shortener

A URL shortener in Go: takes a long URL, returns a short code, and redirects by it.

## API

| Method | Path           | Response |
|--------|----------------|----------|
| POST   | `/api/shorten` | `{"url": "https://..."}` → `201` `{"code": "aB3xK9z", "short_url": "<BASE_URL>/aB3xK9z"}` |
| GET    | `/{code}`      | `302` to the original URL or `404` |

`POST /api/shorten` errors:
- `400` — invalid JSON or URL (the body explains why);
- `503` + `Retry-After: 1` — no free code found, retry the request;
- `500` — internal error (details only in the log).

## Configuration (environment variables)

| Variable      | Default                 | Description |
|---------------|-------------------------|-------------|
| `ADDR`        | `:8080`                 | `host:port` the server listens on |
| `BASE_URL`    | `http://localhost:8080` | public address used to build `short_url` |
| `CODE_LENGTH` | `7`                     | code length, 4 to 32 |
| `STORAGE`     | `memory`                | storage backend (only `memory` for now) |

An invalid `CODE_LENGTH` stops the service at startup.

## Running locally

```sh
go run .
# or
go build -o go_url_shortener.exe .
./go_url_shortener.exe
```

## Layout

```
main.go                         — env config, dependency wiring, server start
internal/httpapi/               — handlers and routing
internal/shortener/             — business logic
    service.go                  — Service: Shorten and Resolve
    code.go                     — code generation (crypto/rand)
    validate.go                 — URL validation
internal/storage/               — contract: Storage interface + ErrNotFound, ErrExists
internal/storage/storagetest/   — shared contract tests for all implementations
internal/storage/memory/        — in-memory implementation
```

## Docker

Two-stage build (`golang:*-trixie` → `distroless/static:nonroot`), limits 128 MiB / 0.5 CPU.

Set `BASE_URL` in `compose.yaml` to the address clients will use, then:

```sh
docker compose up -d --build
docker compose logs -f
docker compose down
```

There is no HTTPS or rate limiting yet. To keep the service off the internet,
publish the port on localhost only: `ports: "127.0.0.1:8080:8080"`.
For public use, put it behind a reverse proxy with HTTPS.

## Stages

- [x] **1. MVP:** in-memory storage, `POST /api/shorten`, `GET /{code}`
- [x] **2. Tests:** unit tests for the generator and storage, `httptest` for handlers, `go test -race`
- [ ] **3. Reliability:**
  - [x] URL validation, `MaxBytesReader`, `slog`, config via environment variables
  - [ ] server timeouts, graceful shutdown, JSON error format, `413` for oversized bodies
- [ ] **4. Persistence:** PostgreSQL + migrations, the storage interface stays the same
- [ ] **5. Analytics:** click counter, `GET /api/links/{code}/stats`
- [ ] **6. Ops:**
  - [x] Dockerfile, compose
  - [ ] rate limiting on link creation, reverse proxy + HTTPS
