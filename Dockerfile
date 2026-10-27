# syntax=docker/dockerfile:1

# Generate the OpenAPI spec from the Go source so the image build needs nothing generated on the host.
FROM docker.io/library/golang:1.27-alpine AS openapi

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY web/embed.go ./web/embed.go
RUN mkdir -p web/dist && : > web/dist/.gitkeep

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go run ./cmd/generate-openapi -o /openapi.yaml

FROM docker.io/library/node:22-alpine AS web-build

WORKDIR /web

COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY web ./
COPY --from=openapi /openapi.yaml /openapi.yaml
RUN npm run openapi
RUN npm run build

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
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/server ./cmd/mailer

# The runtime copies this directory into the image, which gives a named volume mounted over it the same ownership.
RUN mkdir -p /out/logs

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/server /app/server
COPY --from=build /out/mailer /app/mailer
COPY --from=build --chown=nonroot:nonroot /out/logs /var/log/cirrus

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
