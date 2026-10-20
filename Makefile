GOLANGCI_LINT_VERSION ?= v2.14.0
# minimum total coverage (%), excluding generated mocks and main.go
COVERAGE_MIN ?= 80

start:
	@test -f .env || (echo "Missing .env: run 'cp .env.example .env' and set the values" && exit 1)
	docker-compose up -d --build

stop:
	docker-compose down

stopv:
	docker-compose down --volumes

migrate:
    liquibase --defaultsFile=liquibase.properties update

test:
	go test -race -cover ./...

generate:
	go generate ./...

# Fails if generated mocks are out of date with their interfaces
check-generate: generate
	git diff --exit-code -- mocks

# Writes coverage.out and fails if total coverage is below COVERAGE_MIN
coverage:
	go test -race -coverprofile=coverage.out ./...
	grep -v -e "/mocks/" -e "urlshortener/main.go" coverage.out > coverage.filtered.out
	@total=$$(go tool cover -func=coverage.filtered.out | awk '/^total:/ {sub("%", "", $$3); print $$3}'); \
	echo "total coverage: $$total% (minimum $(COVERAGE_MIN)%)"; \
	awk -v total=$$total -v min=$(COVERAGE_MIN) 'BEGIN { exit !(total >= min) }' || { echo "coverage is below $(COVERAGE_MIN)%"; exit 1; }

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

# Fails if go.mod/go.sum are not tidy
tidy-check:
	go mod tidy -diff

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

deps-update:
	go get -u ./...
	go mod tidy
