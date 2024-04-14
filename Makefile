login:
	docker login container-registry.oracle.com -u singh.dhirendra07@gmail.com -p hOCZDyOrtUCa_2K7=

start: login
	docker-compose up -d --build

stop:
	docker-compose down

stopv:
	docker-compose down --volumes

migrate:
    liquibase --defaultsFile=liquibase.properties update