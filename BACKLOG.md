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

- [x] **B-02 · Redirect endpoint doesn't redirect (S)**: done. `GET /:code` (under `BASE_URL`'s path) returns `302` with `Location` and `Cache-Control: no-store`; `HEAD` gets the same redirect without counting a click. Malformed codes return 404 without querying the DB. The old JSON `GET /url/:code` is removed; a JSON lookup returns in F-04.

- [x] **B-03 · Proper error handling & HTTP status codes (M)**: done. Service returns `(string, error)` with wrapped errors, and the repository maps `sql.ErrNoRows` to `domain.ErrURLNotFound`. The controller returns 400 (missing `url` / invalid JSON), 404 (unknown code) and 500 (generic message; details only in the server log). JSON responses: `{"short_url"}`, `{"long_url"}`, `{"error"}`.

- [x] **B-04 · Input validation (S)**: done. The service validates before hashing or saving and returns `domain.InvalidURLError` (400 with a reason). Rules: trimmed, required, ≤ 1000 characters, no whitespace or control characters, `http`/`https` only, a host is required, no credentials, valid port, valid (IDN) domain with a TLD, no private, loopback or reserved IPs, and no links back to the shortener's own `BASE_URL`.

- [x] **B-05 · Hash collisions & duplicate rows (M)**: done. A unique constraint on `SHORT_URL` plus `INSERT … ON CONFLICT (SHORT_URL) DO NOTHING`. When a code is taken, the service reads its owner: for the same URL it returns the existing link; for a different URL (a collision) it retries with a deterministic salted hash, up to 5 attempts. The racy check-then-insert (`IfURLAlreadyExists`) is gone. Verified on Postgres: 50 concurrent shortens gave 1 row, and a planted row holding a victim URL's code didn't hijack it.

- [x] **B-06 · Counter logic is racy and counts the wrong thing (S)**: done. Migration 002 renames `COUNTER_VALUE` to `CLICK_COUNT` (default 0, old shorten-request counts reset). Clicks are counted with one atomic `UPDATE … SET CLICK_COUNT = CLICK_COUNT + 1 … RETURNING LONG_URL`; the background goroutine is gone, and shortening no longer touches the count. Verified: 200 concurrent clicks gave exactly 200.

