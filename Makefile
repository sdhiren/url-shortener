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

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

deps-update:
	go get -u ./...
	go mod tidy
