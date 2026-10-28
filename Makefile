.PHONY: init generate sqlc openapi web-install web-dev web-build build test lint fmt image up down

SQLC ?= sqlc
NPM ?= npm
CONTAINER_ENGINE ?= podman
PLATFORM ?= linux/amd64

# The image name is owned by .env so compose and make cannot disagree. .env cannot
# be include'd (its passwords contain # and $), so these two keys are read directly.
# Keep the values unquoted.
PROJECT_NAME ?= $(shell sed -n 's/^PROJECT_NAME=//p' .env 2>/dev/null)
PROJECT_TAG ?= $(shell sed -n 's/^PROJECT_TAG=//p' .env 2>/dev/null)
IMAGE ?= localhost/$(PROJECT_NAME):$(PROJECT_TAG)

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

image:
	$(CONTAINER_ENGINE) build --platform $(PLATFORM) -t $(IMAGE) .

up:
	$(CONTAINER_ENGINE) compose up

down:
	$(CONTAINER_ENGINE) compose down