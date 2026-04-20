#!/bin/bash
set -e
COMPOSE_DIR="/home/Suraj/vinne/vinne-microservices"

echo "=== Step 1: Start MinIO ==="
sudo docker-compose -f "$COMPOSE_DIR/docker-compose.yml" up -d minio
sleep 8

echo ""
echo "=== Step 2: Create bucket ==="
sudo docker-compose -f "$COMPOSE_DIR/docker-compose.yml" up minio-init
sleep 3

echo ""
echo "=== Step 3: Check MinIO running ==="
sudo docker ps --format '{{.Names}}' | grep -i minio

echo ""
echo "=== Step 4: Restart api-gateway with public CDN endpoint ==="
sudo docker stop vinne-microservices_api-gateway_1 2>/dev/null || true
sudo docker rm vinne-microservices_api-gateway_1 2>/dev/null || true

cd "$COMPOSE_DIR"
sudo STORAGE_CDN_ENDPOINT=http://34.121.254.209:9000/vinne-game-assets docker-compose -f "$COMPOSE_DIR/docker-compose.yml" up -d api-gateway

echo ""
echo "=== Step 5: Verify CDN endpoint in running container ==="
sleep 5
sudo docker exec vinne-microservices_api-gateway_1 env | grep CDN

echo ""
echo "=== Step 6: Open MinIO port 9000 check ==="
curl -sk http://localhost:9000/minio/health/live && echo "MinIO healthy" || echo "MinIO not reachable"
