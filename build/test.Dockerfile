FROM golang:1.25-alpine

RUN apk add --no-cache \
    git \
    make \
    && rm -rf /var/cache/apk/*

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

CMD ["go", "test", "./...", "-v"]
