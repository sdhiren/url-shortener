# URL Shortener — Backlog

Goal: take this from a working prototype to a production-grade service.
We pick one item at a time, implement it, test it, and tick it off here.

Priority: **P0** = do now (security / broken behaviour) · **P1** = core quality · **P2** = production readiness · **P3** = features.
Size: **S** < 1h · **M** a few hours · **L** a day or more.

---

## P0 — Security & correctness bugs

- [x] **B-01 · Remove committed secrets (S)**: done; still to do: rotate the registry token and DB password, optionally purge history
  `Makefile` contains a plaintext Oracle container-registry password, and `.envrc`, `docker-compose.yaml`, and
  `liquibase.properties` contain the DB password. The repo is pushed to GitHub.
  - Rotate the registry token **first** (removing it from git doesn't revoke it; it's in history).
  - Move secrets to an untracked `.env` file, commit a `.env.example`, and add `.env` / `.envrc` to `.gitignore`.
  - Drop the `login` target (or read creds from env / `docker login` interactively). The image used is `postgres` from Docker Hub, so the Oracle login may not be needed at all.
  - Optional: purge history with `git filter-repo`.

- [ ] **B-02 · Redirect endpoint doesn't redirect (S)**
  `GET /url/:url` returns `200` with the long URL as JSON. It should return `302` (or `301`) with a `Location` header, and `404` when the code is unknown (right now it returns `200 ""`).
  Consider moving it to `GET /:code` so short links are actually short.

- [ ] **B-03 · Proper error handling & HTTP status codes (M)**
  - `Shorten` ignores bind errors and carries on with an empty URL.
  - Service methods return `string`, so DB errors come back to the client as `200` with the raw error text, which leaks internal details.
  - Change the service signatures to `(..., error)`, define domain errors (`ErrNotFound`, `ErrInvalidURL`, …), and map them to `400/404/500` in the controller.
  - Return structured JSON: `{"short_url": "...", "code": "..."}` / `{"error": "..."}`.

- [ ] **B-04 · Input validation (S)**
  Reject empty values, non-`http(s)` schemes (`javascript:`, `data:`…), malformed URLs, URLs over 1000 chars (the DB column limit), and links that point back to our own domain. Use `binding:"required,url"` plus `net/url` parsing.

- [ ] **B-05 · Hash collisions & duplicate rows (M)**
  - An 8-char SHA-256 prefix can collide for two different URLs. Nothing detects that, so a collision would redirect users to the wrong site.
  - There's no `UNIQUE` constraint on `SHORT_URL`/`LONG_URL`, and the check-then-insert pattern races under concurrent requests.
  - Fix: add a migration with unique indexes, use `INSERT … ON CONFLICT`, and on a collision retry with a salt. Alternative: switch to base62 encoding of a DB sequence/ID, which can't collide (see F-08).

- [ ] **B-06 · Counter logic is racy and counts the wrong thing (S)**
  - `updateCounter` does read → `+1` → write in a fire-and-forget goroutine, so concurrent updates get lost. Use `SET COUNTER_VALUE = COUNTER_VALUE + 1`.
  - It counts *shorten requests*. Click counts on redirect are usually what people want, so decide which (or track both).
  - The goroutine has no context or error handling and may still be running at shutdown.

- [ ] **B-07 · Fix the `migrate` Makefile target (S)**
  The recipe is indented with spaces, not a tab, so `make migrate` fails. It also points at the wrong properties path.

---

## P1 — Code quality & structure

