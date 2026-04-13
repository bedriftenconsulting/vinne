#!/usr/bin/env node
/**
 * Bulk backfill wallet transaction metadata for ticket-sale debits by pulling
 * ticket details from ticket_service and updating wallet_service in ONE UPDATE.
 *
 * This avoids slow per-row shell loops (and output buffering issues).
 */

import { execFileSync } from 'node:child_process'

const WALLET_DB_URL =
  process.env.WALLET_DB_URL ??
  'postgresql://wallet:wallet123@localhost:5438/wallet_service?sslmode=disable'
const TICKET_DB_URL =
  process.env.TICKET_DB_URL ??
  'postgresql://ticket:ticket123@localhost:5442/ticket_service?sslmode=disable'

const SINCE_MINUTES = Number(process.env.SINCE_MINUTES ?? '60')
const LIMIT = Number(process.env.LIMIT ?? '2000')

function psql(dbUrl, sql) {
  return execFileSync('psql', [dbUrl, '-Atc', sql], { encoding: 'utf8' }).trim()
}

function sqlQuote(str) {
  return `'${String(str).replace(/'/g, "''")}'`
}

function main() {
  const walletSql = `
    SELECT
      transaction_id::text,
      substring((COALESCE(description,'') || ' ' || COALESCE(reference,'')) from '(TKT-[A-Za-z0-9-]+)')::text AS serial
    FROM wallet_transactions
    WHERE transaction_type = 'DEBIT'
      AND status = 'COMPLETED'
      AND (COALESCE(description, '') ILIKE '%Ticket purchase%' OR COALESCE(reference, '') ILIKE '%TKT-%')
      AND NOT (COALESCE(metadata, '{}'::jsonb) ? 'audit_trace')
      AND created_at >= (now() - interval '${SINCE_MINUTES} minutes')
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
    LIMIT ${Number.isFinite(LIMIT) ? LIMIT : 2000};
  `

  const walletRowsRaw = psql(WALLET_DB_URL, walletSql)
  if (!walletRowsRaw) {
    console.log('No wallet transactions matched the bulk backfill filter.')
    return
  }

  const walletRows = walletRowsRaw
    .split('\n')
    .map(line => line.split('|'))
    .filter(parts => parts.length >= 2)
    .map(([transactionId, serial]) => ({ transactionId, serial }))
    .filter(r => r.serial && r.serial.toUpperCase().startsWith('TKT-'))

  const serials = [...new Set(walletRows.map(r => r.serial.toUpperCase()))]
  if (serials.length === 0) {
    console.log('No serials found in matching wallet rows.')
    return
  }

  const serialList = serials.map(sqlQuote).join(',')
  const ticketSql = `
    SELECT
      serial_number::text,
      jsonb_build_object(
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
    WHERE t.serial_number IN (${serialList});
  `

  const ticketRowsRaw = psql(TICKET_DB_URL, ticketSql)
  const ticketMap = new Map()
  if (ticketRowsRaw) {
    for (const line of ticketRowsRaw.split('\n')) {
      const idx = line.indexOf('|')
      if (idx === -1) continue
      const serial = line.slice(0, idx).trim().toUpperCase()
      const meta = line.slice(idx + 1).trim()
      if (serial && meta) ticketMap.set(serial, meta)
    }
  }

  const missing = serials.filter(s => !ticketMap.has(s))
  if (missing.length) {
    console.log(`Tickets missing in ticket_service: ${missing.length}`)
  }

  const pairs = []
  for (const row of walletRows) {
    const meta = ticketMap.get(row.serial.toUpperCase())
    if (!meta) continue
    pairs.push({ transactionId: row.transactionId, meta })
  }

  if (pairs.length === 0) {
    console.log('No wallet rows had matching tickets to backfill.')
    return
  }

  // Use tagged dollar-quoting for JSON to avoid escaping.
  const tag = '$json$'
  const valuesSql = pairs
    .map(p => `(${sqlQuote(p.transactionId)}, ${tag}${p.meta}${tag})`)
    .join(',\n')

  const updateSql = `
    WITH payload(transaction_id, meta) AS (
      VALUES
      ${valuesSql}
    )
    UPDATE wallet_transactions wt
    SET metadata = COALESCE(wt.metadata, '{}'::jsonb) || payload.meta::jsonb
    FROM payload
    WHERE wt.transaction_id = payload.transaction_id;
  `

  execFileSync('psql', [WALLET_DB_URL, '-c', updateSql], { stdio: 'inherit' })
  console.log(`bulk_backfill_updated=${pairs.length}`)
}

main()

