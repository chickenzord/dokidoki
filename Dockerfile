# Stage 1: Build the React frontend
FROM oven/bun:1-alpine AS frontend-builder
WORKDIR /app/web

# Copy web dependency manifests
COPY web/package.json web/bun.lock* ./

# Install frontend dependencies
RUN bun install --frozen-lockfile || bun install

# Copy web source code
COPY web/ ./

# Build frontend SPA
RUN bun run build

# Stage 2: Build the Go backend
FROM golang:alpine AS backend-builder
WORKDIR /app

# Copy Go dependency manifests
COPY go.mod go.sum ./

# Download Go dependencies
RUN go mod download

# Copy backend source code and embedded assets
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/embed.go ./web/embed.go

# Copy built frontend assets from frontend-builder
COPY --from=frontend-builder /app/web/dist ./web/dist

# Build-time arguments for versioning
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# Build Go binary with ldflags
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -o /app/bin/dokidoki \
    ./cmd/dokidoki

# Stage 3: Runner
FROM alpine:3.21 AS runner
RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /app/bin/dokidoki /usr/local/bin/dokidoki

# Environment defaults
ENV DOKIDOKI_PORT=8080
ENV DOKIDOKI_STACKS_DIR=/opt/stacks

# Create default stacks directory
RUN mkdir -p /opt/stacks

# Expose default HTTP/API port
EXPOSE 8080

# Health check verifies connectivity to the Dokidoki host ping endpoint
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD curl -f http://localhost:${DOKIDOKI_PORT}/api/v1/host/ping || exit 1

# Run Dokidoki
ENTRYPOINT ["/usr/local/bin/dokidoki"]
