#!/bin/bash
set -e

echo "=== Stopping old api-gateway ==="
sudo docker stop vinne-microservices_api-gateway_1
sudo docker rm vinne-microservices_api-gateway_1

echo ""
echo "=== Getting current gateway config ==="
# Get the image name
IMAGE=$(sudo docker inspect vinne-microservices_api-gateway_1 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin)[0]['Config']['Image'])" 2>/dev/null || echo "")

# Use docker-compose from the project dir with sudo -u to run as Suraj
echo "=== Restarting via docker-compose ==="
sudo -u Suraj bash -c "cd /home/Suraj/vinne/vinne-microservices && STORAGE_CDN_ENDPOINT=http://34.121.254.209:9000/vinne-game-assets docker-compose up -d api-gateway"

echo ""
echo "=== Verifying CDN endpoint ==="
sleep 5
sudo docker exec vinne-microservices_api-gateway_1 env | grep CDN || echo "Container not ready yet"

echo ""
echo "=== MinIO health ==="
curl -sk http://localhost:9000/minio/health/live && echo "MinIO OK" || echo "MinIO not reachable on localhost"

echo ""
echo "=== Test image upload endpoint ==="
curl -sk https://api.winbig.bedriften.xyz/health
