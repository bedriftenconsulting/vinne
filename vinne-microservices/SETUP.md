# Spiel Microservices Setup Guide

Detailed setup instructions for the Spiel microservices backend.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Infrastructure Setup](#infrastructure-setup)
3. [Service Configuration](#service-configuration)
4. [Database Migrations](#database-migrations)
5. [Starting Services](#starting-services)
6. [Verification](#verification)
7. [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Software

- **Docker Desktop** (Windows/Mac) or **Docker Engine** (Linux)
  - Version 20.10 or higher
  - Docker Compose v2.0+
- **Go** 1.21 or higher
  - Verify: `go version`
- **Git**

### System Requirements

- 8GB RAM minimum (16GB recommended)
- 20GB free disk space
- Ports 4000, 5432-5443, 6379-6390, 9092, 16686 available

## Infrastructure Setup

### Step 1: Start Docker Infrastructure

```bash
cd spiel-microservices
docker-compose up -d
```

This command starts:
- **9 PostgreSQL databases** (one per service)
- **10 Redis instances** (service-specific + shared)
- **Kafka broker** for event streaming
- **Jaeger** for distributed tracing

### Step 2: Verify Infrastructure

```bash
# Check all containers are running
docker ps

# Expected output: ~20 containers in "Up" status
```

### Step 3: Check Container Health

```bash
# PostgreSQL databases
docker exec -it spiel-microservices-service-player-db-1 pg_isready -U player_user

# Redis instances
docker exec -it spiel-microservices-service-player-redis-1 redis-cli ping
# Should return: PONG

# Kafka
docker exec -it spiel-microservices-kafka-1 kafka-topics --list --bootstrap-server localhost:9092
```

## Service Configuration

### Environment Files

Each service requires a `.env` file in its directory. Below are the complete configurations:

#### API Gateway (services/api-gateway/.env)

```env
# Server Configuration
SERVER_PORT=4000
SERVER_HOST=localhost
METRICS_PORT=8080

# Database Configuration
DATABASE_URL=postgresql://api_gateway:gateway123@localhost:5432/api_gateway?sslmode=disable
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=api_gateway
DATABASE_PASSWORD=gateway123
DATABASE_NAME=api_gateway

# Redis Configuration
REDIS_URL=redis://localhost:6380/0
REDIS_HOST=localhost
REDIS_PORT=6380

# Shared Redis
SHARED_REDIS_HOST=localhost
SHARED_REDIS_PORT=6379

# Kafka Configuration
KAFKA_BROKERS=localhost:9092
KAFKA_CONSUMER_GROUP=api-gateway

# OpenTelemetry
JAEGER_ENDPOINT=http://localhost:4318
TRACING_JAEGER_ENDPOINT=http://localhost:4318
SERVICE_NAME=api-gateway

# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key-change-in-production

# Service Discovery
SERVICE_PLAYER_URL=localhost:50064
SERVICE_WALLET_URL=localhost:50059
SERVICE_GAME_URL=localhost:50053
SERVICE_DRAW_URL=localhost:50060
SERVICE_TICKET_URL=localhost:50062
SERVICE_ADMIN_MANAGEMENT_URL=localhost:50057
SERVICE_AGENT_AUTH_URL=localhost:50055
SERVICE_AGENT_MANAGEMENT_URL=localhost:50056
SERVICE_TERMINAL_URL=localhost:50058
SERVICE_PAYMENT_URL=localhost:50061
SERVICE_NOTIFICATION_URL=localhost:50063
```

#### Player Service (services/service-player/.env)

```env
# Server Configuration
SERVER_PORT=50064
SERVER_HOST=localhost
METRICS_PORT=8080

# Database Configuration
DATABASE_URL=postgresql://player_user:player123@localhost:5443/player_db?sslmode=disable
DATABASE_HOST=localhost
DATABASE_PORT=5443
DATABASE_USER=player_user
DATABASE_PASSWORD=player123
DATABASE_NAME=player_db

# Redis Configuration
REDIS_URL=redis://localhost:6390/0
REDIS_HOST=localhost
REDIS_PORT=6390

# Shared Redis
SHARED_REDIS_HOST=localhost
SHARED_REDIS_PORT=6379

# Kafka Configuration
KAFKA_BROKERS=localhost:9092
KAFKA_CONSUMER_GROUP=player-service

# OpenTelemetry
JAEGER_ENDPOINT=http://localhost:4318
TRACING_JAEGER_ENDPOINT=http://localhost:4318
SERVICE_NAME=service-player

# Security Configuration
SECURITY_JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=spiel-player-service
SECURITY_ACCESS_TOKEN_EXPIRY=15m
SECURITY_REFRESH_TOKEN_EXPIRY=168h
```

#### Admin Management Service (services/service-admin-management/.env)

```env
# Server Configuration
SERVER_PORT=50057
SERVER_HOST=localhost
METRICS_PORT=8080

# Database Configuration
DATABASE_URL=postgresql://admin_mgmt:admin_mgmt123@localhost:5437/admin_management?sslmode=disable
DATABASE_HOST=localhost
DATABASE_PORT=5437
DATABASE_USER=admin_mgmt
DATABASE_PASSWORD=admin_mgmt123
DATABASE_NAME=admin_management

# Redis Configuration
REDIS_URL=redis://localhost:6384/0
REDIS_HOST=localhost
REDIS_PORT=6384

# Shared Redis
SHARED_REDIS_HOST=localhost
SHARED_REDIS_PORT=6379

# Kafka Configuration
KAFKA_BROKERS=localhost:9092
KAFKA_CONSUMER_GROUP=admin-management-service

# OpenTelemetry
JAEGER_ENDPOINT=http://localhost:4318
TRACING_JAEGER_ENDPOINT=http://localhost:4318
SERVICE_NAME=service-admin-management

# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key-change-in-production
```

#### Wallet Service (services/service-wallet/.env)

```env
# Server Configuration
SERVER_PORT=50059
SERVER_HOST=localhost
METRICS_PORT=8080

# Database Configuration
DATABASE_URL=postgresql://wallet_user:wallet123@localhost:5439/wallet_db?sslmode=disable
DATABASE_HOST=localhost
DATABASE_PORT=5439
DATABASE_USER=wallet_user
DATABASE_PASSWORD=wallet123
DATABASE_NAME=wallet_db

# Redis Configuration
REDIS_URL=redis://localhost:6389/0
REDIS_HOST=localhost
REDIS_PORT=6389

# Shared Redis
SHARED_REDIS_HOST=localhost
SHARED_REDIS_PORT=6379

# Kafka Configuration
KAFKA_BROKERS=localhost:9092
KAFKA_CONSUMER_GROUP=wallet-service

# OpenTelemetry
JAEGER_ENDPOINT=http://localhost:4318
TRACING_JAEGER_ENDPOINT=http://localhost:4318
SERVICE_NAME=service-wallet
```

#### Game Service (services/service-game/.env)

```env
# Server Configuration
SERVER_PORT=50053
SERVER_HOST=localhost
METRICS_PORT=8080

# Database Configuration
DATABASE_URL=postgresql://game_user:game123@localhost:5433/game_db?sslmode=disable
DATABASE_HOST=localhost
DATABASE_PORT=5433
DATABASE_USER=game_user
DATABASE_PASSWORD=game123
DATABASE_NAME=game_db

# Redis Configuration
REDIS_URL=redis://localhost:6383/0
REDIS_HOST=localhost
REDIS_PORT=6383

# Shared Redis
SHARED_REDIS_HOST=localhost
SHARED_REDIS_PORT=6379

# Kafka Configuration
KAFKA_BROKERS=localhost:9092
KAFKA_CONSUMER_GROUP=game-service

# OpenTelemetry
JAEGER_ENDPOINT=http://localhost:4318
TRACING_JAEGER_ENDPOINT=http://localhost:4318
SERVICE_NAME=service-game
```

#### Draw Service (services/service-draw/.env)

```env
# Server Configuration
SERVER_PORT=50060
SERVER_HOST=localhost
METRICS_PORT=8080

# Database Configuration
DATABASE_URL=postgresql://draw_user:draw123@localhost:5440/draw_db?sslmode=disable
DATABASE_HOST=localhost
DATABASE_PORT=5440
DATABASE_USER=draw_user
DATABASE_PASSWORD=draw123
DATABASE_NAME=draw_db

# Redis Configuration
REDIS_URL=redis://localhost:6388/0
REDIS_HOST=localhost
REDIS_PORT=6388

# Shared Redis
SHARED_REDIS_HOST=localhost
SHARED_REDIS_PORT=6379

# Kafka Configuration
KAFKA_BROKERS=localhost:9092
KAFKA_CONSUMER_GROUP=draw-service

# OpenTelemetry
JAEGER_ENDPOINT=http://localhost:4318
TRACING_JAEGER_ENDPOINT=http://localhost:4318
SERVICE_NAME=service-draw
```

#### Ticket Service (services/service-ticket/.env)

```env
# Server Configuration
SERVER_PORT=50062
SERVER_HOST=localhost
METRICS_PORT=8080

# Database Configuration
DATABASE_URL=postgresql://ticket_user:ticket123@localhost:5442/ticket_db?sslmode=disable
DATABASE_HOST=localhost
DATABASE_PORT=5442
DATABASE_USER=ticket_user
DATABASE_PASSWORD=ticket123
DATABASE_NAME=ticket_db

# Redis Configuration
REDIS_URL=redis://localhost:6387/0
REDIS_HOST=localhost
REDIS_PORT=6387

# Shared Redis
SHARED_REDIS_HOST=localhost
SHARED_REDIS_PORT=6379

# Kafka Configuration
KAFKA_BROKERS=localhost:9092
KAFKA_CONSUMER_GROUP=ticket-service

# OpenTelemetry
JAEGER_ENDPOINT=http://localhost:4318
TRACING_JAEGER_ENDPOINT=http://localhost:4318
SERVICE_NAME=service-ticket
```

## Database Migrations

Run migrations for each service that has a database:

```bash
# Player Service
cd services/service-player
go run cmd/migrate/main.go up

# Admin Management Service
cd services/service-admin-management
go run cmd/migrate/main.go up

# Wallet Service
cd services/service-wallet
go run cmd/migrate/main.go up

# Game Service
cd services/service-game
go run cmd/migrate/main.go up

# Draw Service
cd services/service-draw
go run cmd/migrate/main.go up

# Ticket Service
cd services/service-ticket
go run cmd/migrate/main.go up

# API Gateway
cd services/api-gateway
go run cmd/migrate/main.go up
```

### Verify Migrations

```bash
# Check Player Service tables
docker exec -it spiel-microservices-service-player-db-1 psql -U player_user -d player_db -c "\dt"

# Check Admin Management tables
docker exec -it spiel-microservices-service-admin-management-db-1 psql -U admin_mgmt -d admin_management -c "\dt"
```

## Starting Services

### Recommended Startup Order

Start services in this sequence to ensure proper dependency resolution:

```bash
# Terminal 1: API Gateway
cd services/api-gateway
go run cmd/server/main.go

# Terminal 2: Player Service
cd services/service-player
go run cmd/server/main.go

# Terminal 3: Admin Management
cd services/service-admin-management
go run cmd/server/main.go

# Terminal 4: Wallet Service
cd services/service-wallet
go run cmd/server/main.go

# Terminal 5: Game Service
cd services/service-game
go run cmd/server/main.go

# Terminal 6: Draw Service
cd services/service-draw
go run cmd/server/main.go

# Terminal 7: Ticket Service
cd services/service-ticket
go run cmd/server/main.go
```

### Using Background Processes (Optional)

For Windows PowerShell:
```powershell
# Start in background
Start-Process -NoNewWindow go -ArgumentList "run", "cmd/server/main.go" -WorkingDirectory "services/api-gateway"
```

For Linux/Mac:
```bash
# Start in background
cd services/api-gateway && go run cmd/server/main.go &
```

## Verification

### Health Checks

```bash
# API Gateway
curl http://localhost:4000/health

# Player Service
curl http://localhost:50064/health

# Admin Management
curl http://localhost:50057/health
```

### Test API Endpoints

```bash
# Register a player
curl -X POST http://localhost:4000/api/v1/players/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "phone_number": "233256826832",
    "password": "Test@123",
    "first_name": "John",
    "last_name": "Doe"
  }'

# Admin login
curl -X POST http://localhost:4000/api/v1/admin/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "superadmin@randco.com",
    "password": "Admin@123!"
  }'
```

### View Traces in Jaeger

1. Open http://localhost:16686
2. Select service from dropdown
3. Click "Find Traces"
4. Explore request flows across services

## Troubleshooting

### Go Module Issues

```bash
# Update dependencies
cd shared
go get -u github.com/redis/go-redis/v9@v9.18.0
go mod tidy

# Rebuild
cd services/<service-name>
go build ./...
```

### Database Connection Errors

```bash
# Check database is running
docker ps | grep db

# Test connection
docker exec -it <db-container> psql -U <user> -d <database> -c "SELECT 1;"

# View logs
docker logs <db-container>
```

### Redis Connection Errors

```bash
# Test Redis connection
docker exec -it <redis-container> redis-cli ping

# Check Redis logs
docker logs <redis-container>
```

### Port Conflicts

```bash
# Windows: Find process using port
netstat -ano | findstr :4000
taskkill /PID <pid> /F

# Linux/Mac: Find and kill process
lsof -i :4000
kill -9 <pid>
```

### JWT Token Issues

Ensure `JWT_SECRET` matches across:
- API Gateway
- Player Service (`SECURITY_JWT_SECRET`)
- Admin Management Service

### Service Can't Find Other Services

Check `SERVICE_*_URL` environment variables in API Gateway `.env` file match the actual service ports.

## Performance Tuning

### Database Connection Pooling

Edit service configuration:
```go
// In database initialization
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

### Redis Connection Pooling

```go
// In Redis client initialization
&redis.Options{
    PoolSize:     10,
    MinIdleConns: 5,
}
```

## Monitoring

### Metrics

Each service exposes Prometheus metrics on port 8080:
```bash
curl http://localhost:8080/metrics
```

### Logs

Services log to stdout. Redirect to files if needed:
```bash
go run cmd/server/main.go > service.log 2>&1
```

## Next Steps

- Set up frontend applications (see main README.md)
- Configure CI/CD pipelines
- Set up production environment
- Implement monitoring and alerting
- Review security configurations

## Additional Resources

- [Go Documentation](https://golang.org/doc/)
- [Docker Documentation](https://docs.docker.com/)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [Redis Documentation](https://redis.io/documentation)
- [Kafka Documentation](https://kafka.apache.org/documentation/)
