# syntax=docker/dockerfile:1.7
#
# BuildKit is required (Docker >= 23 enables it by default). The
# `--mount=type=cache` directives below persist npm, Go module, Go build, and
# Vite caches between builds even when lockfiles change.

# ==== Stage 1: Build Frontend ====
FROM node:26-slim AS frontend-builder
WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --prefer-offline --no-audit --no-fund

COPY frontend/ .
RUN --mount=type=cache,target=/app/frontend/node_modules/.vite \
    npm run build

# ==== Stage 2: Build Go Server ====
FROM golang:1.26-alpine AS server-builder
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
RUN apk add --no-cache ca-certificates tzdata libcap && \
    addgroup -g 1000 supervisor && \
    adduser -D -u 1000 -G supervisor supervisor
WORKDIR /app

COPY --from=server-builder /serversupervisor .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist
RUN chown -R supervisor:supervisor /app && \
    setcap cap_net_raw+ep /app/serversupervisor

# cap_net_raw+ep above is what lets the ICMP uptime-probe check (internal/synthetic
# checkICMP) open a raw ping socket while running as the non-root `supervisor`
# user below — CAP_NET_RAW is in Docker's default capability set already, this
# just grants the *binary* permission to use it without being setuid-root. A
# hardened deployment that runs with `cap_drop: [ALL]` needs to explicitly
# `cap_add: [NET_RAW]` back for ICMP checks to work; every other feature is
# unaffected either way — a failed ICMP check just reports itself as down with
# an explicit "CAP_NET_RAW manquant" error instead of the container refusing to
# start.
USER supervisor
EXPOSE 8080

ENTRYPOINT ["./serversupervisor"]
