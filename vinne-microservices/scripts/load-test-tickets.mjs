#!/usr/bin/env node

import { randomUUID } from 'node:crypto'

const BET_TYPES = [
  'DIRECT_1',
  'DIRECT_2',
  'DIRECT_3',
  'PERM_2',
  'PERM_3',
  'BANKER',
  'BANKER_AGAINST',
]

const config = {
  endpoint: process.env.ENDPOINT || 'http://localhost:4000/api/v1/tickets',
  payloadMode: (process.env.PAYLOAD_MODE || 'generic').toLowerCase(), // generic | website
  token: process.env.JWT_TOKEN || '',
  transactions: Number(process.env.TRANSACTIONS || 100),
  mode: (process.env.MODE || 'sequential').toLowerCase(), // sequential | parallel
  concurrency: Number(process.env.CONCURRENCY || 10),
  delayMs: Number(process.env.DELAY_MS || 100),
  retries: Number(process.env.RETRIES || 2),
  minAmount: Number(process.env.MIN_AMOUNT || 1),
  maxAmount: Number(process.env.MAX_AMOUNT || 20),
  drawId:
    process.env.DRAW_ID ||
    '00000000-0000-0000-0000-000000000001',
  playerId: process.env.PLAYER_ID || '',
  customerPhone: process.env.CUSTOMER_PHONE || '',
  customerEmail: process.env.CUSTOMER_EMAIL || 'player@example.com',
  gameCode: process.env.GAME_CODE || 'FRINON',
  gameScheduleId: process.env.GAME_SCHEDULE_ID || 'fd8f942c-8d9f-4745-a2a0-b91f150668cb',
  drawNumber: Number(process.env.DRAW_NUMBER || 1),
}

if (!config.token) {
  console.error('Missing JWT token. Set JWT_TOKEN env var.')
  process.exit(1)
}

if (!['sequential', 'parallel'].includes(config.mode)) {
  console.error('Invalid MODE. Use sequential or parallel.')
  process.exit(1)
}

if (config.transactions <= 0) {
  console.error('TRANSACTIONS must be > 0.')
  process.exit(1)
}

if (config.payloadMode === 'website' && !config.playerId) {
  console.error('PAYLOAD_MODE=website requires PLAYER_ID.')
  process.exit(1)
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

function randInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min
}

function sample(array) {
  return array[randInt(0, array.length - 1)]
}

function uniqueNumbers(count, min = 1, max = 90) {
  const set = new Set()
  while (set.size < count) set.add(randInt(min, max))
  return [...set]
}

function buildBetPayload(betType) {
  if (betType === 'DIRECT_1') return { numbers: uniqueNumbers(1) }
  if (betType === 'DIRECT_2') return { numbers: uniqueNumbers(2) }
  if (betType === 'DIRECT_3') return { numbers: uniqueNumbers(3) }
  if (betType === 'PERM_2') return { numbers: uniqueNumbers(2) }
  if (betType === 'PERM_3') return { numbers: uniqueNumbers(3) }
  if (betType === 'BANKER') {
    const banker = uniqueNumbers(1)[0]
    const others = uniqueNumbers(randInt(2, 4)).filter(n => n !== banker)
    while (others.length < 2) {
      const extra = randInt(1, 90)
      if (extra !== banker && !others.includes(extra)) others.push(extra)
    }
    return { banker, numbers: others }
  }
  // BANKER_AGAINST
  const banker = uniqueNumbers(1)[0]
  const others = uniqueNumbers(randInt(2, 5)).filter(n => n !== banker)
  while (others.length < 2) {
    const extra = randInt(1, 90)
    if (extra !== banker && !others.includes(extra)) others.push(extra)
  }
  return { banker, numbers: others }
}

