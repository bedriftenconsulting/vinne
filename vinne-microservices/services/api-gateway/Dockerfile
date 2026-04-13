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

# Remove environment files to prevent local development mode in production
RUN find . -name ".env" -delete && find . -name "config.env" -delete && find . -name "*.env" -delete

# Build the binary with version information
WORKDIR /app/services/api-gateway
RUN go mod download -x && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X main.Version=${VERSION} \
              -X main.GitBranch=${GIT_BRANCH} \
              -X main.GitCommit=${GIT_HASH} \
              -X main.GitCommitCount=${GIT_COMMIT_COUNT} \
              -X main.BuildTime=${BUILD_TIME}" \
    -a -installsuffix cgo -o api-gateway cmd/server/main.go

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S appuser && \
    adduser -u 1001 -S appuser -G appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/services/api-gateway/api-gateway .

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 4000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:4000/health || exit 1

# Environment variables (will be overridden by Kubernetes)
ENV SERVER_PORT=4000 \
    SERVER_ENV=production \
    LOGGING_LEVEL=info

# Run the binary
CMD ["./api-gateway"]