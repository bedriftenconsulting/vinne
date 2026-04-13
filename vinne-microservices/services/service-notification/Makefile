.PHONY: help run build test clean proto migrate migrate-up migrate-down migrate-create lint fmt

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

run: ## Run the service locally
	@echo "Starting Admin Management Service..."
	go run cmd/server/main.go

build: ## Build the service binary
	@echo "Building Admin Management Service..."
	go build -o bin/service-admin-management cmd/server/main.go

test: ## Run tests
	@echo "Running tests..."
	go test -v ./...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	go test -v ./internal/repositories/...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/ tmp/

proto: ## Generate protobuf code
	@echo "Generating protobuf files..."
	protoc --go_out=. --go_opt=paths=source_relative \
	--go-grpc_out=. --go-grpc_opt=paths=source_relative \
	proto/notification.proto

migrate: ## Show migration help
	@echo "Migration commands:"
	@echo "  make migrate-up     - Apply all pending migrations"
	@echo "  make migrate-down   - Rollback last migration"
	@echo "  make migrate-create NAME=migration_name - Create new migration"
	@echo "  make migrate-status - Show migration status"

migrate-up: ## Apply all pending migrations
	@echo "Applying migrations..."
	goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down: ## Rollback last migration
	@echo "Rolling back last migration..."
	goose -dir migrations postgres "$(DATABASE_URL)" down

migrate-create: ## Create new migration (usage: make migrate-create NAME=create_users_table)
	@if [ -z "$(NAME)" ]; then echo "Please provide NAME: make migrate-create NAME=migration_name"; exit 1; fi
	@echo "Creating migration: $(NAME)"
	goose -dir migrations create $(NAME) sql

migrate-status: ## Show migration status
	@echo "Migration status:"
	goose -dir migrations postgres "$(DATABASE_URL)" status

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...

dev: ## Start development environment with hot reload
	@echo "Starting development environment..."
	air

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t randco/service-admin-management:latest .

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run --rm -p 50053:50053 -p 8080:8080 randco/service-admin-management:latest

# Database connection string from environment or default
DATABASE_URL ?= postgresql://admin_mgmt:admin_mgmt123@localhost:5437/admin_management?sslmode=disable