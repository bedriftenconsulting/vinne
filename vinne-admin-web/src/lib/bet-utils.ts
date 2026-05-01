/**
 * Utility functions for handling bet lines in both old and new (compact) formats
 */

import type { BetLine } from '@/services/tickets'

/**
 * Get the numbers from a bet line
 */
export function getBetLineNumbers(betLine: BetLine): number[] {
  return betLine.selected_numbers || []
}

/**
 * Get the amount from a bet line
 */
export function getBetLineAmount(betLine: BetLine): number {
  return betLine.total_amount ?? 0
}

/**
 * Check if a bet line uses the compact format
 */
export function isCompactFormat(betLine: BetLine): boolean {
  return betLine.number_of_combinations !== undefined && betLine.number_of_combinations > 0
}

/**
 * Check if a bet type is a PERM bet
 */
export function isPermBet(betType: string | undefined | null): boolean {
  if (!betType) return false
  const normalized = betType.toUpperCase().trim()
  return normalized.startsWith('PERM-') || normalized.startsWith('PERM ')
}

/**
 * Check if a bet type is a Banker bet
 */
export function isBankerBet(betType: string | undefined | null): boolean {
  if (!betType) return false
  const normalized = betType.toUpperCase().trim()
  return (
    normalized === 'BANKER' ||
    normalized === 'BANKER ALL' ||
    normalized.includes('BANKER AG') ||
    normalized === 'AGAINST'
  )
}

/**
 * Extract PERM size from bet type (e.g., "PERM-2" -> 2)
 */
export function getPermSize(betType: string | undefined | null): number | null {
  if (!betType) return null
  const match = betType.match(/PERM[-\s](\d+)/i)
  return match ? parseInt(match[1], 10) : null
}

/**
 * Calculate binomial coefficient C(n, r) = n! / (r! * (n-r)!)
 */
export function calculateCombinations(n: number, r: number): number {
  if (r > n || r < 0) return 0
  if (r === 0 || r === n) return 1
  if (r > n - r) r = n - r // Optimization: C(n,r) = C(n,n-r)

  let result = 1
  for (let i = 0; i < r; i++) {
    result *= n - i
    result /= i + 1
  }
  return Math.round(result)
}

/**
 * Generate all combinations of size r from array of numbers
 */
export function generateCombinations(numbers: number[], r: number): number[][] {
  const result: number[][] = []
  const n = numbers.length

  if (r > n || r < 0) return result
  if (r === 0) return [[]]
  if (r === n) return [numbers]

  function backtrack(start: number, current: number[]) {
    if (current.length === r) {
      result.push([...current])
      return
    }

    for (let i = start; i <= n - (r - current.length); i++) {
      current.push(numbers[i])
      backtrack(i + 1, current)
      current.pop()
    }
  }

  backtrack(0, [])
  return result
}

/**
 * Format amount from pesewas to GHS with currency symbol
 */
export function formatAmount(pesewas: number): string {
  const ghs = pesewas / 100
  return `GHS ${ghs.toFixed(2)}`
}

/**
 * Format numbers array for display (e.g., [1, 2, 3] -> "1, 2, 3")
 */
export function formatNumbers(numbers: number[]): string {
  return numbers.join(', ')
}

/**
 * Get a summary description of a bet line for display
 */
export function getBetLineSummary(betLine: BetLine): string {
  const numbers = getBetLineNumbers(betLine)
  const amount = getBetLineAmount(betLine)
  const betType = betLine.bet_type

  if (isPermBet(betType)) {
    const permSize = getPermSize(betType)
    const combinations =
      betLine.number_of_combinations ?? calculateCombinations(numbers.length, permSize || 2)
    const amountPerCombo = betLine.amount_per_combination ?? amount / combinations

    return `${numbers.length} numbers → ${combinations} combinations @ ${formatAmount(amountPerCombo)} each = ${formatAmount(amount)}`
  }

  if (isBankerBet(betType)) {
    const bankerCount = betLine.banker?.length ?? 0
    const opposedCount = betLine.opposed?.length ?? 0
    const combinations =
      betLine.number_of_combinations ??
      (betType.toUpperCase().includes('ALL') ? 89 : bankerCount * opposedCount)

    return `${combinations} combinations @ ${formatAmount(betLine.amount_per_combination ?? amount / combinations)} each = ${formatAmount(amount)}`
  }

  // Direct bets
  return `${numbers.length} numbers × ${formatAmount(amount)}`
}
