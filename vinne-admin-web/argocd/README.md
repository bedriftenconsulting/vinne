# ArgoCD Application Manifests

This directory contains ArgoCD Application manifests for deploying the RandLottery Admin Web application.

## Files

- `admin-web-dev.yaml` - Development environment application
- `admin-web-staging.yaml` - Staging environment application
- `admin-web-production.yaml` - Production environment application
- `admin-web-appset.yaml` - ApplicationSet for managing all environments

## Prerequisites

1. ArgoCD installed in your Kubernetes cluster
2. GitHub repository connected to ArgoCD
3. Docker images available in GitHub Container Registry (ghcr.io/nexalabshq/rand-admin-web)
4. Kubernetes namespaces will be created automatically

## Deployment

### Option 1: Deploy Individual Applications

Deploy to specific environment:

```bash
# Development
kubectl apply -f admin-web-dev.yaml

# Staging
kubectl apply -f admin-web-staging.yaml

# Production (manual sync)
kubectl apply -f admin-web-production.yaml
```

### Option 2: Deploy Using ApplicationSet (Recommended)

Deploy all environments at once:

```bash
kubectl apply -f admin-web-appset.yaml
```

This will create three applications:

- `admin-web-dev` (auto-sync enabled)
- `admin-web-staging` (auto-sync enabled)
- `admin-web-production` (manual sync)

## Configuration

### Environment Mapping

| Environment | Branch  | Namespace              | Auto-Sync | URL                                   |
| ----------- | ------- | ---------------------- | --------- | ------------------------------------- |
| Development | develop | randlottery-dev        | ✅        | https://admin.dev.randlottery.com     |
| Staging     | staging | randlottery-staging    | ✅        | https://admin.staging.randlottery.com |
| Production  | main    | randlottery-production | ❌        | https://admin.randlottery.com         |

### Sync Policies

- **Development & Staging**: Auto-sync enabled with self-healing
- **Production**: Manual sync required for safety

### Image Tags

The applications use branch-based image tags:

- Development: `ghcr.io/nexalabshq/rand-admin-web:develop`
- Staging: `ghcr.io/nexalabshq/rand-admin-web:staging`
- Production: `ghcr.io/nexalabshq/rand-admin-web:main`

## Managing Applications

### Check Application Status

```bash
# List all applications
argocd app list

# Get specific app details
argocd app get admin-web-dev
```

### Manual Sync (Production)

```bash
# Sync production application
argocd app sync admin-web-production

# Or use kubectl
kubectl patch application admin-web-production -n argocd --type merge -p '{"operation": {"initiatedBy": {"username": "admin"}, "sync": {}}}'
```

### Update Image Tag

```bash
# Update to specific image tag
argocd app set admin-web-dev -p image.tag=develop-abc123

# Sync after update
argocd app sync admin-web-dev
```

### Rollback

```bash
# View history
argocd app history admin-web-dev

# Rollback to previous version
argocd app rollback admin-web-dev 2
```

## Monitoring

### ArgoCD UI

Access the ArgoCD UI to monitor application status:

1. Port-forward: `kubectl port-forward svc/argocd-server -n argocd 8080:443`
2. Open: https://localhost:8080
3. Login with admin credentials

### CLI Commands

```bash
# Watch application status
argocd app get admin-web-dev --refresh

# Check sync status
argocd app sync-status admin-web-dev

# View application logs
argocd app logs admin-web-dev
```

## Troubleshooting

### Application Not Syncing

1. Check if repository is accessible:

   ```bash
   argocd repo list
   ```

2. Verify image exists:

   ```bash
   docker pull ghcr.io/nexalabshq/rand-admin-web:develop
   ```

3. Check application events:
   ```bash
   kubectl describe application admin-web-dev -n argocd
   ```

### Image Pull Errors

1. Ensure GitHub Container Registry secret exists:

   ```bash
   kubectl get secret -n randlottery-dev
   ```

2. Create image pull secret if needed:
   ```bash
   kubectl create secret docker-registry ghcr-secret \
     --docker-server=ghcr.io \
     --docker-username=<github-username> \
     --docker-password=<github-token> \
     -n randlottery-dev
   ```

### Sync Failures

1. Check application logs:

   ```bash
   argocd app logs admin-web-dev --kind=Job
   ```

2. Manual refresh:
   ```bash
   argocd app get admin-web-dev --hard-refresh
   ```

## Security Notes

- Production environment requires manual approval for syncs
- All environments use separate namespaces for isolation
- Secrets should be managed via Kubernetes Secrets or External Secrets Operator
- Image pull secrets required for private GitHub Container Registry
