#!/bin/bash
set -e

echo "=== Check current gateway status ==="
sudo docker ps --format '{{.Names}}\t{{.Status}}' | grep gateway || echo "Gateway not running"

echo ""
echo "=== Get gateway image ==="
GW_IMAGE=$(sudo docker inspect vinne-microservices_api-gateway_1 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin)[0]['Config']['Image'])" 2>/dev/null || echo "")
echo "Image: $GW_IMAGE"

echo ""
echo "=== Stop gateway ==="
sudo docker stop vinne-microservices_api-gateway_1 2>/dev/null || true
sudo docker rm vinne-microservices_api-gateway_1 2>/dev/null || true

echo ""
echo "=== Patch docker-compose.yml CDN line ==="
COMPOSE_FILE="/home/Suraj/vinne/vinne-microservices/docker-compose.yml"
# The YAML format uses: STORAGE_CDN_ENDPOINT: http://localhost:9000/...
sudo sed -i 's|STORAGE_CDN_ENDPOINT: http://localhost:9000/vinne-game-assets|STORAGE_CDN_ENDPOINT: http://34.121.254.209:9000/vinne-game-assets|g' "$COMPOSE_FILE"
echo "Patched:"
sudo grep "CDN_ENDPOINT" "$COMPOSE_FILE"

echo ""
echo "=== Start only api-gateway (no recreate of others) ==="
sudo -u Suraj bash -c "cd /home/Suraj/vinne/vinne-microservices && docker-compose up -d --no-deps api-gateway"

echo ""
echo "=== Wait 8s ==="
sleep 8

echo ""
echo "=== CDN in container ==="
sudo docker exec vinne-microservices_api-gateway_1 env | grep CDN

echo ""
echo "=== API health ==="
curl -sk https://api.winbig.bedriften.xyz/health
