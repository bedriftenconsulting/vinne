#!/bin/bash
set -e

echo "=== Restarting api-gateway and payment with correct CDN ==="
sudo -u Suraj bash -c "
  cd /home/Suraj/vinne/vinne-microservices
  STORAGE_CDN_ENDPOINT=http://34.121.254.209:9000/vinne-game-assets docker-compose up -d api-gateway service-payment
"

echo ""
echo "=== Waiting 10s for containers to start ==="
sleep 10

echo ""
echo "=== Running containers ==="
sudo docker ps --format '{{.Names}}\t{{.Status}}' | grep -E 'gateway|payment'

echo ""
echo "=== CDN endpoint in gateway ==="
sudo docker exec vinne-microservices_api-gateway_1 env 2>/dev/null | grep CDN || echo "Container not found"

echo ""
echo "=== API health ==="
curl -sk https://api.winbig.bedriften.xyz/health
