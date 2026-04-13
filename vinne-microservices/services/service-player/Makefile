# Player Service Makefile

.PHONY: build test run clean deps proto migrate docker-build docker-run

# Variables
SERVICE_NAME := player-service
DOCKER_IMAGE := randco/$(SERVICE_NAME)
VERSION := latest
PORT := 50053

# Go commands
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

# Build the service
build:
	@echo "Building $(SERVICE_NAME)..."
	$(GOBUILD) -o bin/$(SERVICE_NAME) ./cmd/server
	@echo "Build complete: bin/$(SERVICE_NAME)"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run the service
run:
	@echo "Starting $(SERVICE_NAME) on port $(PORT)..."
	$(GOCMD) run ./cmd/server

# Run with hot reload (requires air)
dev:
	@echo "Starting $(SERVICE_NAME) with hot reload..."
	air

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "Clean complete"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies updated"

# Generate protobuf files
proto:
	@echo "Generating protobuf files..."
	protoc --go_out=. --go-grpc_out=. proto/*.proto
	@echo "Protobuf generation complete"

# Run database migrations
migrate-up:
	@echo "Running database migrations..."
	goose -dir migrations postgres "postgresql://postgres:password@localhost:5432/player_db?sslmode=disable" up

# Rollback database migrations
migrate-down:
	@echo "Rolling back database migrations..."
	goose -dir migrations postgres "postgresql://postgres:password@localhost:5432/player_db?sslmode=disable" down

# Create new migration
migrate-create:
	@echo "Creating new migration..."
	@read -p "Enter migration name: " name; \
	goose -dir migrations create $$name sql

# Docker commands
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(VERSION) .
	@echo "Docker image built: $(DOCKER_IMAGE):$(VERSION)"

# Run with Docker Compose
docker-run:
	@echo "Starting services with Docker Compose..."
	docker-compose up --build

# Stop Docker Compose
docker-stop:
	@echo "Stopping Docker Compose services..."
	docker-compose down

# Run integration tests with testcontainers
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration ./...

# Lint code
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	@echo "Development tools installed"

# Show help
help:
	@echo "Available commands:"
	@echo "  build          - Build the service binary"
	@echo "  test           - Run unit tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-integration - Run integration tests"
	@echo "  run            - Run the service"
	@echo "  dev            - Run with hot reload (requires air)"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Download and tidy dependencies"
	@echo "  proto          - Generate protobuf files"
	@echo "  migrate-up     - Run database migrations"
	@echo "  migrate-down    - Rollback database migrations"
	@echo "  migrate-create  - Create new migration"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run with Docker Compose"
	@echo "  docker-stop    - Stop Docker Compose services"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  install-tools  - Install development tools"
	@echo "  help           - Show this help message"
