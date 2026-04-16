# WinBig Africa

**WinBig Africa** is a comprehensive lottery and competition management platform built for the African market. The platform enables users to participate in exciting competitions to win luxury prizes like cars, electronics, and cash.

## Platform Overview

This monorepo contains three main components:

| Component | Location | Tech Stack | Purpose |
|-----------|----------|------------|---------|
| **Microservices Backend** | `vinne-microservices/` | Go 1.24, gRPC, PostgreSQL, Redis, Kafka | Core business logic — 12 services handling games, draws, tickets, payments, wallets, and more |
| **Admin Web Portal** | `vinne-admin-web/` | React 19, TypeScript, Vite, TanStack | Internal dashboard for platform administrators |
| **Player Website** | `/` (root) | React 18, TypeScript, Vite | Public-facing competition site where players buy tickets and check results |

## Key Features

- **Game Management** — Create and configure lottery games with flexible prize structures
- **Draw Engine** — Automated draw execution with provably fair RNG
- **Ticket System** — Secure ticket generation, validation, and tracking
- **Payment Integration** — MTN MoMo, Telecel, AirtelTigo mobile money support
- **Multi-channel Notifications** — SMS (Hubtel), Email (Mailgun), Push notifications
- **Admin Portal** — Full-featured dashboard for game, draw, player, and agent management
- **Player Portal** — Mobile-first competition browsing, ticket purchase, and results

## Tech Stack

### Backend
- **Language:** Go 1.24
- **RPC:** gRPC with Protocol Buffers
- **Databases:** PostgreSQL 17 (one per service)
- **Cache:** Redis 7.4+ (per-service + shared session store)
- **Message Queue:** Apache Kafka 3.9+ (KRaft mode)
- **Tracing:** OpenTelemetry + Jaeger
- **Metrics:** Prometheus + Grafana
- **Storage:** MinIO (S3-compatible)

### Frontend
- **Framework:** React 18/19 with TypeScript
- **Build Tool:** Vite
- **Routing:** TanStack Router
- **Data Fetching:** TanStack Query
- **Styling:** Tailwind CSS
- **UI Components:** Shadcn/ui (Radix UI primitives)
- **Forms:** React Hook Form + Zod validation

## Quick Start

See **[LOCAL_SETUP.md](./LOCAL_SETUP.md)** for complete setup instructions including:
- Prerequisites and system requirements
- Cloning the repository
- Starting Docker infrastructure (databases, Redis, Kafka, Jaeger)
- Running database migrations
- Starting all 12 microservices
- Launching the admin portal and player website
- Default credentials and access points

## Repository Structure

```
vinne/
├── vinne-microservices/          # Go microservices backend
│   ├── services/                 # 12 microservices
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
│   └── docs/                     # Backend documentation
├── vinne-admin-web/              # Admin dashboard (React 19)
│   ├── src/
│   └── package.json
├── src/                          # Player website (React 18)
├── docs/                         # Project documentation
├── LOCAL_SETUP.md                # Complete local dev setup guide
└── README.md                     # This file
```

## Documentation

- **[LOCAL_SETUP.md](./LOCAL_SETUP.md)** — Complete local development setup guide
- **[vinne-microservices/README.md](./vinne-microservices/README.md)** — Backend architecture overview
- **[vinne-microservices/SETUP.md](./vinne-microservices/SETUP.md)** — Detailed microservices setup
- **[vinne-microservices/docs/SERVICE_ENVIRONMENT_VARIABLES.md](./vinne-microservices/docs/SERVICE_ENVIRONMENT_VARIABLES.md)** — Complete environment variable reference
- **[vinne-admin-web/README.md](./vinne-admin-web/README.md)** — Admin portal features and setup

## Services & Ports

| Service | gRPC Port | HTTP/Metrics | Database | Redis | Description |
|---------|-----------|--------------|----------|-------|-------------|
| API Gateway | 4000 (HTTP) | 8080 | — | 6379 | REST API entry point |
| Admin Management | 50057 | 8085 | 5437 | 6384 | Admin auth, RBAC |
| Agent Auth | 50052 | — | 5434 | 6381 | Agent authentication |
| Game | 50053 | 8090 | 5441 | 6388 | Game configuration |
| Draw | 50060 | 8091 | 5436 | 6383 | Draw execution, RNG |
| Wallet | 50059 | 8087 | 5438 | 6385 | Wallet, transactions |
| Terminal | 50054 | 8088 | 5439 | 6386 | POS terminal mgmt |
| Payment | 50061 | 8089 | 5440 | 6387 | Payment gateway |
| Ticket | 50062 | 8092 | 5442 | 6389 | Ticket generation |
| Notification | 50063 | 8093 | 5443 | 6390 | SMS, Email, Push |
| Player | 50064 | 8094 | 5444 | 6391 | Player auth, profile |

**Infrastructure:**
- Kafka: `9092`
- Jaeger UI: `16686`
- Prometheus: `9090`
- Grafana: `3001`
- MinIO: `9000` (API), `9001` (Console)

**Frontend:**
- Player Website: `5173`
- Admin Portal: `6176`

## Default Credentials (Local Dev)

### Admin Portal
- Email: `superadmin@randco.com`
- Password: `Admin@123!`

### Grafana
- Username: `admin`
- Password: `admin`

### MinIO
- Username: `minioadmin`
- Password: `minioadmin123`

> ⚠️ Change all default credentials in non-local environments.

## Development Workflow

```bash
# Start infrastructure (databases, Redis, Kafka, Jaeger)
cd vinne-microservices
docker compose up -d

# Run migrations
./scripts/run-migrations.sh

# Start all microservices
./scripts/start-services.sh

# In separate terminals:
# Start admin portal
cd vinne-admin-web && npm run dev

# Start player website
npm run dev
```

## License

Proprietary — All rights reserved.

## Contact

For questions or support, contact the development team.