function buildTicket(index) {
  const bet_type = sample(BET_TYPES)
  const bet = buildBetPayload(bet_type)
  if (config.payloadMode === 'website') {
    const normalizedBetType = bet_type.replace('_', '-')
    const stakePesewas = randInt(config.minAmount, config.maxAmount) * 100

    const line = {
      line_number: 1,
      bet_type: normalizedBetType,
      selected_numbers: [],
      total_amount: stakePesewas,
    }

    if (bet_type === 'BANKER') {
      line.banker = [bet.banker]
      line.selected_numbers = bet.numbers
    } else if (bet_type === 'BANKER_AGAINST') {
      line.banker = [bet.banker]
      line.opposed = bet.numbers
    } else {
      line.selected_numbers = bet.numbers
      if (bet_type === 'PERM_2' || bet_type === 'PERM_3') {
        line.number_of_combinations = 1
        line.amount_per_combination = stakePesewas
      }
    }

    return {
      game_code: config.gameCode,
      game_schedule_id: config.gameScheduleId,
      draw_number: config.drawNumber,
      selected_numbers: line.selected_numbers,
      bet_lines: [line],
      customer_phone: config.customerPhone,
      customer_email: config.customerEmail,
      payment_method: 'WALLET',
      timestamp: new Date().toISOString(),
      external_ref: `loadtest-${Date.now()}-${index}-${randomUUID().slice(0, 8)}`,
    }
  }

  return {
    game_type: '5/90',
    bet_type,
    ...bet,
    amount: randInt(config.minAmount, config.maxAmount),
    draw_id: config.drawId,
    terminal_id: randomUUID(),
    agent_id: randomUUID(),
    timestamp: new Date().toISOString(),
    external_ref: `loadtest-${Date.now()}-${index}-${randomUUID().slice(0, 8)}`,
  }
}

async function sendWithRetry(index) {
  const payload = buildTicket(index)
  const targetEndpoint =
    config.payloadMode === 'website'
      ? `http://localhost:4000/api/v1/players/${config.playerId}/tickets`
      : config.endpoint
  let attempt = 0
  let lastError = null

  while (attempt <= config.retries) {
    attempt += 1
    try {
      const res = await fetch(targetEndpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${config.token}`,
        },
        body: JSON.stringify(payload),
      })

      const bodyText = await res.text()
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}: ${bodyText}`)
      }

      console.log(`[${index}] SUCCESS (attempt ${attempt})`)
      return { ok: true }
    } catch (err) {
      lastError = err
      console.log(`[${index}] FAIL (attempt ${attempt}) -> ${err.message}`)
      if (attempt <= config.retries) {
        await sleep(100)
      }
    }
  }

  return { ok: false, error: lastError?.message || 'unknown error' }
}

async function runSequential() {
  const results = []
  for (let i = 1; i <= config.transactions; i += 1) {
    if (config.delayMs > 0) await sleep(config.delayMs)
    // eslint-disable-next-line no-await-in-loop
    results.push(await sendWithRetry(i))
  }
  return results
}

async function runParallel() {
  const results = new Array(config.transactions)
  let next = 1

  async function worker() {
    while (true) {
      const current = next
      next += 1
      if (current > config.transactions) return
      if (config.delayMs > 0) await sleep(config.delayMs)
      results[current - 1] = await sendWithRetry(current)
    }
  }

  const workers = Array.from(
    { length: Math.max(1, Math.min(config.concurrency, config.transactions)) },
    () => worker()
  )
  await Promise.all(workers)
  return results
}

async function main() {
  console.log('Starting ticket load test with config:')
  console.log(JSON.stringify(config, null, 2))

  const startedAt = Date.now()
  const results = config.mode === 'parallel' ? await runParallel() : await runSequential()
  const durationMs = Date.now() - startedAt

  const success_count = results.filter(r => r?.ok).length
  const failure_count = results.length - success_count

  console.log('\nSummary')
  console.log(`total_sent: ${results.length}`)
  console.log(`success_count: ${success_count}`)
  console.log(`failure_count: ${failure_count}`)
  console.log(`duration_ms: ${durationMs}`)

  if (failure_count > 0) process.exitCode = 1
}

main().catch(err => {
  console.error('Fatal error:', err)
  process.exit(1)
})
