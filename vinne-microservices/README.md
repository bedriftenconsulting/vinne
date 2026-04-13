# Rand Lottery Microservices Platform

Backend microservices platform for Rand Lottery Ltd Lottery Management System.

## Architecture Overview

This platform follows a microservices architecture with:

- **API Gateway** - REST API entry point with gRPC translation
- **gRPC Services** - Internal service communication
- **Event-Driven Communication** - Apache Kafka for async messaging
- **Database per Service** - PostgreSQL instances per service
- **Distributed Tracing** - OpenTelemetry/Jaeger

## Tech Stack

- **Language**: Go 1.23+
- **RPC Framework**: gRPC with Protocol Buffers
- **Database**: PostgreSQL 17
- **Cache**: Redis 7.4+
- **Message Queue**: Apache Kafka 3.9+ (KRaft mode)
- **Tracing**: OpenTelemetry/Jaeger
- **Container Orchestration**: Kubernetes with Helm

## Services

| Service | gRPC Port | Description |
|---------|-----------|-------------|
| API Gateway | 4000 (HTTP) | REST API entry point |
| Admin Management | 50057 | Admin authentication, user management, RBAC |
| Agent Auth | 50052 | Agent authentication and device management |
| Agent Management | 50058 | Agent registration, KYC, retailer management |
| Game | 50053 | Game configuration and prize structures |
| Terminal | 50054 | POS terminal management and monitoring |
| Wallet | 50059 | Agent/retailer wallet and transactions |
| Draw | 50060 | Draw execution and RNG management |
| Payment | 50061 | Payment gateway integration (MTN, Telecel, AirtelTigo) |
| Ticket | 50062 | Ticket generation and validation |
| Notification | 50063 | Multi-channel messaging (SMS, Email, Push) |
| Player | 50064 | Player authentication and management |

## Project Structure

```
randco-microservices/
├── services/                 # Individual microservices
│   ├── api-gateway/         # REST API Gateway
│   ├── service-admin-management/
│   ├── service-agent-auth/
│   ├── service-agent-management/
│   ├── service-draw/
│   ├── service-game/
│   ├── service-notification/
│   ├── service-payment/
│   ├── service-player/
│   ├── service-terminal/
│   ├── service-ticket/
│   └── service-wallet/
├── proto/                   # Protocol Buffer definitions
├── shared/                  # Shared libraries
├── helm/                    # Kubernetes Helm charts
├── argocd/                  # ArgoCD configurations
├── scripts/                 # Development scripts
└── docs/                    # Documentation
```

## Getting Started

### Prerequisites

- Go 1.23+
- Docker and Docker Compose
- PostgreSQL 17
- Redis 7.4+
- Apache Kafka 3.9+
- Protocol Buffer compiler (protoc)

### Local Development

1. **Start Infrastructure**:

```bash
./scripts/start-infrastructure.sh
```

This starts PostgreSQL, Redis, Kafka, and Jaeger containers.

2. **Start All Services**:

```bash
./scripts/start-services.sh
```

3. **Verify Services**:

```bash
./scripts/test-integration.sh
```

4. **Stop Services**:

```bash
./scripts/stop-services.sh
```

### Running Individual Services

```bash
cd services/service-admin-management
go run cmd/server/main.go
```

### Database Migrations

Each service manages its own migrations using Goose:

```bash
cd services/service-admin-management
goose -dir migrations postgres "postgresql://user:pass@localhost:5437/admin_management" up
```

## Configuration

### Environment Variables

Each service uses Viper-based configuration. Key environment variables:

```bash
# Server
SERVER_PORT=50057
SERVER_HOST=0.0.0.0

# Database
DATABASE_HOST=localhost
DATABASE_PORT=5437
DATABASE_NAME=admin_management
DATABASE_USER=admin_mgmt
DATABASE_PASSWORD=admin_mgmt123

# Redis
REDIS_HOST=localhost
REDIS_PORT=6384
REDIS_PASSWORD=

# Kafka
KAFKA_BROKERS=localhost:9092

# Tracing
JAEGER_ENDPOINT=http://localhost:4318/v1/traces
```

See `docs/SERVICE_ENVIRONMENT_VARIABLES.md` for complete reference.

## Kubernetes Deployment

Helm charts are provided in the `helm/` directory:

```bash
# Deploy all microservices
helm install rand-microservices ./helm/microservices -n microservices

# Deploy individual service
helm install admin-management ./helm/microservices/charts/service-admin-management -n microservices
```

See the main project README for detailed Kubernetes deployment instructions.

## API Documentation

- API Gateway endpoints: `docs/api/`
- Port reference: `docs/PORTS_REFERENCE.md`
- Environment variables: `docs/SERVICE_ENVIRONMENT_VARIABLES.md`

## Testing

Integration tests use real infrastructure via Testcontainers:

```bash
cd services/service-admin-management
go test ./...
```

## License

Private - Rand Lottery Ltd
