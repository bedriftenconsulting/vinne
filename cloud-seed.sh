#!/bin/bash
set -e

echo "=== Step 1: Clear old test game ==="
sudo docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "
  DELETE FROM game_schedules WHERE game_id IN (SELECT id FROM games WHERE code IN ('CARPARK26','BMWNEWWPRO','IPHONE17'));
  DELETE FROM games WHERE code IN ('CARPARK26','BMWNEWWPRO','IPHONE17');
"

echo ""
echo "=== Step 2: Insert BMW M3 game ==="
sudo docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "
INSERT INTO games (
  id, code, name, type, description, status,
  draw_frequency, draw_time_str,
  base_price, min_stake_amount, max_stake_amount,
  total_tickets, max_tickets_per_player, sales_cutoff_minutes,
  prize_details, rules, game_category, game_format, organizer,
  number_range_min, number_range_max, selection_count,
  start_date, end_date, draw_date,
  multi_draw_enabled, created_at, updated_at
) VALUES (
  'e419ba9d-4565-4a9c-b8f9-0e5041e5d044',
  'BMWNEWWPRO',
  'BMW M3 Competition',
  'raffle',
  'Win a brand new BMW M3 Competition',
  'ACTIVE',
  'special',
  '20:00',
  20.00, 20.00, 20.00,
  1000, 10, 30,
  '[{\"rank\": 1, \"label\": \"1st Prize\", \"description\": \"BMW M3\"}]',
  'One ticket per transaction',
  'private', 'competition', 'winbig_africa',
  1, 100, 1,
  '2026-05-03', '2026-05-03', '2026-05-03',
  false, NOW(), NOW()
);
"

echo ""
echo "=== Step 3: Insert iPhone 17 game ==="
sudo docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "
INSERT INTO games (
  id, code, name, type, description, status,
  draw_frequency, draw_time_str,
  base_price, min_stake_amount, max_stake_amount,
  total_tickets, max_tickets_per_player, sales_cutoff_minutes,
  prize_details, rules, game_category, game_format, organizer,
  number_range_min, number_range_max, selection_count,
  start_date, end_date, draw_date,
  multi_draw_enabled, created_at, updated_at
) VALUES (
  '6d02ec42-d611-44d6-97e7-8dbcd69fd300',
  'IPHONE17',
  'iPhone 17 Pro Max',
  'raffle',
  'Win the latest iPhone 17 Pro Max',
  'ACTIVE',
  'special',
  '20:00',
  20.00, 20.00, 20.00,
  1000, 10, 30,
  '[{\"rank\": 1, \"label\": \"1st Prize\", \"description\": \"iPhone 17 Pro Max\"}]',
  'One ticket per transaction',
  'private', 'competition', 'winbig_africa',
  1, 100, 1,
  '2026-05-03', '2026-05-03', '2026-05-03',
  false, NOW(), NOW()
);
"

echo ""
echo "=== Step 4: Create schedules ==="
sudo docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "
INSERT INTO game_schedules (
  id, game_id, game_code, game_name,
  scheduled_start, scheduled_end, scheduled_draw,
  draw_number, status, is_active, created_at, updated_at
)
SELECT
  gen_random_uuid(),
  g.id, g.code, g.name,
  '2026-04-20 00:00:00'::timestamp,
  '2026-05-03 19:30:00'::timestamp,
  '2026-05-03 20:00:00'::timestamp,
  1, 'SCHEDULED', true, NOW(), NOW()
FROM games g
WHERE g.code IN ('BMWNEWWPRO', 'IPHONE17');
"

echo ""
echo "=== Final state ==="
sudo docker exec vinne-microservices_service-game-db_1 psql -U game -d game_service -c "
SELECT g.code, g.name, g.status, g.draw_date, g.prize_details,
       COUNT(gs.id) as schedules
FROM games g
LEFT JOIN game_schedules gs ON gs.game_id = g.id
GROUP BY g.id, g.code, g.name, g.status, g.draw_date, g.prize_details
ORDER BY g.created_at DESC;
"
