#!/bin/bash
set -e

echo "=== Start MinIO ==="
sudo -u Suraj bash -c "cd /home/Suraj/vinne/vinne-microservices && docker-compose up -d --no-deps minio"
sleep 6

echo ""
echo "=== MinIO status ==="
sudo docker ps --format '{{.Names}}\t{{.Ports}}\t{{.Status}}' | grep minio

echo ""
echo "=== Get MinIO host IP on docker bridge ==="
MINIO_IP=$(sudo docker inspect vinne-microservices_minio_1 | python3 -c "import json,sys; d=json.load(sys.stdin); print(list(d[0]['NetworkSettings']['Networks'].values())[0]['IPAddress'])")
echo "MinIO IP: $MINIO_IP"

echo ""
echo "=== Test MinIO internally ==="
curl -sk http://$MINIO_IP:9000/minio/health/live && echo "MinIO reachable at $MINIO_IP:9000" || echo "MinIO not reachable"

echo ""
echo "=== Update nginx to use MinIO docker IP ==="
sudo sed -i "s|proxy_pass http://127.0.0.1:9000/;|proxy_pass http://$MINIO_IP:9000/;|g" /etc/nginx/sites-enabled/api.winbig.bedriften.xyz
sudo nginx -t && sudo systemctl reload nginx

echo ""
echo "=== Test storage proxy ==="
sleep 2
curl -sk -o /dev/null -w "%{http_code}" https://api.winbig.bedriften.xyz/storage/vinne-game-assets/
echo " (storage proxy)"
