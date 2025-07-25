.PHONY: dev-up migrate seed test bench

dev-up:
	docker compose up -d db

migrate:
	go run ./cmd/migrate up

seed:
	go run ./cmd/demo seed

test:
	go test ./...

bench:
	go test -bench=. ./... 