#!/bin/bash

echo "=== Start admin-management-db if not running ==="
sudo -u Suraj bash -c "cd /home/Suraj/vinne/vinne-microservices && docker-compose up -d --no-deps service-admin-management-db"
sleep 5

echo ""
echo "=== Admin users ==="
sudo docker exec vinne-microservices_service-admin-management-db_1 psql -U admin_mgmt -d admin_management -c "SELECT email, username, role FROM admin_users ORDER BY created_at LIMIT 10;" 2>/dev/null || \
sudo docker exec $(sudo docker ps --format '{{.Names}}' | grep admin-management-db | head -1) psql -U admin_mgmt -d admin_management -c "SELECT email, username, role FROM admin_users ORDER BY created_at LIMIT 10;"

echo ""
echo "=== Check seed-admin in Suraj home ==="
sudo ls /home/Suraj/vinne/ | grep -i seed || echo "no seed files"
sudo cat /home/Suraj/vinne/seed-admin.sh 2>/dev/null || true
