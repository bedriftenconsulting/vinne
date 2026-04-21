#!/bin/bash
set -e
COMPOSE_FILE="/home/Suraj/vinne/vinne-microservices/docker-compose.yml"

echo "=== Add trailing dot variant to CORS origins ==="
sudo sed -i 's|https://winbig.bedriften.xyz,https://admin|https://winbig.bedriften.xyz,https://winbig.bedriften.xyz.,https://admin|g' "$COMPOSE_FILE"

echo "Patched:"
sudo grep "ALLOWED_ORIGINS" "$COMPOSE_FILE" | head -3

echo ""
echo "=== Restart gateway ==="
sudo docker stop vinne-microservices_api-gateway_1 2>/dev/null || true
sudo docker rm vinne-microservices_api-gateway_1 2>/dev/null || true
sudo -u Suraj bash -c "cd /home/Suraj/vinne/vinne-microservices && docker-compose up -d --no-deps api-gateway"
sleep 8

echo ""
echo "=== Test CORS with trailing dot ==="
curl -sk -X OPTIONS "https://api.winbig.bedriften.xyz/api/v1/players/games" \
  -H "Origin: https://winbig.bedriften.xyz." \
  -D - -o /dev/null | grep -i "access-control-allow-origin"

echo ""
echo "=== Test CORS without trailing dot ==="
curl -sk -X OPTIONS "https://api.winbig.bedriften.xyz/api/v1/players/games" \
  -H "Origin: https://winbig.bedriften.xyz" \
  -D - -o /dev/null | grep -i "access-control-allow-origin"
