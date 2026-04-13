# ArgoCD Applications

This directory contains ArgoCD application manifests for deploying the Rand Lottery microservices platform.

## Structure

```
argocd/
├── dev/                    # Development environment applications
├── staging/                # Staging environment applications
└── production/             # Production environment applications
```

Each environment directory contains individual ArgoCD application manifests for each microservice:

- `service-api-gateway.yaml` - API Gateway service
- `service-admin-management.yaml` - Admin Management service
- `service-agent-auth.yaml` - Agent Authentication service
- `service-agent-management.yaml` - Agent Management service
- `service-draw.yaml` - Draw service
- `service-game.yaml` - Game service
- `service-payment.yaml` - Payment service
- `service-terminal.yaml` - Terminal service
- `service-wallet.yaml` - Wallet service

## Deployment

### Deploy all services for an environment

```bash
# Development
kubectl apply -f argocd/dev/

# Staging
kubectl apply -f argocd/staging/

# Production
kubectl apply -f argocd/production/
```

### Deploy individual services

```bash
# Deploy only API Gateway to dev
kubectl apply -f argocd/dev/service-api-gateway.yaml

# Deploy only Admin Management to staging
kubectl apply -f argocd/staging/service-admin-management.yaml
```

## Configuration

### Environment-specific settings:

| Environment | Branch | Namespace | Auto-sync | Values File |
|-------------|--------|-----------|-----------|-------------|
| Development | develop | microservices-dev | Yes | values-dev.yaml |
| Staging | staging | microservices-staging | Yes | values-staging.yaml |
| Production | main | microservices-prod | No (Manual) | values-prod.yaml |

### Sync Policies

- **Development & Staging**: Automatic sync with self-healing enabled
- **Production**: Manual sync for controlled deployments

### Global Configuration

All applications share global configuration for:
- Container registry: `ghcr.io/nexalabshq`
- Database: PostgreSQL external instance
- Cache: Redis external instance
- Message queue: Kafka
- Secrets: HashiCorp Vault

## Managing Applications

### View application status

```bash
# List all applications
argocd app list

# Get specific application details
argocd app get service-api-gateway-dev

# View application history
argocd app history service-api-gateway-dev
```

### Sync applications

```bash
# Sync specific application
argocd app sync service-api-gateway-dev

# Sync with prune
argocd app sync service-api-gateway-dev --prune

# Sync all apps for an environment
argocd app sync -l environment=dev
```

### Rollback

```bash
# Rollback to previous version
argocd app rollback service-api-gateway-dev

# Rollback to specific revision
argocd app rollback service-api-gateway-dev 2
```

## Regenerating Applications

To regenerate all ArgoCD applications:

```bash
./scripts/generate-argocd-apps.sh
```

This will recreate all application manifests based on the current configuration.

## Notes

- Each service can be deployed, scaled, and managed independently
- Services in the same environment share the same namespace
- Production deployments require manual approval
- All services use Helm charts located in `helm/microservices/charts/`