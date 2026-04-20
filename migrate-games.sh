#!/bin/bash
set -e

echo "=== Cloud games ==="
docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "SELECT id, code, name, status, draw_frequency, draw_date, prize_details::text FROM games ORDER BY created_at DESC;"

echo ""
echo "=== Cloud game_schedules count ==="
docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "SELECT COUNT(*) FROM game_schedules;"

echo ""
echo "=== Cloud schema check (draw_date column) ==="
docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "\d games" | grep -E "(draw_date|start_date|end_date|prize)"
