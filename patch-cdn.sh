#!/bin/bash
set -e

COMPOSE_FILE="/home/Suraj/vinne/vinne-microservices/docker-compose.yml"

echo "=== Patching CDN endpoint in docker-compose.yml ==="
sudo sed -i 's|STORAGE_CDN_ENDPOINT=http://localhost:9000/vinne-game-assets|STORAGE_CDN_ENDPOINT=http://34.121.254.209:9000/vinne-game-assets|g' "$COMPOSE_FILE"

echo "=== Verifying patch ==="
sudo grep "CDN_ENDPOINT" "$COMPOSE_FILE"

echo ""
echo "=== Restarting api-gateway ==="
sudo -u Suraj bash -c "cd /home/Suraj/vinne/vinne-microservices && docker-compose up -d --force-recreate api-gateway"

echo ""
echo "=== Waiting 8s ==="
sleep 8

echo ""
echo "=== CDN endpoint in running container ==="
sudo docker exec vinne-microservices_api-gateway_1 env | grep CDN

echo ""
echo "=== API health ==="
curl -sk https://api.winbig.bedriften.xyz/health
