# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build

WORKDIR /src

# Create an empty template directory owned by container.
RUN mkdir /templates

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

# Templates are not baked into the image.
# This allows for hot reloading by updating the templates on host.
# Whether or not this actually does anything depends on the implementation of the app.
COPY --from=build --chown=nonroot:nonroot /templates /templates

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/main"]
