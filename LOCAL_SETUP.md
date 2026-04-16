# Vinne Platform — Local Development Setup

Complete guide for cloning the repository, running all migrations, and getting every service running locally.

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Repository Structure](#2-repository-structure)
3. [Services & Ports Reference](#3-services--ports-reference)
4. [Prerequisites](#4-prerequisites)
5. [Clone the Repository](#5-clone-the-repository)
6. [Backend — Microservices Setup](#6-backend--microservices-setup)
   - [6.1 Start Infrastructure (Docker)](#61-start-infrastructure-docker)
   - [6.2 Configure Environment Variables](#62-configure-environment-variables)
   - [6.3 Run Database Migrations](#63-run-database-migrations)
   - [6.4 Start All Services](#64-start-all-services)
7. [Admin Web Portal Setup](#7-admin-web-portal-setup)
8. [Player Website Setup](#8-player-website-setup)
9. [Verify Everything is Running](#9-verify-everything-is-running)
10. [Default Credentials](#10-default-credentials)
11. [Useful Commands](#11-useful-commands)
12. [Troubleshooting](#12-troubleshooting)

---

## 1. Project Overview

This is a **lottery management platform** composed of three main parts:

| Part | Location | Tech | Purpose |
|------|----------|------|---------|
| **Microservices Backend** | `vinne-microservices/` | Go 1.24, gRPC, PostgreSQL, Redis, Kafka | Core business logic — 12 services |
| **Admin Web Portal** | `vinne-admin-web/` | React 19, TypeScript, Vite, TanStack | Internal dashboard for admins |
| **Player Website** | `/` (root) | React 18, TypeScript, Vite | Public-facing competition site |

All three live in the same monorepo at `https://github.com/winbigafrica/vinne.git`.

---

## 2. Repository Structure

```
vinne/
├── vinne-microservices/          # Go microservices backend
│   ├── services/
│   │   ├── api-gateway/          # REST → gRPC gateway (port 4000)
│   │   ├── service-admin-management/
│   │   ├── service-agent-auth/
│   │   ├── service-agent-management/
│   │   ├── service-draw/
│   │   ├── service-game/
│   │   ├── service-notification/
│   │   ├── service-payment/
│   │   ├── service-player/
│   │   ├── service-terminal/
│   │   ├── service-ticket/
│   │   └── service-wallet/
│   ├── proto/                    # Protocol Buffer definitions
│   ├── shared/                   # Shared Go libraries
│   ├── scripts/                  # Helper scripts
│   ├── docker-compose.yml        # Full infrastructure stack
│   └── Makefile
├── vinne-admin-web/              # Admin dashboard (React 19)
│   ├── src/
│   ├── .env.localdev             # Local dev environment config
│   └── package.json
├── src/                          # Player website (React 18)
├── package.json
└── LOCAL_SETUP.md                # ← you are here
```

---

## 3. Services & Ports Reference

### Microservices (gRPC)

| Service | gRPC Port | Metrics Port | Database Port | Redis Port | Description |
|---------|-----------|--------------|---------------|------------|-------------|
| **API Gateway** | `4000` (HTTP/REST) | `8080` | — | `6379` (shared) | REST entry point, routes to all gRPC services |
| **Admin Management** | `50057` | `8085` | `5437` | `6384` | Admin auth, user management, RBAC, permissions |
| **Agent Auth** | `50052` | — | `5434` | `6381` | Agent device authentication, session management |
| **Agent Management** | `50058` | `8086` | `5435` | `6382` | Agent registration, KYC, retailer management |
| **Game** | `50053` | `8090` | `5441` | `6388` | Game configuration, prize structures, scheduling |
| **Draw** | `50060` | `8091` | `5436` | `6383` | Draw execution, RNG, winner selection |
| **Wallet** | `50059` | `8087` | `5438` | `6385` | Agent/retailer wallets, transactions, commissions |
| **Terminal** | `50054` | `8088` | `5439` | `6386` | POS terminal management and monitoring |
| **Payment** | `50061` | `8089` | `5440` | `6387` | Payment gateway (MTN MoMo, Telecel, AirtelTigo) |
| **Ticket** | `50062` | `8092` | `5442` | `6389` | Ticket generation, validation, cancellation |
| **Notification** | `50063` | `8093` | `5443` | `6390` | SMS, Email, Push notifications (Hubtel, Mailgun) |
| **Player** | `50064` | `8094` | `5444` | `6391` | Player auth, profile, ticket history |

### Infrastructure Services

| Service | Port(s) | Purpose |
|---------|---------|---------|
| **Kafka** | `9092` | Event streaming between services |
| **Shared Redis** | `6379` | Session store, shared cache |
| **Jaeger UI** | `16686` | Distributed tracing dashboard |
| **Prometheus** | `9090` | Metrics collection |
| **Grafana** | `3001` | Metrics dashboards (admin/admin) |
| **MinIO Console** | `9001` | S3-compatible object storage UI |
| **MinIO API** | `9000` | Object storage API |

### Frontend Applications

| App | Port | Description |
|-----|------|-------------|
| **Player Website** | `5173` | Public-facing competition site |
| **Admin Portal** | `6176` | Internal admin dashboard |

---

## 4. Prerequisites

Install the following before starting:

| Tool | Version | Install |
|------|---------|---------|
| **Git** | Any | https://git-scm.com |
| **Go** | 1.24+ | https://go.dev/dl |
| **Docker Desktop** | 20.10+ | https://www.docker.com/products/docker-desktop |
| **Node.js** | 18+ | https://nodejs.org |
| **npm** | 9+ | Bundled with Node.js |

Verify your setup:

```bash
git --version
go version        # should be go1.24.x or higher
docker --version
docker compose version
node --version    # should be v18.x or higher
npm --version
```

> **System requirements:** 16 GB RAM recommended (8 GB minimum), 20 GB free disk space.

---

## 5. Clone the Repository

```bash
git clone https://github.com/winbigafrica/vinne.git
cd vinne
```

The repo contains all three parts — microservices, admin portal, and player website — in one place.

---

## 6. Backend — Microservices Setup

All commands below run from inside the `vinne-microservices/` directory unless stated otherwise.

### 6.1 Start Infrastructure (Docker)

This single command starts all databases, Redis instances, Kafka, Jaeger, Prometheus, Grafana, and MinIO:

```bash
cd vinne-microservices
docker compose up -d
```

Or use the helper script which also runs migrations automatically:

```bash
./scripts/start-infrastructure.sh
```

Wait about 10–15 seconds for all containers to become healthy. Verify with:

```bash
docker ps
```

You should see roughly 25+ containers in `Up` status. Key ones to confirm:

```
randco-microservices-service-admin-management-db-1   Up
randco-microservices-service-game-db-1               Up
randco-microservices-service-player-db-1             Up
randco-microservices-kafka-1                         Up
randco-microservices-jaeger-1                        Up
```

### 6.2 Configure Environment Variables

Each service reads a `.env` file from its own directory. Copy the example files and fill in values:

```bash
# From vinne-microservices/
for service in services/*/; do
  if [ -f "$service/.env.example" ]; then
    cp "$service/.env.example" "$service/.env"
  fi
done
```

If `.env.example` files are not present, create `.env` files manually. The critical values that **must match across all services** are:

```env
JWT_SECRET=your-super-secret-jwt-key-minimum-32-chars
SECURITY_JWT_ISSUER=randlotteryltd
```

> ⚠️ **Important:** `JWT_SECRET` must be identical in every service's `.env` file. If they differ, authentication will fail across service boundaries.

Below are the minimal `.env` contents for each service for local development:

#### `services/api-gateway/.env`
```env
SERVER_PORT=4000
REDIS_HOST=localhost
REDIS_PORT=6379
JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=randlotteryltd
KAFKA_BROKERS=localhost:9092
JAEGER_ENDPOINT=http://localhost:4318
SERVICES_ADMIN_MANAGEMENT_HOST=localhost
SERVICES_ADMIN_MANAGEMENT_PORT=50057
SERVICES_AGENT_AUTH_HOST=localhost
SERVICES_AGENT_AUTH_PORT=50052
SERVICES_AGENT_MANAGEMENT_HOST=localhost
SERVICES_AGENT_MANAGEMENT_PORT=50058
SERVICES_WALLET_HOST=localhost
SERVICES_WALLET_PORT=50059
SERVICES_GAME_HOST=localhost
SERVICES_GAME_PORT=50053
SERVICES_DRAW_HOST=localhost
SERVICES_DRAW_PORT=50060
SERVICES_TICKET_HOST=localhost
SERVICES_TICKET_PORT=50062
SERVICES_PAYMENT_HOST=localhost
SERVICES_PAYMENT_PORT=50061
SERVICES_TERMINAL_HOST=localhost
SERVICES_TERMINAL_PORT=50054
SERVICES_NOTIFICATION_HOST=localhost
SERVICES_NOTIFICATION_PORT=50063
SERVICES_PLAYER_HOST=localhost
SERVICES_PLAYER_PORT=50064
```

#### `services/service-admin-management/.env`
```env
SERVER_PORT=50057
DATABASE_URL=postgresql://admin_mgmt:admin_mgmt123@localhost:5437/admin_management?sslmode=disable
REDIS_HOST=localhost
REDIS_PORT=6384
JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=randlotteryltd
KAFKA_BROKERS=localhost:9092
JAEGER_ENDPOINT=http://localhost:4318
```

#### `services/service-agent-auth/.env`
```env
SERVER_PORT=50052
DATABASE_URL=postgresql://agent:agent123@localhost:5434/agent_auth?sslmode=disable
REDIS_HOST=localhost
REDIS_PORT=6381
JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=randlotteryltd
KAFKA_BROKERS=localhost:9092
JAEGER_ENDPOINT=http://localhost:4318
```

#### `services/service-agent-management/.env`
```env
SERVER_PORT=50058
DATABASE_URL=postgresql://agent_mgmt:agent_mgmt123@localhost:5435/agent_management?sslmode=disable
REDIS_HOST=localhost
REDIS_PORT=6382
JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=randlotteryltd
KAFKA_BROKERS=localhost:9092
JAEGER_ENDPOINT=http://localhost:4318
SERVICES_AGENT_AUTH_HOST=localhost
SERVICES_AGENT_AUTH_PORT=50052
SERVICES_WALLET_HOST=localhost
SERVICES_WALLET_PORT=50059
```

#### `services/service-game/.env`
```env
SERVER_PORT=50053
DATABASE_URL=postgresql://game:game123@localhost:5441/game_service?sslmode=disable
REDIS_HOST=localhost
REDIS_PORT=6388
JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=randlotteryltd
KAFKA_BROKERS=localhost:9092
JAEGER_ENDPOINT=http://localhost:4318
SERVICES_DRAW_HOST=localhost
SERVICES_DRAW_PORT=50060
SERVICES_TICKET_HOST=localhost
SERVICES_TICKET_PORT=50062
SERVICES_NOTIFICATION_HOST=localhost
SERVICES_NOTIFICATION_PORT=50063
SCHEDULER_TIMEZONE=Africa/Accra
```

#### `services/service-draw/.env`
```env
SERVER_PORT=50060
DATABASE_URL=postgresql://draw_service:draw_service123@localhost:5436/draw_service?sslmode=disable
REDIS_HOST=localhost
REDIS_PORT=6383
JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=randlotteryltd
KAFKA_BROKERS=localhost:9092
JAEGER_ENDPOINT=http://localhost:4318
SERVICES_TICKET_HOST=localhost
SERVICES_TICKET_PORT=50062
SERVICES_WALLET_HOST=localhost
SERVICES_WALLET_PORT=50059
```

#### `services/service-wallet/.env`
```env
SERVER_PORT=50059
DATABASE_URL=postgresql://wallet:wallet123@localhost:5438/wallet_service?sslmode=disable
REDIS_HOST=localhost
REDIS_PORT=6385
JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=randlotteryltd
KAFKA_BROKERS=localhost:9092
JAEGER_ENDPOINT=http://localhost:4318
SERVICES_AGENT_MANAGEMENT_HOST=localhost
SERVICES_AGENT_MANAGEMENT_PORT=50058
SERVICES_PAYMENT_HOST=localhost
SERVICES_PAYMENT_PORT=50061
```

#### `services/service-terminal/.env`
```env
SERVER_PORT=50054
DATABASE_URL=postgresql://terminal:terminal123@localhost:5439/terminal_service?sslmode=disable
REDIS_HOST=localhost
REDIS_PORT=6386
JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=randlotteryltd
KAFKA_BROKERS=localhost:9092
JAEGER_ENDPOINT=http://localhost:4318
SERVICES_AGENT_AUTH_HOST=localhost
SERVICES_AGENT_AUTH_PORT=50052
```

#### `services/service-payment/.env`
```env
SERVER_PORT=50061
DATABASE_URL=postgresql://payment:payment123@localhost:5440/payment_service?sslmode=disable
REDIS_HOST=localhost
REDIS_PORT=6387
JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=randlotteryltd
KAFKA_BROKERS=localhost:9092
JAEGER_ENDPOINT=http://localhost:4318
PAYMENT_DEFAULT_CURRENCY=GHS
PAYMENT_TEST_MODE=true
SERVICES_WALLET_HOST=localhost
SERVICES_WALLET_PORT=50059
```

#### `services/service-ticket/.env`
```env
SERVER_PORT=50062
DATABASE_URL=postgresql://ticket:ticket123@localhost:5442/ticket_service?sslmode=disable
REDIS_HOST=localhost
REDIS_PORT=6389
JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=randlotteryltd
KAFKA_BROKERS=localhost:9092
JAEGER_ENDPOINT=http://localhost:4318
SERVICES_GAME_HOST=localhost
SERVICES_GAME_PORT=50053
SERVICES_DRAW_HOST=localhost
SERVICES_DRAW_PORT=50060
SERVICES_PAYMENT_HOST=localhost
SERVICES_PAYMENT_PORT=50061
SERVICES_WALLET_HOST=localhost
SERVICES_WALLET_PORT=50059
```

#### `services/service-notification/.env`
```env
SERVER_PORT=50063
DATABASE_URL=postgresql://notification:notification123@localhost:5443/notification_service?sslmode=disable
REDIS_URL=redis://localhost:6390
JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=randlotteryltd
KAFKA_BROKERS=localhost:9092
JAEGER_ENDPOINT=http://localhost:4318
SERVICES_ADMIN_MANAGEMENT_HOST=localhost
SERVICES_ADMIN_MANAGEMENT_PORT=50057
```

#### `services/service-player/.env`
```env
SERVER_PORT=50064
DATABASE_URL=postgresql://player:player123@localhost:5444/player_service?sslmode=disable
REDIS_HOST=localhost
REDIS_PORT=6391
SECURITY_JWT_SECRET=your-super-secret-jwt-key-change-in-production
SECURITY_JWT_ISSUER=randlotteryltd
SECURITY_ACCESS_TOKEN_EXPIRY=15m
SECURITY_REFRESH_TOKEN_EXPIRY=168h
KAFKA_BROKERS=localhost:9092
JAEGER_ENDPOINT=http://localhost:4318
```

> See `vinne-microservices/docs/SERVICE_ENVIRONMENT_VARIABLES.md` for the full list of optional variables for each service.

### 6.3 Run Database Migrations

Migrations use [Goose](https://github.com/pressly/goose) and are stored as SQL files in each service's `migrations/` folder.

**Option A — Use the migration script (recommended):**

```bash
# From vinne-microservices/
./scripts/run-migrations.sh
```

**Option B — Run per service with Goose:**

Install Goose first if you don't have it:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Then run migrations for each service:

```bash
# Admin Management
goose -dir services/service-admin-management/migrations \
  postgres "postgresql://admin_mgmt:admin_mgmt123@localhost:5437/admin_management?sslmode=disable" up

# Agent Auth
goose -dir services/service-agent-auth/migrations \
  postgres "postgresql://agent:agent123@localhost:5434/agent_auth?sslmode=disable" up

# Agent Management
goose -dir services/service-agent-management/migrations \
  postgres "postgresql://agent_mgmt:agent_mgmt123@localhost:5435/agent_management?sslmode=disable" up

# Game
goose -dir services/service-game/migrations \
  postgres "postgresql://game:game123@localhost:5441/game_service?sslmode=disable" up

# Draw
goose -dir services/service-draw/migrations \
  postgres "postgresql://draw_service:draw_service123@localhost:5436/draw_service?sslmode=disable" up

# Wallet
goose -dir services/service-wallet/migrations \
  postgres "postgresql://wallet:wallet123@localhost:5438/wallet_service?sslmode=disable" up

# Terminal
goose -dir services/service-terminal/migrations \
  postgres "postgresql://terminal:terminal123@localhost:5439/terminal_service?sslmode=disable" up

# Payment
goose -dir services/service-payment/migrations \
  postgres "postgresql://payment:payment123@localhost:5440/payment_service?sslmode=disable" up

# Ticket
goose -dir services/service-ticket/migrations \
  postgres "postgresql://ticket:ticket123@localhost:5442/ticket_service?sslmode=disable" up

# Notification
goose -dir services/service-notification/migrations \
  postgres "postgresql://notification:notification123@localhost:5443/notification_service?sslmode=disable" up

# Player
goose -dir services/service-player/migrations \
  postgres "postgresql://player:player123@localhost:5444/player_service?sslmode=disable" up
```

**Option C — Run via `go run` (no Goose install needed):**

```bash
cd services/service-admin-management && go run cmd/migrate/main.go up
cd services/service-agent-auth       && go run cmd/migrate/main.go up
cd services/service-agent-management && go run cmd/migrate/main.go up
cd services/service-game             && go run cmd/migrate/main.go up
cd services/service-draw             && go run cmd/migrate/main.go up
cd services/service-wallet           && go run cmd/migrate/main.go up
cd services/service-terminal         && go run cmd/migrate/main.go up
cd services/service-payment          && go run cmd/migrate/main.go up
cd services/service-ticket           && go run cmd/migrate/main.go up
cd services/service-notification     && go run cmd/migrate/main.go up
cd services/service-player           && go run cmd/migrate/main.go up
```

**Verify migrations ran:**

```bash
# Check tables exist in a database (example: admin management)
docker exec -it randco-microservices-service-admin-management-db-1 \
  psql -U admin_mgmt -d admin_management -c "\dt"
```

### 6.4 Start All Services

**Option A — Use the start script (all services in parallel):**

```bash
# From vinne-microservices/
./scripts/start-services.sh
```

**Option B — Use Make:**

```bash
# From vinne-microservices/
make dev
```

**Option C — Start each service manually** (useful for debugging individual services):

Open a separate terminal for each service. Start in this order:

```bash
# Terminal 1 — Admin Management (other services depend on it)
cd vinne-microservices/services/service-admin-management
go run cmd/server/main.go

# Terminal 2 — Agent Auth
cd vinne-microservices/services/service-agent-auth
go run cmd/server/main.go

# Terminal 3 — Agent Management
cd vinne-microservices/services/service-agent-management
go run cmd/server/main.go

# Terminal 4 — Wallet
cd vinne-microservices/services/service-wallet
go run cmd/server/main.go

# Terminal 5 — Terminal
cd vinne-microservices/services/service-terminal
go run cmd/server/main.go

# Terminal 6 — Payment
cd vinne-microservices/services/service-payment
go run cmd/server/main.go

# Terminal 7 — Game
cd vinne-microservices/services/service-game
go run cmd/server/main.go

# Terminal 8 — Draw
cd vinne-microservices/services/service-draw
go run cmd/server/main.go

# Terminal 9 — Ticket
cd vinne-microservices/services/service-ticket
go run cmd/server/main.go

# Terminal 10 — Notification
cd vinne-microservices/services/service-notification
go run cmd/server/main.go

# Terminal 11 — Player
cd vinne-microservices/services/service-player
go run cmd/server/main.go

# Terminal 12 — API Gateway (start last)
cd vinne-microservices/services/api-gateway
go run cmd/server/main.go
```

---

## 7. Admin Web Portal Setup

```bash
cd vinne-admin-web
npm install
```

The `.env.localdev` file is already committed and configured for local development:

```env
VITE_API_URL=http://localhost:4000/api/v1
VITE_APP_URL=http://localhost:6176
VITE_ENVIRONMENT=development
```

Start the dev server:

```bash
npm run dev
```

Access at: **http://localhost:6176**

---

## 8. Player Website Setup

```bash
# From the repo root
npm install
npm run dev
```

Access at: **http://localhost:5173**

The player website connects to the API Gateway at `http://localhost:4000`.

---

## 9. Verify Everything is Running

Once all services are up, confirm with these checks:

```bash
# API Gateway health
curl http://localhost:4000/health

# Admin login (should return a JWT token)
curl -X POST http://localhost:4000/api/v1/admin/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"superadmin@randco.com","password":"Admin@123!"}'

# Player registration
curl -X POST http://localhost:4000/api/v1/players/auth/register \
  -H "Content-Type: application/json" \
  -d '{"phone_number":"233256826832","password":"Test@123","first_name":"John","last_name":"Doe"}'
```

**Access Points Summary:**

| Service | URL |
|---------|-----|
| Player Website | http://localhost:5173 |
| Admin Portal | http://localhost:6176 |
| API Gateway | http://localhost:4000 |
| Jaeger Tracing | http://localhost:16686 |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3001 |
| MinIO Console | http://localhost:9001 |

---

## 10. Default Credentials

### Admin Portal

| Email | Password | Role |
|-------|----------|------|
| `superadmin@randco.com` | `Admin@123!` | Super Admin |
| `surajmohammedbwoy@gmail.com` | `Admin@123!` | Admin |

> Change these immediately in any non-local environment.

### Grafana

| Username | Password |
|----------|----------|
| `admin` | `admin` |

### MinIO

| Username | Password |
|----------|----------|
| `minioadmin` | `minioadmin123` |

### Databases (local dev only)

| Service | Host | Port | User | Password | Database |
|---------|------|------|------|----------|----------|
| Admin Management | localhost | 5437 | admin_mgmt | admin_mgmt123 | admin_management |
| Agent Auth | localhost | 5434 | agent | agent123 | agent_auth |
| Agent Management | localhost | 5435 | agent_mgmt | agent_mgmt123 | agent_management |
| Game | localhost | 5441 | game | game123 | game_service |
| Draw | localhost | 5436 | draw_service | draw_service123 | draw_service |
| Wallet | localhost | 5438 | wallet | wallet123 | wallet_service |
| Terminal | localhost | 5439 | terminal | terminal123 | terminal_service |
| Payment | localhost | 5440 | payment | payment123 | payment_service |
| Ticket | localhost | 5442 | ticket | ticket123 | ticket_service |
| Notification | localhost | 5443 | notification | notification123 | notification_service |
| Player | localhost | 5444 | player | player123 | player_service |

---

## 11. Useful Commands

### Docker / Infrastructure

```bash
# Start all infrastructure
docker compose up -d

# Stop all infrastructure (keeps data)
docker compose down

# Stop and wipe all data volumes (full reset)
docker compose down -v

# View logs for a specific container
docker logs -f randco-microservices-service-game-db-1

# Check all running containers
docker ps
```

### Migrations

```bash
# Run all migrations
./scripts/run-migrations.sh

# Roll back last migration for a service (using goose)
goose -dir services/service-game/migrations \
  postgres "postgresql://game:game123@localhost:5441/game_service?sslmode=disable" down

# Check migration status
goose -dir services/service-game/migrations \
  postgres "postgresql://game:game123@localhost:5441/game_service?sslmode=disable" status
```

### Makefile Shortcuts (from `vinne-microservices/`)

```bash
make dev          # Start all services via docker-compose
make stop         # Stop all services
make clean        # Stop and remove all volumes
make build        # Build all Docker images
make logs         # Tail logs from all services
make proto        # Regenerate Protocol Buffer code
make help         # Show all available commands
```

### Proto Regeneration

If you modify `.proto` files, regenerate the Go code:

```bash
cd vinne-microservices
./scripts/generate-protos.sh
```

### Admin Web

```bash
cd vinne-admin-web
npm run dev           # Start dev server (port 6176)
npm run build         # Production build
npm run lint          # Run ESLint
npm run format        # Run Prettier
npm run type-check    # TypeScript check only
```

### Player Website

```bash
npm run dev           # Start dev server (port 5173)
npm run build         # Production build
npm run lint          # Run ESLint
npm test              # Run tests (single run)
```

---

## 12. Troubleshooting

### Docker containers not starting

```bash
# Check for port conflicts
netstat -ano | findstr :5437   # Windows
lsof -i :5437                  # Mac/Linux

# Restart Docker Desktop and try again
docker compose down && docker compose up -d
```

### Migration fails — "container not found"

The container name includes the folder name as a prefix. If you cloned into a folder other than `vinne-microservices`, the container names will differ. Check actual names with:

```bash
docker ps --format "{{.Names}}" | grep db
```

Then update the container names in `scripts/run-migrations.sh` accordingly.

### Service fails to start — "connection refused" on database

The database container may still be initialising. Wait 10–15 seconds after `docker compose up -d` before starting services. You can also check readiness:

```bash
docker exec -it randco-microservices-service-game-db-1 pg_isready -U game
```

### JWT authentication errors across services

All services must share the same `JWT_SECRET`. Double-check every `.env` file has the same value. The Player service uses `SECURITY_JWT_SECRET` (not `JWT_SECRET`) — make sure that matches too.

### Go module / dependency errors

```bash
cd vinne-microservices/shared
go mod tidy

cd vinne-microservices/services/<service-name>
go mod tidy
go build ./...
```

### Port already in use

```bash
# Windows — find and kill process on a port
netstat -ano | findstr :50057
taskkill /PID <pid> /F

# Mac/Linux
lsof -ti :50057 | xargs kill -9
```

### Admin portal shows blank / API errors

1. Confirm the API Gateway is running on port 4000
2. Check `vinne-admin-web/.env.localdev` has `VITE_API_URL=http://localhost:4000/api/v1`
3. Check browser console for CORS errors — the gateway's `SECURITY_ALLOWED_ORIGINS` must include `http://localhost:6176`

---

## Additional References

- Full environment variable reference: `vinne-microservices/docs/SERVICE_ENVIRONMENT_VARIABLES.md`
- Microservices architecture overview: `vinne-microservices/README.md`
- Detailed microservices setup: `vinne-microservices/SETUP.md`
- Admin portal README: `vinne-admin-web/README.md`
- Proto definitions: `vinne-microservices/proto/`