- [ ] **Q-01 · Use contexts properly (S)**: use `QueryRowContext` / `ExecContext` and pass `c.Request.Context()` rather than `*gin.Context`. Drop the per-call `Prepare` (unneeded with `$1` params).
- [ ] **Q-02 · Remove dead code (S)**: the unused `Cache` map in the repository, the commented-out repository tests, `// var db` in `main.go`, and the unused `URL` struct tags.
- [ ] **Q-03 · Configuration management (S)**: base URL `http://localhost:8080/` and port `:8080` are hardcoded. Load a typed `Config` struct from env (`BASE_URL`, `PORT`, `DB_*`, `GIN_MODE`) with validation and defaults.
- [ ] **Q-04 · Idiomatic project layout (M)**: `src/` isn't idiomatic Go. Move to `cmd/server/main.go` + `internal/{handler,service,repository,config,model}`. Split request/response DTOs from the domain model.
- [ ] **Q-05 · Structured logging (S)**: replace `fmt.Println` with `log/slog` (JSON in prod), add a request-ID middleware, and log errors once at the boundary.
- [ ] **Q-06 · Naming & formatting (S)**: run `gofmt`/`goimports`, fix the `SaveShotenedURL` typo, use camelCase instead of `snake_case` vars, and rename the `context` param that shadows the package.
- [ ] **Q-07 · Dependency hygiene (S)**: commit or revert the pending `go.mod`/`go.sum` changes. Either commit `vendor/` or stop using it (it's gitignored but present). Upgrade Go, Gin, and `x/net`, then run `govulncheck`. Migrate from the archived `github.com/golang/mock` to `go.uber.org/mock`.
- [ ] **Q-08 · Linting (S)**: add `golangci-lint` with a `.golangci.yml` (errcheck, govet, staticcheck, revive, gosec).

---

## P1 — Testing

- [ ] **T-01 · Repository integration tests (M)**: the repository has 0% coverage because all tests are commented out. Use `testcontainers-go` (real Postgres) or `sqlmock`.
- [ ] **T-02 · Expand service/controller tests (S)**: cover error paths, validation, and status codes after B-02/B-03. Service is at 73.7%.
- [ ] **T-03 · End-to-end API test (M)**: spin up the router plus a real DB, then shorten, redirect, and assert the counter.
- [ ] **T-04 · Race detector & coverage in CI (S)**: run `go test -race -cover ./...`.

---

## P2 — Production readiness

- [ ] **P-01 · Graceful shutdown & server timeouts (S)**: use `http.Server` with Read/Write/Idle timeouts and `signal.NotifyContext` + `Shutdown`. Configure the DB pool (`SetMaxOpenConns`, `SetConnMaxLifetime`).
- [ ] **P-02 · Health endpoints (S)**: `/healthz` (liveness) and `/readyz` (pings the DB). Wire them into the compose healthcheck.
- [ ] **P-03 · Production Dockerfile (S)**: multi-stage build, a static binary with `CGO_ENABLED=0`, a `distroless`/`scratch` runtime, a non-root user, a `.dockerignore`, and pinned base image versions.
- [ ] **P-04 · Compose fixes (S)**: the API should wait for Liquibase (`condition: service_completed_successfully`). Pin `postgres` and `liquibase` versions, drop the obsolete `version:` key, and don't publish the DB port by default.
- [ ] **P-05 · Migrations (S)**: add indexes on `SHORT_URL`/`LONG_URL`, `CREATED_AT`, and `EXPIRES_AT` columns, and fix the `author="your_name"` changeset. Optionally replace Liquibase (a JVM container) with `golang-migrate` or `goose` run at startup.
- [ ] **P-06 · CI pipeline (S)**: GitHub Actions to lint, test with `-race`, run `govulncheck`, and build the Docker image on each PR.
- [ ] **P-07 · Rate limiting (M)**: per-IP limits on `POST /shorten` (in-memory token bucket first, Redis later).
- [ ] **P-08 · Observability (M)**: Prometheus `/metrics` (request latency, status codes, shortens and redirects), optional OpenTelemetry tracing.
- [ ] **P-09 · Security headers & CORS (S)**: CORS config for a frontend, `gin.SetTrustedProxies`, and a request body size limit.
- [ ] **P-10 · README & API docs (S)**: setup, env vars, architecture, and an OpenAPI/Swagger spec (`swaggo`).

---

## P3 — Features

- [ ] **F-01 · Custom aliases (M)**: `POST /shorten {"url": "...", "alias": "my-link"}` with validation of the allowed charset and reserved words.
- [ ] **F-02 · Link expiry (M)**: an optional `expires_at` / TTL. Expired links return `410 Gone`, with a cleanup job.
- [ ] **F-03 · Click analytics (L)**: a `clicks` table (timestamp, referrer, user agent, country) and `GET /api/links/:code/stats`.
- [ ] **F-04 · Link management API (M)**: `GET /api/links`, `GET /api/links/:code`, `DELETE /api/links/:code`, with pagination.
- [ ] **F-05 · Users & auth (L)**: API keys or JWT, so links belong to users and only owners can delete them or see stats.
- [ ] **F-06 · Redis cache for redirects (M)**: a read-through cache on the hot path, with invalidation on delete/expiry.
- [ ] **F-07 · QR codes (S)**: `GET /api/links/:code/qr` returns a PNG.
- [ ] **F-08 · Base62 ID-based codes (M)**: generate codes from a sequence (or Snowflake ID) instead of a hash, so collisions can't happen. This is an alternative fix for B-05.
- [ ] **F-09 · Web UI (L)**: a simple frontend (HTMX or React) to shorten links and view stats.
- [ ] **F-10 · Deployment (M)**: deploy to Fly.io, Render, or Kubernetes (Helm chart) with managed Postgres.

---

## Suggested order

1. **B-01** (secrets: rotate today)
2. **Q-07**, then **Q-03** (clean deps + config; this makes everything else easier)
3. **B-03 → B-04 → B-02** (error model, validation, real redirects)
4. **B-05 + P-05**, then **B-06** (data integrity)
5. **Q-01, Q-02, Q-05, Q-06** (cleanup pass), then **Q-04** (layout refactor)
6. **T-01 → T-04**, **Q-08**, **P-06** (safety net)
7. **P-01 → P-04, P-10** (ship-ready)
8. Features: **F-04 → F-01 → F-02 → F-03 → F-06 → F-05 …**
