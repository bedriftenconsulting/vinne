#!/usr/bin/env bash
set -euo pipefail

# Backfill tickets.draw_date/draw_time from game_service.game_schedules.scheduled_draw
# using tickets.game_schedule_id.
#
# This fixes Draw Time showing as <nil>/Not available in admin UI for tickets that
# were issued without persisting the schedule's draw timestamp.

TICKET_DB_URL="${TICKET_DB_URL:-postgresql://ticket:ticket123@localhost:5442/ticket_service?sslmode=disable}"
GAME_DB_URL="${GAME_DB_URL:-postgresql://game:game123@localhost:5441/game_service?sslmode=disable}"

LIMIT="${LIMIT:-5000}"
DRY_RUN="${DRY_RUN:-0}"

echo "ticket_db: $TICKET_DB_URL"
echo "game_db:   $GAME_DB_URL"
echo "limit:     $LIMIT"
echo "dry_run:   $DRY_RUN"
echo

ticket_query="
  SELECT
    id::text,
    serial_number::text,
    game_schedule_id::text
  FROM tickets
  WHERE game_schedule_id IS NOT NULL
    AND (draw_date IS NULL OR draw_time IS NULL)
  ORDER BY created_at DESC
  LIMIT ${LIMIT};
"

rows="$(psql "$TICKET_DB_URL" -Atc "$ticket_query" || true)"
if [[ -z "${rows//[[:space:]]/}" ]]; then
  echo "No tickets matched the draw-time backfill filter."
  exit 0
fi

updated=0
missing_schedule=0

while IFS='|' read -r ticket_id serial schedule_id; do
  sched_q="
    SELECT scheduled_draw::text
    FROM game_schedules
    WHERE id='${schedule_id}'
    LIMIT 1;
  "
  scheduled_draw="$(psql "$GAME_DB_URL" -Atc "$sched_q" || true)"

  if [[ -z "${scheduled_draw//[[:space:]]/}" ]]; then
    echo "MISS  ticket=$serial schedule=$schedule_id (schedule not found)"
    missing_schedule=$((missing_schedule + 1))
    continue
  fi

  if [[ "$DRY_RUN" == "1" ]]; then
    echo "DRY   ticket=$serial draw=$scheduled_draw"
    updated=$((updated + 1))
    continue
  fi

  upd="
    UPDATE tickets
    SET
      draw_date = '${scheduled_draw}'::timestamp,
      draw_time = '${scheduled_draw}'::timestamp
    WHERE id='${ticket_id}';
  "
  psql "$TICKET_DB_URL" -c "$upd" >/dev/null
  echo "OK    ticket=$serial draw=$scheduled_draw"
  updated=$((updated + 1))
done <<<"$rows"

echo
echo "summary:"
echo "updated=$updated"
echo "missing_schedule=$missing_schedule"

