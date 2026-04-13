#!/usr/bin/env node
/**
 * Seeds tickets through the same API the spiel-website uses:
 *   POST /api/v1/players/{playerId}/tickets
 *
 * This creates real tickets + wallet debit transactions that show up in the back office.
 *
 * Usage (recommended):
 *   PLAYER_PASSWORD='...' node scripts/seed-tickets-via-website.mjs
 *
 * Or provide an existing token:
 *   PLAYER_JWT='...' node scripts/seed-tickets-via-website.mjs
 */

import crypto from 'node:crypto'

const env = process.env

const API_BASE = env.API_BASE ?? 'http://localhost:4000/api/v1'

// Player identity (defaults are for Jeffery's local env we've been working with).
const PLAYER_PHONE = env.PLAYER_PHONE ?? '233243552239'
const PLAYER_PASSWORD = env.PLAYER_PASSWORD ?? ''
const PLAYER_JWT = env.PLAYER_JWT ?? ''
const PLAYER_ID = env.PLAYER_ID ?? '337ede81-5eee-49c7-952d-420b19ec827a'

// Game/schedule (Game ID prefix 96caa432 -> THROWBACK THURSDAY / THUTHB)
const GAME_ID = env.GAME_ID ?? '96caa432-be74-480f-a2e0-013d51eccf75'
const GAME_CODE = env.GAME_CODE ?? 'THUTHB'
const GAME_SCHEDULE_ID = env.GAME_SCHEDULE_ID ?? 'e7467888-1048-4891-82ae-f3dabd6ee434' // 2026-04-03 16:00
const DRAW_NUMBER = Number(env.DRAW_NUMBER ?? '1')

const COUNT = Number(env.COUNT ?? '100')
const CONCURRENCY = Math.max(1, Number(env.CONCURRENCY ?? '1'))
const DELAY_MS = Math.max(0, Number(env.DELAY_MS ?? '0'))

const INCLUDE_PAIR_TICKETS = Math.max(0, Number(env.INCLUDE_PAIR_TICKETS ?? '3'))
const PAIR_A = Number(env.PAIR_A ?? '15')
const PAIR_B = Number(env.PAIR_B ?? '22')

const BET_TYPES = (env.BET_TYPES
  ? env.BET_TYPES.split(',').map(s => s.trim()).filter(Boolean)
  : ['DIRECT-1', 'DIRECT-2', 'DIRECT-3', 'PERM-2', 'PERM-3', 'BANKER-AGAINST'])

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

function randInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min
}

function sample(arr) {
  return arr[randInt(0, arr.length - 1)]
}

function pickUniqueNumbers(count, { min = 1, max = 90, exclude = new Set() } = {}) {
  const out = []
  const seen = new Set(exclude)
  while (out.length < count) {
    const n = randInt(min, max)
    if (seen.has(n)) continue
    seen.add(n)
    out.push(n)
  }
  return out
}

function nChooseK(n, k) {
  if (k < 0 || k > n) return 0
  if (k === 0 || k === n) return 1
  let kk = Math.min(k, n - k)
  let num = 1
  let den = 1
  for (let i = 1; i <= kk; i++) {
    num *= (n - (kk - i))
    den *= i
  }
  return Math.round(num / den)
}

