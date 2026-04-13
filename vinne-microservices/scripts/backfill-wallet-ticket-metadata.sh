#!/usr/bin/env bash
set -euo pipefail

# Backfill wallet transaction metadata for ticket-sale debits.
# This is needed because player-wallet debit transactions are currently created
# with empty metadata in service-wallet. The admin UI uses metadata to render
# staked numbers, bet type, lines, per-line stake, game and draw time.

WALLET_DB_URL="${WALLET_DB_URL:-postgresql://wallet:wallet123@localhost:5438/wallet_service?sslmode=disable}"
TICKET_DB_URL="${TICKET_DB_URL:-postgresql://ticket:ticket123@localhost:5442/ticket_service?sslmode=disable}"

LIMIT="${LIMIT:-5000}"
DRY_RUN="${DRY_RUN:-0}"
QUIET="${QUIET:-0}"

echo "wallet_db:  $WALLET_DB_URL"
echo "ticket_db:  $TICKET_DB_URL"
echo "limit:      $LIMIT"
echo "dry_run:    $DRY_RUN"
echo "quiet:      $QUIET"
echo

wallet_query="
  SELECT
    id::text,
    transaction_id::text,
    COALESCE(reference, '')::text,
    COALESCE(description, '')::text,
    substring((COALESCE(description, '') || ' ' || COALESCE(reference, '')) from '(TKT-[A-Za-z0-9-]+)')::text AS serial
  FROM wallet_transactions
  WHERE transaction_type = 'DEBIT'
    AND status = 'COMPLETED'
    AND (COALESCE(description, '') ILIKE '%Ticket purchase%' OR COALESCE(reference, '') ILIKE '%TKT-%')
    AND NOT (COALESCE(metadata, '{}'::jsonb) ? 'audit_trace')
    AND (
      metadata IS NULL
      OR metadata = '{}'::jsonb
      OR NOT (metadata ? 'ticket')
      OR NOT (metadata ? 'stake_numbers')
      OR NOT (metadata ? 'bet_type')
      OR NOT (metadata ? 'number_of_lines')
      OR NOT (metadata ? 'unit_price')
      OR NOT (metadata ? 'game_name')
      OR NOT (metadata ? 'draw_datetime')
      OR COALESCE(metadata->>'draw_datetime', '') IN ('', 'null', '<nil>')
    )
  ORDER BY created_at DESC
  LIMIT ${LIMIT};
"

rows="$(psql "$WALLET_DB_URL" -Atc "$wallet_query" || true)"
if [[ -z "${rows//[[:space:]]/}" ]]; then
  echo "No wallet transactions matched the backfill filter."
  exit 0
fi

updated=0
skipped=0
missing_ticket=0

log_line() {
  if [[ "$QUIET" != "1" ]]; then
    echo "$@"
  fi
}

while IFS='|' read -r id txn_id reference description serial; do
  if [[ -z "${serial//[[:space:]]/}" ]]; then
    log_line "SKIP  id=$id txn_id=$txn_id (no TKT- serial found)"
    skipped=$((skipped + 1))
    continue
  fi

  ticket_metadata_query="
    SELECT jsonb_build_object(
      'ticket', to_jsonb(t),
      'stake_numbers', t.selected_numbers,
      'bet_lines', t.bet_lines,
      'bet_type', COALESCE(t.bet_lines->0->>'bet_type', ''),
      'number_of_lines', t.number_of_lines,
      'unit_price', t.unit_price,
      'total_amount', t.total_amount,
      'game_name', t.game_name,
      'game_code', t.game_code,
      'draw_datetime', t.draw_date
    )::text
    FROM tickets t
    WHERE t.serial_number = '${serial}'
    LIMIT 1;
  "

  meta="$(psql "$TICKET_DB_URL" -Atc "$ticket_metadata_query" || true)"
  if [[ -z "${meta//[[:space:]]/}" ]]; then
    log_line "MISS  id=$id txn_id=$txn_id serial=$serial (ticket not found)"
    missing_ticket=$((missing_ticket + 1))
    continue
  fi

  if [[ "$DRY_RUN" == "1" ]]; then
    log_line "DRY   id=$id txn_id=$txn_id serial=$serial"
    updated=$((updated + 1))
    continue
  fi

  # Escape single-quotes for SQL string literal.
  meta_escaped="${meta//\'/\'\'}"

  update_sql="
    UPDATE wallet_transactions
    SET metadata = COALESCE(metadata, '{}'::jsonb) || '${meta_escaped}'::jsonb
    WHERE id = '${id}';
  "
  psql "$WALLET_DB_URL" -c "$update_sql" >/dev/null

  log_line "OK    id=$id txn_id=$txn_id serial=$serial"
  updated=$((updated + 1))
done <<<"$rows"

echo
echo "summary:"
echo "updated=$updated"
echo "skipped_no_serial=$skipped"
echo "missing_ticket=$missing_ticket"
