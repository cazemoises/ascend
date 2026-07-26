-include .env
export

.PHONY: dev down build test migrate

dev:
	docker compose up -d

down:
	docker compose down

build:
	docker compose build

migrate:
	go run ./cmd/migrate up

test:
	go test ./...
	cd web && npm test -- --run

lint:
	golangci-lint run
	cd web && npm run lint
