up:
	docker-compose up -d

down:
	docker-compose down

logs:
	docker-compose logs -f

clean:
	docker-compose down -v --remove-orphans

build:
	docker build . --platform=linux/amd64

export GOOSE_MIGRATION_DIR=migrations
export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING=postgresql://postgres:password@127.0.0.1/oliverbutler?sslmode=disable

migration:
	goose -s create $(NAME) sql

migrate:
	goose up

.PHONY: migration
