# syntax=docker/dockerfile:1.7
#
# BuildKit is required (Docker >= 23 enables it by default). The
# `--mount=type=cache` directives below persist npm, Go module, Go build, and
# Vite caches between builds even when lockfiles change.

# ==== Stage 1: Build Frontend ====
FROM node:22-slim AS frontend-builder
WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --prefer-offline --no-audit --no-fund

COPY frontend/ .
RUN --mount=type=cache,target=/app/frontend/node_modules/.vite \
    npm run build

# ==== Stage 2: Build Go Server ====
FROM golang:1.25-alpine AS server-builder
RUN apk add --no-cache git
WORKDIR /app

COPY server/go.mod server/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY server/ .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /serversupervisor ./cmd/server

# ==== Stage 3: Final Image ====
FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=server-builder /serversupervisor .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

EXPOSE 8080

ENTRYPOINT ["./serversupervisor"]
