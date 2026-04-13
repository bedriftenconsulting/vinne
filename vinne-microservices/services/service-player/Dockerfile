# Build stage
FROM golang:1.24-alpine AS builder

# Build arguments for version information
ARG GIT_BRANCH=unknown
ARG GIT_COMMIT_COUNT=0
ARG GIT_HASH=unknown
ARG BUILD_TIME=unknown
ARG VERSION=dev

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy entire workspace (needed for go.work)
COPY . .

# Build the binary with version information
WORKDIR /app/services/service-player
RUN go mod download -x && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X main.Version=${VERSION} \
              -X main.GitBranch=${GIT_BRANCH} \
              -X main.GitCommit=${GIT_HASH} \
              -X main.GitCommitCount=${GIT_COMMIT_COUNT} \
              -X main.BuildTime=${BUILD_TIME}" \
    -a -installsuffix cgo -o player cmd/server/main.go

# Final stage
FROM alpine:3.19

# Install runtime dependencies (including goose for migrations)
RUN apk --no-cache add ca-certificates tzdata

# Install goose for migrations
RUN apk add --no-cache --virtual .build-deps wget && \
    wget -O /usr/local/bin/goose https://github.com/pressly/goose/releases/download/v3.20.0/goose_linux_x86_64 && \
    chmod +x /usr/local/bin/goose && \
    apk del .build-deps

# Create non-root user
RUN addgroup -g 1001 -S appuser && \
    adduser -u 1001 -S appuser -G appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/services/service-player/player .

# Copy migrations
COPY --from=builder /app/services/service-player/migrations ./migrations

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 50063

# Health check (gRPC health check)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/bin/sh", "-c", "echo '' | nc -w 1 localhost 50063 || exit 1"]

# Environment variables (will be overridden by Kubernetes/Vault)
# Note: Sensitive values like JWT_SECRET should be provided at runtime via secrets
ENV SERVER_PORT=50063 \
    SERVER_ENV=production \
    LOG_LEVEL=info

# Run the binary
CMD ["./player"]