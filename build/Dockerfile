FROM alpine:3.20

RUN apk add --no-cache \
    ca-certificates \
    libc6-compat \
    && rm -rf /var/cache/apk/*

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

RUN mkdir -p /app/logs && \
    chown -R appuser:appuser /app

WORKDIR /app
USER appuser
CMD ["/app/main"]