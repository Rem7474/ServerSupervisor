# syntax=docker/dockerfile:1.7
#
# BuildKit is required (Docker >= 23 enables it by default). The
# `--mount=type=cache` directives below persist npm, Go module, Go build, and
# Vite caches between builds even when lockfiles change.

# ==== Stage 1: Build Frontend ====
# --platform=$BUILDPLATFORM pins this stage to the builder's own arch (not the
# image's target arch) for every multi-arch build: the compiled bundle is pure
# JS/CSS/HTML, so building it under QEMU emulation for a second target arch
# would be pure waste — same output, ~5-10x slower. BuildKit also shares this
# stage's cached layer across every target platform in the same build.
FROM --platform=$BUILDPLATFORM node:26-alpine AS frontend-builder
WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --prefer-offline --no-audit --no-fund

COPY frontend/ .
RUN --mount=type=cache,target=/app/frontend/node_modules/.vite \
    npm run build

# ==== Stage 2: Build Go Server ====
# Same --platform=$BUILDPLATFORM reasoning as above: Go cross-compiles
# natively (GOOS/GOARCH), so the compiler itself never needs to run under
# emulation — only GOARCH needs to change, not the machine executing `go
# build`. TARGETOS/TARGETARCH are populated automatically by buildx from
# whatever --platform the image is being built for.
FROM --platform=$BUILDPLATFORM golang:1.27.0-alpine AS server-builder
RUN apk add --no-cache git
WORKDIR /app
ARG TARGETOS
ARG TARGETARCH

COPY server/go.mod server/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY server/ .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /serversupervisor ./cmd/server
# seed-demo fills a DEMO_MODE=true database with fixture data (local demo mode
# + the release screenshot pipeline, see docker-compose.demo.yml). It hard-
# refuses to run without DEMO_MODE=true, so shipping it in every image is
# inert in production, not a security concern.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /seed-demo ./cmd/seed-demo

# ==== Stage 3: Final Image ====
FROM alpine:3.24
RUN apk add --no-cache --upgrade ca-certificates tzdata libcap && \
    addgroup -g 1000 supervisor && \
    adduser -D -u 1000 -G supervisor supervisor
WORKDIR /app

COPY --from=server-builder /serversupervisor .
COPY --from=server-builder /seed-demo .
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

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

ENTRYPOINT ["./serversupervisor"]