function buildBetLine(betType, { forcePair = false } = {}) {
  // Amount per combination in pesewas (GHS 1 - 10 by default).
  const amountPerCombination = randInt(100, 1000)

  if (betType === 'DIRECT-1' || betType === 'DIRECT-2' || betType === 'DIRECT-3') {
    const needed = Number(betType.split('-')[1])
    let selected = pickUniqueNumbers(needed)
    if (forcePair) {
      // Put 15 & 22 into the line if it fits; otherwise switch to PERM-2.
      if (needed >= 2) {
        const extraNeeded = needed - 2
        selected = [PAIR_A, PAIR_B, ...pickUniqueNumbers(extraNeeded, { exclude: new Set([PAIR_A, PAIR_B]) })]
      } else {
        // DIRECT-1 can't contain both numbers; caller should avoid forcing this bet type.
        selected = pickUniqueNumbers(needed)
      }
    }

    return {
      bet_type: betType,
      selected_numbers: selected,
      number_of_combinations: undefined,
      amount_per_combination: undefined,
      total_amount: amountPerCombination,
    }
  }

  if (betType === 'PERM-2' || betType === 'PERM-3') {
    const permSize = Number(betType.split('-')[1])
    // Pick a little more than required so combinations exist.
    const n = randInt(permSize + 1, permSize + 3)
    let selected = pickUniqueNumbers(n)
    if (forcePair) {
      // Ensure both are present; keep unique.
      const rest = pickUniqueNumbers(Math.max(0, n - 2), { exclude: new Set([PAIR_A, PAIR_B]) })
      selected = [PAIR_A, PAIR_B, ...rest]
    }
    const combos = nChooseK(selected.length, permSize)

    return {
      bet_type: betType,
      selected_numbers: selected,
      number_of_combinations: combos,
      amount_per_combination: amountPerCombination,
      total_amount: amountPerCombination * combos,
    }
  }

  if (betType === 'BANKER-AGAINST') {
    // 1 banker, 2-5 opposed numbers.
    const opposedCount = randInt(2, 5)
    const banker = pickUniqueNumbers(1)
    const opposed = pickUniqueNumbers(opposedCount, { exclude: new Set(banker) })

    const combos = banker.length * opposed.length
    return {
      bet_type: betType,
      banker,
      opposed,
      selected_numbers: [],
      number_of_combinations: combos,
      amount_per_combination: amountPerCombination,
      total_amount: amountPerCombination * combos,
    }
  }

  // Fallback to DIRECT-2
  const selected = pickUniqueNumbers(2)
  return {
    bet_type: 'DIRECT-2',
    selected_numbers: selected,
    total_amount: amountPerCombination,
  }
}

async function httpJson(url, { method = 'GET', headers = {}, body } = {}) {
  const res = await fetch(url, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...headers,
    },
    body: body ? JSON.stringify(body) : undefined,
  })

  const text = await res.text()
  let data
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = { raw: text }
  }

  if (!res.ok) {
    const err = new Error(`HTTP ${res.status} ${res.statusText}`)
    err.status = res.status
    err.data = data
    throw err
  }

  return data
}

async function loginForToken() {
  if (PLAYER_JWT) return PLAYER_JWT
  if (!PLAYER_PASSWORD) {
    throw new Error('Missing auth: set PLAYER_JWT or PLAYER_PASSWORD')
  }

  const deviceId = crypto.randomUUID()
  const loginBody = {
    phone_number: PLAYER_PHONE,
    password: PLAYER_PASSWORD,
    device_id: deviceId,
    channel: 'mobile',
    device_info: {
      device_type: 'script',
      os: process.platform,
      os_version: process.version,
      app_version: 'seed-1.0',
      user_agent: 'seed-tickets-via-website.mjs',
    },
  }

  const data = await httpJson(`${API_BASE}/players/login`, { method: 'POST', body: loginBody })
  const token = data?.access_token
  if (!token) {
    throw new Error(`Login succeeded but no access_token in response: ${JSON.stringify(data).slice(0, 300)}`)
  }
  return token
}

async function resolveScheduleId() {
  if (GAME_SCHEDULE_ID && GAME_SCHEDULE_ID !== 'auto') return GAME_SCHEDULE_ID

  // Uses the same public endpoint the website uses to fetch a game's active schedule.
  const data = await httpJson(`${API_BASE}/players/games/${GAME_ID}/schedule`, { method: 'GET' })
  const scheduleId = data?.data?.schedule?.id ?? data?.schedule?.id
  if (!scheduleId) {
    throw new Error(`Could not resolve schedule id from /players/games/${GAME_ID}/schedule`)
  }
  return scheduleId
}

function computeTicketLevelSelectedNumbers(betLine) {
  // The website sends ticket-level selected_numbers for non-banker bets.
  if (betLine.banker?.length) return []
  return betLine.selected_numbers ?? []
}

