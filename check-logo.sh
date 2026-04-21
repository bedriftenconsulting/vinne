#!/bin/bash
echo "=== DB logo_url ==="
sudo docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "SELECT code, logo_url FROM games ORDER BY updated_at DESC;"

echo ""
echo "=== MinIO files ==="
MINIO=$(sudo docker ps --format '{{.Names}}' | grep minio | grep -v init | head -1)
sudo docker exec $MINIO mc ls local/vinne-game-assets/games/ --recursive 2>/dev/null || echo "mc not available"

echo ""
echo "=== Test image URLs ==="
curl -sk -o /dev/null -w "IPHONE17: %{http_code}\n" "https://api.winbig.bedriften.xyz/storage/vinne-game-assets/games/6d02ec42-d611-44d6-97e7-8dbcd69fd300/logo.jpg"
curl -sk -o /dev/null -w "BMW: %{http_code}\n" "https://api.winbig.bedriften.xyz/storage/vinne-game-assets/games/e419ba9d-4565-4a9c-b8f9-0e5041e5d044/logo.jpg"
