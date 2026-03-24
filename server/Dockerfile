# ==== Stage 1: Build Frontend ====
FROM node:25-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ .
RUN npm run build

# ==== Stage 2: Build Go Server ====
FROM golang:1.26-alpine AS server-builder
RUN apk add --no-cache git
WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /serversupervisor ./cmd/server

# ==== Stage 3: Final Image ====
FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=server-builder /serversupervisor .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

EXPOSE 8080

ENTRYPOINT ["./serversupervisor"]
