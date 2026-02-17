# ==== Build Agent ====
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /agent ./cmd/agent

# This Dockerfile is for building the agent binary only.
# The agent should run as a native binary on the host (not in Docker).

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /agent .
ENTRYPOINT ["./agent"]
