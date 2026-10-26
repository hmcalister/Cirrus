.PHONY: generate sqlc openapi build test lint fmt up down

SQLC ?= sqlc

# Regenerate everything derived from source.
generate: sqlc openapi

sqlc:
	$(SQLC) generate -f sqlc/sqlc.yaml

openapi:
	go run ./cmd/generate-openapi -o openapi.yaml

build: generate
	go build ./...

test: generate
	go test -race ./...

lint: generate
	go vet ./...
	golangci-lint run ./...

fmt:
	gofmt -w .

up: generate
	docker compose up --build

down:
	docker compose down
