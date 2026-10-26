.PHONY: init generate sqlc openapi web-install web-dev web-build build test lint fmt up down

SQLC ?= sqlc
BUN ?= bun
CONTAINER_ENGINE ?= podman

generate: sqlc openapi

sqlc:
	$(SQLC) generate -f sqlc/sqlc.yaml

openapi: web-install
	go run ./cmd/generate-openapi -o openapi.yaml
	$(BUN) run --cwd web openapi

init: web-install

web-install:
	$(BUN) install --cwd web

web-dev: openapi
	$(BUN) run --cwd web dev

web-build: openapi
	$(BUN) run --cwd web build

build: generate web-build
	go build ./...

test: generate
	go test -race ./...

lint: generate
	go vet ./...
	golangci-lint run ./...

fmt:
	gofmt -w .

up:
	$(CONTAINER_ENGINE) compose up --build

down:
	$(CONTAINER_ENGINE) compose down
