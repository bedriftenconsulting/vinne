#!/bin/bash
echo "=== API Gateway storage env ==="
sudo docker inspect vinne-microservices_api-gateway_1 | python3 -c "
import json, sys
data = json.load(sys.stdin)
env = data[0]['Config']['Env']
for e in env:
    if any(k in e for k in ['STORAGE', 'CDN', 'MINIO', 'SPACES', 'BUCKET', 'S3']):
        print(e)
"

echo ""
echo "=== Check if MinIO is running ==="
sudo docker ps --format '{{.Names}}' | grep -i minio || echo "No MinIO container"

echo ""
echo "=== Sample game logo_url from DB ==="
sudo docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "SELECT code, logo_url FROM games WHERE logo_url IS NOT NULL AND logo_url != '' LIMIT 5;"
