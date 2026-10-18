start:
	@test -f .env || (echo "Missing .env: run 'cp .env.example .env' and set the values" && exit 1)
	docker-compose up -d --build

stop:
	docker-compose down

stopv:
	docker-compose down --volumes

migrate:
    liquibase --defaultsFile=liquibase.properties update