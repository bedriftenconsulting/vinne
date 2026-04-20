#!/bin/bash
set -e

IPHONE_ID="6d02ec42-d611-44d6-97e7-8dbcd69fd300"
BMW_ID="e419ba9d-4565-4a9c-b8f9-0e5041e5d044"
BUCKET="vinne-game-assets"
MINIO_URL="http://127.0.0.1:9000"
CDN_BASE="https://api.winbig.bedriften.xyz/storage/vinne-game-assets"

echo "=== Upload images to MinIO via mc ==="
# Use MinIO mc client inside the minio container
MINIO_CONTAINER=$(sudo docker ps --format '{{.Names}}' | grep minio | grep -v init | head -1)
echo "MinIO container: $MINIO_CONTAINER"

# Copy images into the container
sudo docker cp /tmp/iphone17-logo.jpg $MINIO_CONTAINER:/tmp/iphone17-logo.jpg
sudo docker cp /tmp/bmw-logo.jpg $MINIO_CONTAINER:/tmp/bmw-logo.jpg

# Use mc to upload
sudo docker exec $MINIO_CONTAINER sh -c "
  mc alias set local http://localhost:9000 minioadmin minioadmin 2>/dev/null || true
  mc mb --ignore-existing local/$BUCKET
  mc anonymous set public local/$BUCKET
  mc cp /tmp/iphone17-logo.jpg local/$BUCKET/games/$IPHONE_ID/logo.jpg
  mc cp /tmp/bmw-logo.jpg local/$BUCKET/games/$BMW_ID/logo.jpg
  echo 'Upload done'
  mc ls local/$BUCKET/games/
"

echo ""
echo "=== Update logo_url in DB ==="
sudo docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "
UPDATE games SET logo_url = '$CDN_BASE/games/$IPHONE_ID/logo.jpg' WHERE code = 'IPHONE17';
UPDATE games SET logo_url = '$CDN_BASE/games/$BMW_ID/logo.jpg' WHERE code = 'BMWNEWWPRO';
SELECT code, logo_url FROM games;
"

echo ""
echo "=== Test image URLs ==="
curl -sk -o /dev/null -w "iPhone logo: %{http_code}\n" "$CDN_BASE/games/$IPHONE_ID/logo.jpg"
curl -sk -o /dev/null -w "BMW logo: %{http_code}\n" "$CDN_BASE/games/$BMW_ID/logo.jpg"
