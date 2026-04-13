.PHONY: proto clean build test run migration-up migration-down migration-create

# Proto generation
proto:
	@echo "Generating protobuf code..."
	cd ../../ && protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/payment/v1/payment.proto
	@echo "✓ Protobuf code generated"

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	rm -f ../../proto/payment/v1/*.pb.go
	@echo "✓ Clean complete"

# Build the service
build:
	@echo "Building payment service..."
	go build -o bin/payment-service cmd/server/main.go
	@echo "✓ Build complete"

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "✓ Tests complete"

# Run the service
run:
	@echo "Starting payment service..."
	go run cmd/server/main.go

# Database migrations
migration-up:
	@echo "Running migrations..."
	goose -dir migrations postgres "$(DATABASE_URL)" up
	@echo "✓ Migrations complete"

migration-down:
	@echo "Rolling back last migration..."
	goose -dir migrations postgres "$(DATABASE_URL)" down
	@echo "✓ Rollback complete"

migration-create:
	@echo "Creating new migration: $(name)"
	goose -dir migrations create $(name) sql
	@echo "✓ Migration file created"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	@echo "✓ Dependencies installed"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✓ Code formatted"

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run
	@echo "✓ Linting complete"

# All (proto + build + test)
all: proto fmt lint build test
	@echo "✓ All tasks complete"
