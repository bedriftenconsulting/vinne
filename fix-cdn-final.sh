#!/bin/bash
set -e

COMPOSE_FILE="/home/Suraj/vinne/vinne-microservices/docker-compose.yml"

echo "=== Update CDN to use HTTPS nginx proxy ==="
sudo sed -i 's|STORAGE_CDN_ENDPOINT: http://34.121.254.209:9000/vinne-game-assets|STORAGE_CDN_ENDPOINT: https://api.winbig.bedriften.xyz/storage/vinne-game-assets|g' "$COMPOSE_FILE"
sudo sed -i 's|STORAGE_CDN_ENDPOINT: http://localhost:9000/vinne-game-assets|STORAGE_CDN_ENDPOINT: https://api.winbig.bedriften.xyz/storage/vinne-game-assets|g' "$COMPOSE_FILE"

echo "Patched CDN:"
sudo grep "CDN_ENDPOINT" "$COMPOSE_FILE"

echo ""
echo "=== Restart gateway only ==="
sudo docker stop vinne-microservices_api-gateway_1 2>/dev/null || true
sudo docker rm vinne-microservices_api-gateway_1 2>/dev/null || true
sudo -u Suraj bash -c "cd /home/Suraj/vinne/vinne-microservices && docker-compose up -d --no-deps api-gateway"

sleep 8

echo ""
echo "=== CDN in container ==="
sudo docker exec vinne-microservices_api-gateway_1 env | grep CDN

echo ""
echo "=== Test storage proxy ==="
curl -sk -o /dev/null -w "%{http_code}" https://api.winbig.bedriften.xyz/storage/vinne-game-assets/
echo " (storage proxy response)"

echo ""
echo "=== API health ==="
curl -sk https://api.winbig.bedriften.xyz/health
