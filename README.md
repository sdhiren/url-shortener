# URL Shortener

[![CI](https://github.com/sdhiren/url-shortener/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/sdhiren/url-shortener/actions/workflows/ci.yml)

A URL shortening service written in Go, using [Gin](https://gin-gonic.com/) and PostgreSQL.
You send it a long URL and it gives you a short one; you send the short code and it gives you the original URL back.

## Contents

- [Features](#features)
- [Architecture](#architecture)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [API reference](#api-reference)
- [Development](#development)
- [CI](#ci)
- [Contributing](#contributing)
- [Status and roadmap](#status-and-roadmap)

## Features

- Shorten any public `http`/`https` URL to an 8-character code.
- The same long URL always returns the same short code.
- Strict URL validation, with a clear reason when a URL is rejected (see [URL validation rules](#url-validation-rules)).
- JSON errors with proper HTTP status codes; internal errors are logged, never sent to clients.
- Typed, validated configuration from environment variables: the service lists every invalid setting at startup.
- Database schema managed with Liquibase migrations.
- Runs locally with one command through Docker Compose.

## Architecture

```mermaid
flowchart LR
    client([Client]) -->|HTTP| router[router<br/>Gin routes]
    router --> controller[controller<br/>request parsing,<br/>status codes]
    controller --> service[service<br/>validation,<br/>business rules]
    service --> util[util<br/>short code generation]
    service --> repository[repository<br/>SQL]
    repository --> db[(PostgreSQL)]
    liquibase[Liquibase] -. migrations .-> db
```

Each layer depends only on the interface of the layer below, so the tests use generated mocks.

| Path | Purpose |
|---|---|
| `main.go` | Loads config, connects to Postgres, starts the HTTP server |
| `router/` | Wires dependencies together and registers the routes |
| `src/controller/` | HTTP handlers: parse requests, map errors to status codes |
| `src/service/` | Business logic: URL validation, de-duplication, building short URLs |
| `src/repository/` | Postgres queries |
| `src/util/` | Short code generation (first 8 characters of a base64url SHA-256 hash) |
| `src/config/` | Loads and validates configuration from the environment |
| `src/domain/` | Domain errors shared across layers |
| `model/request/`, `model/response/` | JSON request and response bodies |
| `mocks/` | Generated mocks (`make generate`); don't edit by hand |
| `infrastructure/changelog/` | Liquibase changelog and SQL migrations |

## Quick start

**Prerequisites:** [Docker](https://docs.docker.com/get-docker/) with Docker Compose, and `make`.

```bash
cp .env.example .env
```

Edit `.env` and set at least `DB_PASSWORD`. Then start Postgres, run the migrations and start the API:

```bash
make start
```

Shorten a URL:

```bash
curl -s -X POST http://localhost:8080/shorten -H 'Content-Type: application/json' -d '{"url": "https://example.com/some/long/path"}'
```

```json
{"short_url":"http://localhost:8080/Xy12Ab34"}
```

Look up the original URL with the code from that response:

```bash
curl -s http://localhost:8080/url/Xy12Ab34
```

```json
{"long_url":"https://example.com/some/long/path"}
```

Stop everything with `make stop`, or `make stopv` to also delete the database volume.

## Configuration

All configuration comes from environment variables. With Docker Compose they are read from `.env`, which is gitignored; `.env.example` lists them all. Never commit real values.

| Variable | Required | Default | Allowed values |
|---|---|---|---|
| `PORT` | no | `8080` | 1–65535 |
| `BASE_URL` | no | `http://localhost:<PORT>/` | absolute `http`/`https` URL, no credentials, query or fragment; a trailing `/` is added if missing |
| `APP_MODE` | no | `release` | `debug`, `release`, `test` |
| `DB_USER` | yes | | |
| `DB_PASSWORD` | yes | | wrap in single quotes in `.env` if it contains `$` |
| `DB_HOST` | yes | | `db` inside Docker Compose, `localhost` when running the API on your machine |
| `DB_PORT` | no | `5432` | 1–65535 |
| `DB_NAME` | yes | | |
| `DB_SSLMODE` | no | `disable` | `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full` |

`BASE_URL` is the public address of the service. Short links are built by appending the code to it, so in production set it to your real domain, e.g. `https://sho.rt/`.

`APP_MODE` sets Gin's mode. It's used instead of Gin's own `GIN_MODE`, because Gin crashes on an invalid `GIN_MODE` before the service can validate it.

If any setting is invalid, the service prints every problem at once and exits:

```text
Invalid configuration:
invalid PORT "abc": must be a number between 1 and 65535
missing required environment variables: DB_USER, DB_PASSWORD, DB_HOST, DB_NAME
```

## API reference

All request and response bodies are JSON. Errors always have the shape `{"error": "<message>"}`.

### `POST /shorten`

Creates a short URL, or returns the existing one if this long URL was shortened before.

**Request**

```json
{"url": "https://example.com/some/long/path"}
```

**Responses**

| Status | Body | When |
|---|---|---|
| `200` | `{"short_url": "http://localhost:8080/Xy12Ab34"}` | Shortened, or already existed |
| `400` | `{"error": "url is required"}` | `url` is missing or empty |
| `400` | `{"error": "request body must be JSON like {\"url\": \"https://example.com\"}"}` | Body is not valid JSON, or `url` is not a string |
| `400` | `{"error": "invalid url: <reason>"}` | URL fails validation (see below) |
| `500` | `{"error": "internal server error"}` | Unexpected error, e.g. the database is down; details are in the server log |

### `GET /url/:code`

Returns the original URL for a short code.

```bash
curl -s http://localhost:8080/url/Xy12Ab34
```

| Status | Body | When |
|---|---|---|
| `200` | `{"long_url": "https://example.com/some/long/path"}` | Code exists |
| `404` | `{"error": "short url not found"}` | Unknown code |
| `500` | `{"error": "internal server error"}` | Unexpected error |

> This endpoint returns JSON for now. Backlog item B-02 changes it to a real `302` redirect.

### URL validation rules

Surrounding whitespace is trimmed first. A URL is rejected with `400` if it:

| Rule | Example of a rejected URL |
|---|---|
| is empty | `""` |
| is longer than 1000 characters | |
| contains spaces or control characters | `https://example.com/a b` |
| doesn't use `http` or `https` | `www.example.com`, `ftp://…`, `javascript:…` |
| has no host | `https://` |
| contains a username or password | `https://google.com@evil.example` |
| has a port outside 1–65535 | `https://example.com:0` |
| has an invalid host name | `https://-example.com`, `https://999.1.1.1` |
| isn't a public domain with a TLD | `http://localhost`, `https://intranet/` |
| points to a private, loopback or reserved IP | `http://127.0.0.1`, `http://192.168.1.1`, `http://169.254.169.254` |
| points back to this shortener's `BASE_URL` | `https://sho.rt/abc123` |

Unicode domains such as `https://münchen.de` are accepted.

## Development

**Prerequisites:** Go (the version in [`go.mod`](go.mod); Go downloads it automatically if yours is older), Docker, `make`.

### Run the API on your machine

Start only Postgres and the migrations in Docker:

```bash
docker compose up -d db liquibase
```

Then run the API with `DB_HOST=localhost` (the other values come from your `.env`):

```bash
set -a; source .env; set +a; DB_HOST=localhost go run .
```

### Make targets

| Command | What it does |
|---|---|
| `make start` / `make stop` / `make stopv` | Start / stop the Docker Compose stack (`stopv` also deletes the DB volume) |
| `make test` | Run all tests with the race detector and coverage |
| `make coverage` | Run tests, write `coverage.out`, and fail if total coverage is below `COVERAGE_MIN` (80%) |
| `make lint` | Run [golangci-lint](https://golangci-lint.run/) with `.golangci.yml` |
| `make generate` | Regenerate mocks in `mocks/` with `mockgen` |
| `make check-generate` | Fail if the committed mocks are out of date |
| `make tidy-check` | Fail if `go.mod` / `go.sum` aren't tidy |
| `make vuln` | Scan dependencies for known vulnerabilities with `govulncheck` |
| `make deps-update` | Update all dependencies and tidy |

### Mocks

Interfaces in `src/service`, `src/repository` and `src/util` have `//go:generate` directives. `mockgen` is pinned as a Go tool in `go.mod`, so `make generate` always uses the same version. Run it after changing an interface, and commit the result.

### Database migrations

Migrations live in `infrastructure/changelog/`. To add one, put a new SQL file in `sql/` and add a `changeSet` for it to `changelog.xml`. Docker Compose runs Liquibase on `make start`.

## CI

[GitHub Actions](.github/workflows/ci.yml) runs on every pull request and every push to `main`:

| Job | Checks |
|---|---|
| **Lint** | `go mod tidy`, mocks up to date, `go vet`, golangci-lint |
| **Test** | tests with the race detector, coverage of at least 80% (summary on the run page, report uploaded as an artifact) |
| **Security** | `govulncheck`, and a [gitleaks](https://github.com/gitleaks/gitleaks) secret scan with custom rules in `.gitleaks.toml` |
| **Docker image** | builds the image on PRs; on `main`, also pushes it to `ghcr.io/sdhiren/url-shortener` tagged with the commit SHA and `latest` |

Dependabot ([config](.github/dependabot.yml)) opens weekly PRs for Go modules, GitHub Actions and the Docker base image.

## Contributing

- `main` is protected: all changes go through a pull request, and PRs are squash-merged.
- Use one branch and one PR per backlog item.
- Commit messages and PR titles use the style `Dhiren: <short lowercase summary>`. With squash merges, the PR title becomes the commit on `main`.
- Before opening a PR, run `make lint test check-generate tidy-check`.
- Never commit secrets. Put them in `.env`, which is gitignored. CI scans every PR for leaks.
- If a change affects setup, configuration or the API, update this README in the same PR.

## Status and roadmap

This project is being turned into a production-ready service, one item at a time. [`BACKLOG.md`](BACKLOG.md) has the full prioritized list. Known limitations right now:

- `GET /url/:code` returns JSON instead of redirecting (B-02).
- Two different URLs could get the same 8-character code, and nothing detects it yet (B-05).
- The usage counter can lose updates under concurrent requests (B-06).
