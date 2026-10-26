.PHONY: init generate sqlc openapi web-install web-dev web-build build test lint fmt up down

SQLC ?= sqlc
NPM ?= npm
CONTAINER_ENGINE ?= podman

generate: sqlc openapi

sqlc:
	$(SQLC) generate -f sqlc/sqlc.yaml

openapi: web-install
	go run ./cmd/generate-openapi -o openapi.yaml
	$(NPM) --prefix web run openapi

init: web-install

web-install:
	$(NPM) install --prefix web

web-dev: openapi
	$(NPM) --prefix web run dev

web-build: openapi
	$(NPM) --prefix web run build

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
