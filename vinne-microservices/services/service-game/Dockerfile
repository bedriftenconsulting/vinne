# Build stage
FROM golang:1.24-alpine AS builder

# Build arguments for version information
ARG GIT_BRANCH=unknown
ARG GIT_COMMIT_COUNT=0
ARG GIT_HASH=unknown
ARG BUILD_TIME=unknown
ARG VERSION=dev
# Rebuild trigger: 2025-09-11T04:55:00Z

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy entire workspace (needed for go.work)
COPY . .

# Remove environment files to prevent local development mode in production
RUN find . -name ".env" -delete && find . -name "config.env" -delete && find . -name "*.env" -delete

# Build the binary with version information
WORKDIR /app/services/service-game
RUN go mod download -x && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X main.Version=${VERSION} \
              -X main.GitBranch=${GIT_BRANCH} \
              -X main.GitCommit=${GIT_HASH} \
              -X main.GitCommitCount=${GIT_COMMIT_COUNT} \
              -X main.BuildTime=${BUILD_TIME}" \
    -a -installsuffix cgo -o game cmd/server/main.go

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
COPY --from=builder /app/services/service-game/game .

# Copy migrations if they exist
COPY --from=builder /app/services/service-game/migrations* ./migrations/

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 50053

# Health check (gRPC health check)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/bin/sh", "-c", "echo '' | nc -w 1 localhost 50053 || exit 1"]

# Environment variables (will be overridden by Kubernetes/Vault)
ENV SERVER_PORT=50053 \
    SERVER_ENV=production \
    LOGGING_LEVEL=info \
    DATABASE_URL="" \
    REDIS_URL=""

# Run the binary
CMD ["./game"]