async function submitTicket({ token, scheduleId, betLine, idx }) {
  const ticketReq = {
    game_code: GAME_CODE,
    game_schedule_id: scheduleId,
    draw_number: DRAW_NUMBER,
    selected_numbers: computeTicketLevelSelectedNumbers(betLine),
    bet_lines: [
      {
        line_number: 1,
        bet_type: betLine.bet_type,
        selected_numbers: betLine.selected_numbers ?? [],
        banker: betLine.banker,
        opposed: betLine.opposed,
        number_of_combinations: betLine.number_of_combinations,
        amount_per_combination: betLine.amount_per_combination,
        total_amount: betLine.total_amount,
      },
    ],
    customer_phone: `+${PLAYER_PHONE}`,
    customer_email: 'player@example.com',
    payment_method: 'WALLET',
  }

  const startedAt = Date.now()
  const data = await httpJson(`${API_BASE}/players/${PLAYER_ID}/tickets`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: ticketReq,
  })

  const tookMs = Date.now() - startedAt
  const serial =
    data?.data?.ticket?.serial_number ??
    data?.ticket?.serial_number ??
    data?.data?.serial_number ??
    data?.serial_number

  return { ok: true, idx, tookMs, serial, response: data }
}

async function main() {
  if (!Number.isFinite(DRAW_NUMBER) || DRAW_NUMBER < 1) {
    throw new Error(`Invalid DRAW_NUMBER: ${DRAW_NUMBER}`)
  }
  if (COUNT < 1) {
    throw new Error(`Invalid COUNT: ${COUNT}`)
  }
  if (INCLUDE_PAIR_TICKETS > COUNT) {
    throw new Error(`INCLUDE_PAIR_TICKETS (${INCLUDE_PAIR_TICKETS}) must be <= COUNT (${COUNT})`)
  }

  const token = await loginForToken()
  const scheduleId = await resolveScheduleId()

  // Pick which indices must include the (15,22) pair.
  const pairIdxs = new Set()
  while (pairIdxs.size < INCLUDE_PAIR_TICKETS) {
    pairIdxs.add(randInt(1, COUNT))
  }

  const results = []
  let success = 0
  let fail = 0

  const queue = []
  for (let i = 1; i <= COUNT; i++) queue.push(i)

  async function worker(workerId) {
    while (queue.length) {
      const idx = queue.shift()
      const forcePair = pairIdxs.has(idx)

      // If we must force the pair, pick a bet type that can contain both numbers reliably.
      const betType = forcePair ? 'PERM-2' : sample(BET_TYPES)
      const betLine = buildBetLine(betType, { forcePair })

      try {
        const res = await submitTicket({ token, scheduleId, betLine, idx })
        results.push(res)
        success++
        const pairTag = forcePair ? ' pair(15,22)' : ''
        console.log(
          `[${workerId}] #${idx} OK ${betLine.bet_type} total=${betLine.total_amount}psw serial=${res.serial ?? 'n/a'}${pairTag}`,
        )
      } catch (err) {
        fail++
        const status = err?.status ?? 'n/a'
        const msg = err?.data?.message ?? err?.message ?? String(err)
        console.log(`[${workerId}] #${idx} FAIL status=${status} bet=${betLine.bet_type} msg=${String(msg).slice(0, 180)}`)
        results.push({ ok: false, idx, error: { status, msg, data: err?.data } })
      }

      if (DELAY_MS) await sleep(DELAY_MS)
    }
  }

  const workers = []
  for (let w = 1; w <= CONCURRENCY; w++) workers.push(worker(w))
  await Promise.all(workers)

  const pairOk = results.filter(r => r.ok && pairIdxs.has(r.idx)).length
  console.log('')
  console.log('summary:')
  console.log(`total_sent=${COUNT}`)
  console.log(`success_count=${success}`)
  console.log(`failure_count=${fail}`)
  console.log(`pair_15_22_requested=${INCLUDE_PAIR_TICKETS}`)
  console.log(`pair_15_22_success=${pairOk}`)
}

main().catch(err => {
  console.error('fatal:', err?.message ?? err)
  process.exit(1)
})
