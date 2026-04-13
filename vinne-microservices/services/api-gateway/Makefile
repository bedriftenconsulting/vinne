.PHONY: build run test clean docker-build docker-run

# Variables
SERVICE_NAME=api-gateway
BINARY_NAME=api-gateway
DOCKER_IMAGE=randco/$(SERVICE_NAME):latest
MAIN_PATH=cmd/server/main.go

# Build the binary
build:
	@echo "Building $(SERVICE_NAME)..."
	@go build -o bin/$(BINARY_NAME) $(MAIN_PATH)

# Run the service
run:
	@echo "Running $(SERVICE_NAME)..."
	@go run $(MAIN_PATH)

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	@docker run -p 8080:8080 --env-file .env $(DOCKER_IMAGE)

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# Generate mocks
mocks:
	@echo "Generating mocks..."
	@go generate ./...