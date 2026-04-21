#!/bin/bash
set -e
COMPOSE_DIR="/home/Suraj/vinne/vinne-microservices"

echo "=== Pull latest code ==="
sudo -u Suraj bash -c "cd $COMPOSE_DIR && git pull origin main"

echo ""
echo "=== Rebuild api-gateway ==="
sudo -u Suraj bash -c "cd $COMPOSE_DIR && docker-compose build api-gateway"

echo ""
echo "=== Restart api-gateway ==="
sudo docker stop vinne-microservices_api-gateway_1 2>/dev/null || true
sudo docker rm vinne-microservices_api-gateway_1 2>/dev/null || true
sudo -u Suraj bash -c "cd $COMPOSE_DIR && docker-compose up -d --no-deps api-gateway"

echo ""
echo "=== Wait 10s ==="
sleep 10

echo ""
echo "=== Health check ==="
curl -sk https://api.winbig.bedriften.xyz/health

echo ""
echo "=== Test tickets_sold in schedule ==="
curl -sk "https://api.winbig.bedriften.xyz/api/v1/players/games/6d02ec42-d611-44d6-97e7-8dbcd69fd300/schedule" | python3 -c "
import json, sys
d = json.load(sys.stdin)
schedules = d.get('data', {}).get('schedules', [])
for s in schedules:
    print('tickets_sold:', s.get('tickets_sold', 'MISSING'))
"
