include .env  
export

BINARY_DIR  := ./bin
BINARY      := $(BINARY_DIR)/$(APP_NAME)
MIGRATIONS  := ./migrations
DB_URL      ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)

.PHONY: build run \
				test test-verbose lint tidy vet \
				migrate-up migrate-down migrate-reset migrate-status migrate-create \
				docker-up docker-down docker-build

# Build
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BINARY_DIR)
	go build -ldflags="-s -w" -o $(BINARY) ./cmd/api

run:
	go run ./cmd/api

# Quality
test:
	go test ./... -race -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

test-verbose:
	go test ./... -race -v

lint:
	golangci-lint run ./...

tidy:
	go mod tidy
	go mod verify

vet:
	go vet ./...

# Database
migrate-up:
	@goose -dir $(MIGRATIONS) postgres "$(DB_URL)" up

migrate-down:
	@goose -dir $(MIGRATIONS) postgres "$(DB_URL)" down

migrate-reset:
	@goose -dir $(MIGRATIONS) postgres "$(DB_URL)" reset

migrate-status:
	@goose -dir $(MIGRATIONS) postgres "$(DB_URL)" status

migrate-create:
	@goose -dir $(MIGRATIONS) create $(name) sql

# Docker
docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-build:
	docker compose build
