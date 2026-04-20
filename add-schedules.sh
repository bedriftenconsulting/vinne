#!/bin/bash
sudo docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "
INSERT INTO game_schedules (
  id, game_id, game_name, frequency,
  scheduled_start, scheduled_end, scheduled_draw,
  status, is_active, created_at, updated_at
)
SELECT
  gen_random_uuid(), g.id, g.name, g.draw_frequency,
  '2026-04-20 00:00:00'::timestamp,
  '2026-05-03 19:30:00'::timestamp,
  '2026-05-03 20:00:00'::timestamp,
  'SCHEDULED', true, NOW(), NOW()
FROM games g
WHERE g.code IN ('BMWNEWWPRO', 'IPHONE17');
"

echo "=== Final check ==="
sudo docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "
SELECT g.code, g.name, g.status, g.draw_date, g.prize_details,
       COUNT(gs.id) as schedules
FROM games g
LEFT JOIN game_schedules gs ON gs.game_id = g.id
GROUP BY g.id, g.code, g.name, g.status, g.draw_date, g.prize_details
ORDER BY g.created_at DESC;
"
