TEST_DATABASE_URL ?= postgres://app:app@localhost:5436/app_test?sslmode=disable

.PHONY: dev run build test test-e2e lint migrate seed gen

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null)

dev: ## hot reload (go install github.com/air-verse/air@latest)
	air --build.cmd "go build -o bin/api ./cmd/api" --build.bin "./bin/api" --build.exclude_dir "bin,tmp"

run:
	go run ./cmd/api

build:
	go build -ldflags "-s -w -X govite/internal/system/interface/http.Version=$(VERSION)" -o bin/api ./cmd/api

test: ## unit tests; e2e tests skip without TEST_DATABASE_URL
	go test ./...

test-e2e: ## needs: docker compose up -d postgres_test (from repo root)
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race -p 1 ./...

lint:
	gofmt -l . | (! grep .) && go vet ./...

migrate: ## make migrate CMD="down" for other goose commands
	go run ./cmd/api migrate $(CMD)

seed:
	go run ./cmd/api seed

gen: ## after editing internal/platform/postgres/queries or internal/policy
	sqlc generate
	go run ./cmd/gen
