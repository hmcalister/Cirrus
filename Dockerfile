# syntax=docker/dockerfile:1

# Generate the OpenAPI spec from the Go source so the image build needs
# nothing generated on the host.
FROM docker.io/library/golang:1.27-alpine AS openapi

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY web/embed.go ./web/embed.go
# internal/api imports package web, whose //go:embed needs the dist directory
# to exist even though the spec generator never serves it.
RUN mkdir -p web/dist && : > web/dist/.gitkeep

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go run ./cmd/generate-openapi -o /openapi.yaml

FROM docker.io/oven/bun:1 AS web-build

WORKDIR /web

COPY web/package.json web/bun.lock ./
RUN --mount=type=cache,target=/root/.bun/install/cache \
    bun install --frozen-lockfile

COPY web ./
COPY --from=openapi /openapi.yaml /openapi.yaml
RUN bun run openapi
RUN bun run build

FROM docker.io/library/golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY web/embed.go ./web/embed.go
COPY --from=web-build /web/dist ./web/dist

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/main ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/main /app/main

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/main"]
