# Environment Configurations

## Overview

RANDCO Microservices supports 4 distinct environments with different purposes and configurations.

## Environments

| Environment | Purpose | Infrastructure | Access | Auto-Deploy |
|------------|---------|---------------|--------|-------------|
| **Local** | Development on engineer's laptop | Docker Compose | Localhost only | No |
| **Dev** | Integration testing | Kubernetes (small) | VPN/Internal | On commit to `develop` |
| **Staging** | Pre-production testing | Kubernetes (medium) | VPN/Internal | On commit to `main` |
| **Production** | Live traffic | Kubernetes (full) | Public | On release tag |

## Environment Characteristics

### 🖥️ Local (Engineer's Laptop)
- **Purpose**: Individual development and testing
- **Infrastructure**: Docker Compose
- **Database**: PostgreSQL in Docker
- **Cache**: Redis in Docker
- **Message Queue**: Kafka in Docker (single node)
- **Access**: http://localhost:4000
- **Data**: Local test data, resets on `make clean`
- **Scaling**: Manual (1 instance per service)
- **Deployment**: `make dev` or `docker-compose up`

### 🔧 Dev Environment
- **Purpose**: Integration testing, daily builds
- **Infrastructure**: Kubernetes cluster (small)
- **Database**: Shared PostgreSQL instance
- **Cache**: Shared Redis instance
- **Message Queue**: Kafka (3 nodes)
- **Access**: https://dev.randco.internal
- **Data**: Test data, refreshed daily
- **Scaling**: Fixed (2 replicas per service)
- **Deployment**: Automatic on push to `develop` branch

### 🧪 Staging Environment
- **Purpose**: UAT, pre-production validation
- **Infrastructure**: Kubernetes cluster (medium)
- **Database**: Dedicated PostgreSQL instances
- **Cache**: Dedicated Redis instances
- **Message Queue**: Kafka cluster (3 nodes)
- **Access**: https://staging.randco.com.gh
- **Data**: Production-like data (anonymized)
- **Scaling**: Auto (2-10 replicas)
- **Deployment**: Automatic on push to `main` branch

### 🚀 Production Environment
- **Purpose**: Live customer traffic
- **Infrastructure**: Kubernetes cluster (full HA)
- **Database**: HA PostgreSQL with read replicas
- **Cache**: Redis Cluster
- **Message Queue**: Kafka cluster (5 nodes)
- **Access**: https://api.randco.com.gh
- **Data**: Live production data
- **Scaling**: Auto (3-50 replicas based on load)
- **Deployment**: Manual approval on release tags

## Resource Allocation

| Service | Local | Dev | Staging | Production |
|---------|-------|-----|---------|------------|
| **service-admin-auth** | 1 pod | 2 pods | 2-5 pods | 3-10 pods |
| **service-agent-auth** | 1 pod | 2 pods | 2-8 pods | 3-15 pods |
| **service-player-auth** | 1 pod | 2 pods | 3-15 pods | 5-50 pods |
| **service-game** | 1 pod | 2 pods | 2-10 pods | 3-20 pods |
| **service-ticket** | 1 pod | 2 pods | 3-20 pods | 5-100 pods |
| **service-payment** | 1 pod | 2 pods | 3-15 pods | 5-30 pods |

## Database Configuration

| Environment | Setup | Backup | Data |
|------------|-------|--------|------|
| **Local** | Single PostgreSQL | None | Sample data |
| **Dev** | Shared PostgreSQL | Daily | Test data |
| **Staging** | Dedicated instances | Daily | Anonymized prod data |
| **Production** | HA with replicas | Continuous | Live data |

## Access URLs

### Local
- GraphQL Gateway: http://localhost:4000
- Jaeger UI: http://localhost:16686
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3001

### Dev
- GraphQL Gateway: https://dev-api.randco.internal
- Jaeger UI: https://dev-jaeger.randco.internal
- Prometheus: https://dev-prometheus.randco.internal
- Grafana: https://dev-grafana.randco.internal

### Staging
- GraphQL Gateway: https://staging-api.randco.com.gh
- Jaeger UI: https://staging-jaeger.randco.com.gh
- Prometheus: https://staging-prometheus.randco.com.gh
- Grafana: https://staging-grafana.randco.com.gh

### Production
- GraphQL Gateway: https://api.randco.com.gh
- Jaeger UI: https://jaeger.randco.com.gh (restricted)
- Prometheus: https://prometheus.randco.com.gh (restricted)
- Grafana: https://grafana.randco.com.gh (restricted)