#!/bin/bash
# Auto-start all vinne containers on VM boot
# This runs as root via GCP startup script

# Add swap to prevent OOM during operations
if [ ! -f /swapfile ]; then
    fallocate -l 4G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

# Wait for docker
sleep 5

# Start all containers
cd /root/vinne/vinne-microservices
docker-compose up -d

# Remove stale api-gateway container if it exists with wrong image
docker rm -f vinne-microservices_api-gateway_1 2>/dev/null || true
docker rm -f 3fd9aad42091_vinne-microservices_api-gateway_1 2>/dev/null || true

# Start api-gateway fresh
docker run -d \
  --name vinne-microservices_api-gateway_1 \
  --network vinne-microservices_randco-net \
  --restart unless-stopped \
  -p 4000:4000 \
  -e SERVER_PORT=4000 \
  -e REDIS_HOST=redis-shared \
  -e REDIS_PORT=6379 \
  -e JWT_SECRET=your-super-secret-jwt-key-change-in-production \
  -e SECURITY_JWT_ISSUER=randco-admin-management \
  -e SECURITY_JWT_ISSUER=randlotteryltd \
  -e SECURITY_ALLOWED_ORIGINS="http://localhost:3000,http://localhost:5173,http://localhost:6176,https://winbig.bedriften.xyz,https://admin.winbig.bedriften.xyz,https://api.winbig.bedriften.xyz" \
  -e KAFKA_BROKERS=kafka:29092 \
  -e TRACING_ENABLED=false \
  -e ENVIRONMENT=development \
  -e SERVICES_ADMIN_MANAGEMENT_HOST=service-admin-management \
  -e SERVICES_ADMIN_MANAGEMENT_PORT=51057 \
  -e SERVICES_AGENT_AUTH_HOST=service-agent-auth \
  -e SERVICES_AGENT_AUTH_PORT=51052 \
  -e SERVICES_AGENT_MANAGEMENT_HOST=service-agent-management \
  -e SERVICES_AGENT_MANAGEMENT_PORT=51056 \
  -e SERVICES_WALLET_HOST=service-wallet \
  -e SERVICES_WALLET_PORT=51059 \
  -e SERVICES_GAME_HOST=service-game \
  -e SERVICES_GAME_PORT=51053 \
  -e SERVICES_DRAW_HOST=service-draw \
  -e SERVICES_DRAW_PORT=51060 \
  -e SERVICES_TICKET_HOST=service-ticket \
  -e SERVICES_TICKET_PORT=51062 \
  -e SERVICES_PAYMENT_HOST=service-payment \
  -e SERVICES_PAYMENT_PORT=51061 \
  -e SERVICES_TERMINAL_HOST=service-terminal \
  -e SERVICES_TERMINAL_PORT=51054 \
  -e SERVICES_NOTIFICATION_HOST=service-notification \
  -e SERVICES_NOTIFICATION_PORT=51063 \
  -e SERVICES_PLAYER_HOST=service-player \
  -e SERVICES_PLAYER_PORT=51064 \
  -e STORAGE_PROVIDER=spaces \
  -e STORAGE_ENDPOINT=http://minio:9000 \
  -e STORAGE_REGION=us-east-1 \
  -e STORAGE_BUCKET=vinne-game-assets \
  -e STORAGE_ACCESS_KEY_ID=minioadmin \
  -e STORAGE_SECRET_ACCESS_KEY=minioadmin \
  -e STORAGE_CDN_ENDPOINT=http://34.121.254.209:9000/vinne-game-assets \
  -e STORAGE_FORCE_PATH_STYLE=true \
  vinne-microservices_api-gateway:latest

echo "Startup complete" >> /var/log/vinne-startup.log
