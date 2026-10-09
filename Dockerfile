# syntax=docker/dockerfile:1

# The build context is the repository root (see compose.yaml), so that this
# stage can see the Go source. Dependencies are downloaded before the source is
# copied in, so a source-only change does not invalidate the download layer.
FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal

# CGO_ENABLED=0 produces a fully static binary, which is what allows the final
# stage to be distroless/static. -trimpath strips local paths from the binary,
# and -s -w drop the symbol table and DWARF data.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/main ./cmd/server

# distroless/static carries CA certificates and a nonroot (uid 65532) user, but
# no shell and no package manager.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/main /app/main

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/main"]