- [x] **B-07 · Fix the `migrate` Makefile target (S)**: done in P-06. The recipe was indented with spaces, which GNU make 4 on CI rejects for the whole Makefile (macOS make 3.81 didn't). `make migrate` now runs `docker compose run --rm liquibase`.

---

## P1 — Code quality & structure

- [ ] **Q-01 · Use contexts properly (S)**: use `QueryRowContext` / `ExecContext` and pass `c.Request.Context()` rather than `*gin.Context`. Drop the per-call `Prepare` (unneeded with `$1` params). *(The repository already uses the `*Context` methods since B-06.)*
- [ ] **Q-02 · Remove dead code (S)**: the unused `Cache` map in the repository, the commented-out repository tests, `// var db` in `main.go`, and the unused `URL` struct tags. *(The `Cache` map and `URL` struct were removed in B-06.)*
- [x] **Q-03 · Configuration management (S)**: done. `config.Load()` returns a typed `Config` (`PORT`, `BASE_URL`, `APP_MODE`, `DB_*`) with defaults and validation, and reports every invalid setting at once. `APP_MODE` is used instead of `GIN_MODE` because gin panics on a bad `GIN_MODE` before validation runs.
- [ ] **Q-04 · Idiomatic project layout (M)**: `src/` isn't idiomatic Go. Move to `cmd/server/main.go` + `internal/{handler,service,repository,config,model}`. Split request/response DTOs from the domain model.
- [ ] **Q-05 · Structured logging (S)**: replace `fmt.Println` with `log/slog` (JSON in prod), add a request-ID middleware, and log errors once at the boundary.
- [ ] **Q-06 · Naming & formatting (S)**: run `gofmt`/`goimports`, fix the `SaveShotenedURL` typo, use camelCase instead of `snake_case` vars, and rename the `context` param that shadows the package. Then turn on `revive` in `.golangci.yml` (left off in P-06 because of about 20 naming warnings).
- [x] **Q-07 · Dependency hygiene (S)**: done. Go 1.27.2, Gin 1.12, x/net 0.61, govulncheck clean, stale `vendor/` removed, mocks moved to `go.uber.org/mock` (mockgen pinned as a `go tool`), and `make test/generate/check-generate/vuln` added.
- [x] **Q-08 · Linting (S)**: done. `.golangci.yml` (golangci-lint v2) with the standard linters plus `gosec`, `errorlint`, `bodyclose`, `sqlclosecheck`, `rowserrcheck`, `misspell`, `nilerr` and `unconvert`; `gofmt`/`goimports` formatters; `make lint`. `revive` is deferred to Q-06.

---

## P1 — Documentation

- [x] **D-01 · README (S)**: done. `README.md` covers features, architecture (Mermaid diagram and layout table), quick start, a configuration table, the API reference with all status codes and URL validation rules, development and make targets, CI, contributing, and known limitations. Keep it updated in the same PR as any change to setup, config or the API.

## P1 — Testing

- [ ] **T-01 · Repository integration tests (M)**: every repository method is covered with exact-SQL `go-sqlmock` tests (100%). Still to do: run them against a real Postgres (`testcontainers-go`) so the SQL itself is checked.
- [x] **T-02 · Expand service/controller tests (S)**: done. Every package is at 100%, including redirect/HEAD tests and router tests that send real HTTP requests through every layer to sqlmock.
- [ ] **T-03 · End-to-end API test (M)**: spin up the router plus a real DB, then shorten, redirect, and assert the counter.
- [x] **T-04 · Race detector & coverage in CI (S)**: done in P-06: `make coverage` runs `go test -race` and fails below 80% total (excluding mocks and `main.go`).

---

## P2 — Production readiness

- [ ] **P-01 · Graceful shutdown & server timeouts (S)**: use `http.Server` with Read/Write/Idle timeouts and `signal.NotifyContext` + `Shutdown`. Configure the DB pool (`SetMaxOpenConns`, `SetConnMaxLifetime`).
- [ ] **P-02 · Health endpoints (S)**: `/healthz` (liveness) and `/readyz` (pings the DB). Wire them into the compose healthcheck.
- [ ] **P-03 · Production Dockerfile (S)**: multi-stage build, a static binary with `CGO_ENABLED=0`, a `distroless`/`scratch` runtime, a non-root user, a `.dockerignore`, and pinned base image versions.
- [ ] **P-04 · Compose fixes (S)**: the API should wait for Liquibase (`condition: service_completed_successfully`). Pin `postgres` and `liquibase` versions, drop the obsolete `version:` key, and don't publish the DB port by default. *(Partly done in P-06: the API now waits for Liquibase to finish, and Liquibase gets its credentials as arguments, since Liquibase Community 4.9 ignores `LIQUIBASE_COMMAND_*` env vars, so B-01 had silently broken migrations.)*
- [x] **P-05 · Migrations (S)**: done in migration 003: duplicates merged, unique constraint on `SHORT_URL`, and a `CREATED_AT` column. Decided against the rest: no `LONG_URL` index (nothing queries by long URL any more, and a btree index fails on long multi-byte URLs); `EXPIRES_AT` belongs to F-02; and changeset 001's `author` can't be changed, because Liquibase would treat it as a new changeset. Replacing Liquibase is still optional.
- [x] **P-06 · CI pipeline with GitHub Actions (M)**: done. `.github/workflows/ci.yml` runs on PRs and on pushes to `main`, with Go from `go.mod`, caching, concurrency cancelling and read-only permissions:
  - **Lint:** `make tidy-check`, `make check-generate`, `go vet`, golangci-lint v2.14.0.
  - **Test:** `make coverage` (race detector, 80% gate), coverage table in the run summary, report uploaded as an artifact.
  - **Security:** `govulncheck` and gitleaks. `.gitleaks.toml` adds rules for plain passwords in config files and `docker login -p`, which the default rules missed. The pre-B-01 leaks are listed in `.gitleaksignore`.
  - **Docker image:** built on every run; on `main`, pushed to `ghcr.io/sdhiren/url-shortener` (`sha-<commit>` and `latest`).
  - **Dependabot** for Go modules, GitHub Actions and the Docker base image, and a CI badge in the README.
  - Still to do: an integration job with a Postgres service container once T-01/T-03 exist, and adding the CI jobs as required status checks in the `protect_main` ruleset.
- [ ] **P-07 · Rate limiting (M)**: per-IP limits on `POST /shorten` (in-memory token bucket first, Redis later).
- [ ] **P-08 · Observability (M)**: Prometheus `/metrics` (request latency, status codes, shortens and redirects), optional OpenTelemetry tracing.
- [ ] **P-09 · Security headers & CORS (S)**: CORS config for a frontend, `gin.SetTrustedProxies`, and a request body size limit.
- [ ] **P-10 · API docs (S)**: an OpenAPI/Swagger spec (`swaggo`) served at `/swagger`, linked from the README (D-01).

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
3. ~~**B-03 → B-04 → B-02**~~ (done: error model, validation, real redirects)
4. ~~**B-05 + P-05**, **B-06**~~ (done: data integrity)
5. **Q-01, Q-02, Q-05, Q-06** (cleanup pass), then **Q-04** (layout refactor)
6. ~~**D-01**, **Q-08 → P-06**~~ (done), then **T-01 → T-03**
7. **P-01 → P-04, P-10** (ship-ready)
8. Features: **F-04 → F-01 → F-02 → F-03 → F-06 → F-05 …**
