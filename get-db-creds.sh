#!/bin/bash
echo "=== Player DB ==="
sudo docker inspect vinne-microservices_service-player-db_1 | python3 -c "
import json, sys
env = json.load(sys.stdin)[0]['Config']['Env']
for e in env:
    if any(k in e for k in ['POSTGRES', 'DATABASE']):
        print(e)
"

echo ""
echo "=== Player DB port on host ==="
sudo docker port vinne-microservices_service-player-db_1

echo ""
echo "=== All DB ports ==="
sudo docker ps --format '{{.Names}}\t{{.Ports}}' | grep db
