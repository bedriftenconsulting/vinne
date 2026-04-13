# Player Service

The Player Service is a microservice responsible for player authentication, registration, profile management, and wallet operations in the Rand Lottery platform.

## Features

- Multi-channel player authentication (Mobile, Web, Telegram, USSD)
- OTP-based registration and verification
- USSD password-based authentication
- Player profile management
- Wallet operations (deposits/withdrawals)
- Session management and device tracking
- JWT token handling
- Integration with Notification, Wallet, and Payment services

## Quick Start

### Prerequisites

- Go 1.23+
- PostgreSQL 13+
- Redis 6+
- Docker (optional)

### Development Setup

1. Clone the repository and navigate to the service directory:
   ```bash
   cd services/service-player
   ```

2. Install dependencies:
   ```bash
   make deps
   ```

3. Install development tools:
   ```bash
   make install-tools
   ```

4. Start the service:
   ```bash
   make run
   ```

   Or with hot reload:
   ```bash
   make dev
   ```

### Configuration

The service uses Viper for configuration management. Configuration can be provided via:

- `config.yaml` file
- Environment variables (prefixed with uppercase and underscores)
- Command line flags

See `config.yaml` for default configuration values.

### Database Setup

1. Create the database:
   ```sql
   CREATE DATABASE player_db;
   ```

2. Run migrations:
   ```bash
   make migrate-up
   ```

### Testing

Run unit tests:
```bash
make test
```

Run tests with coverage:
```bash
make test-coverage
```

Run integration tests:
```bash
make test-integration
```

### Docker

Build Docker image:
```bash
make docker-build
```

Run with Docker Compose:
```bash
make docker-run
```

## API Documentation

The service exposes gRPC endpoints. See `proto/player.proto` for the complete API specification.

## Architecture

The service follows a layered architecture:

- **API Layer**: gRPC handlers
- **Business Logic**: Service layer with business rules
- **Data Access**: Repository pattern for data operations
- **Infrastructure**: Database, Redis, Kafka integrations

## Monitoring

The service includes:

- OpenTelemetry tracing with Jaeger
- Health checks
- Structured logging
- Metrics collection

## Development

### Adding New Features

1. Define the gRPC service in `proto/player.proto`
2. Generate protobuf code: `make proto`
3. Implement the handler in `internal/handlers/`
4. Add business logic in `internal/services/`
5. Create repository methods in `internal/repositories/`
6. Add tests

### Code Style

- Follow Go conventions and best practices
- Use interfaces for dependencies
- Handle errors explicitly
- Write tests for new functionality
- Use dependency injection

## License

Copyright © Rand Lottery Ltd. All rights reserved.
