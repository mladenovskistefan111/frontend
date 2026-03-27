# ---- Build stage ----
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0
ENV GOOS=linux

RUN apk add --no-cache git

WORKDIR /src

# Download dependencies first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN go build -ldflags="-s -w" -o /bin/frontend ./cmd/server

# ---- Runtime stage ----
FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates \
    && addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /bin/frontend .
COPY --chown=appuser:appgroup templates/ ./templates/
COPY --chown=appuser:appgroup static/ ./static/

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://localhost:${PORT:-8080}/_healthz || exit 1

ENTRYPOINT ["./frontend"]