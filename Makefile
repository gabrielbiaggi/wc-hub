.PHONY: dev up down logs test lint build migrate

dev:
	docker compose up --build

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=150

test:
	cd back && go test ./...
	cd front && npm run typecheck

build:
	cd back && go build ./cmd/api
	cd front && npm run build

migrate:
	docker compose run --rm migrate

