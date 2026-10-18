# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/main ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/main /app/main

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/main"]
