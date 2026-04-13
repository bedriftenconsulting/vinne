/**
 * PermCombinationViewer Component
 *
 * Displays all combinations for PERM and Banker bets in a collapsible, user-friendly format.
 * Supports both compact format (with pre-calculated combinations) and legacy format.
 */

import { useMemo, useState } from 'react'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import type { BetLine } from '@/services/tickets'
import {
  getBetLineNumbers,
  getBetLineAmount,
  isPermBet,
  isBankerBet,
  getPermSize,
  generateCombinations,
  formatAmount,
  formatNumbers,
  getBetLineSummary,
  isCompactFormat,
} from '@/lib/bet-utils'

interface PermCombinationViewerProps {
  betLine: BetLine
  className?: string
}

export function PermCombinationViewer({ betLine, className }: PermCombinationViewerProps) {
  const [isOpen, setIsOpen] = useState(false)

  // Get numbers and amount (supports both old and new formats)
  const numbers = getBetLineNumbers(betLine)
  const totalAmount = getBetLineAmount(betLine)
  const betType = betLine.bet_type

  // Generate combinations for PERM bets
  const combinations = useMemo(() => {
    if (isPermBet(betType)) {
      const permSize = getPermSize(betType)
      if (!permSize) return []
      return generateCombinations(numbers, permSize)
    }

    if (isBankerBet(betType)) {
      // For Banker bets, show banker + paired numbers
      const banker = betLine.banker || []
      const opposed = betLine.opposed || []

      if (betType.toUpperCase().includes('ALL')) {
        // Banker All: banker with every other number (1-90 except banker)
        const allNumbers = Array.from({ length: 90 }, (_, i) => i + 1)
        return allNumbers.filter(num => !banker.includes(num)).map(num => [...banker, num])
      }

      if (betType.toUpperCase().includes('AG') || betType.toUpperCase().includes('AGAINST')) {
        // Banker Against: banker with each opposed number
        return opposed.map(opp => [...banker, opp])
      }
    }

    return []
  }, [betLine, numbers, betType])

  const combinationCount = betLine.number_of_combinations ?? combinations.length
  const amountPerCombination = betLine.amount_per_combination ?? totalAmount / combinationCount

  // Don't render if not a PERM or Banker bet
  if (!isPermBet(betType) && !isBankerBet(betType)) {
    return null
  }

  return (
    <Card className={className}>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base font-medium">
            {betType}{' '}
            {isCompactFormat(betLine) && (
              <Badge variant="secondary" className="ml-2">
                Compact Format
              </Badge>
            )}
          </CardTitle>
          <div className="flex items-center gap-2">
            <Badge variant="outline">{combinationCount} combinations</Badge>
            <Badge>{formatAmount(totalAmount)}</Badge>
          </div>
        </div>
      </CardHeader>

      <CardContent>
        {/* Summary */}
        <div className="mb-3 text-sm text-muted-foreground">
          <p className="font-medium">{getBetLineSummary(betLine)}</p>
          {isPermBet(betType) && (
            <p className="mt-1">
              Selected Numbers: <span className="font-mono">{formatNumbers(numbers)}</span>
            </p>
          )}
          {isBankerBet(betType) && (
            <div className="mt-1 space-y-1">
              {betLine.banker && betLine.banker.length > 0 && (
                <p>
                  Banker: <span className="font-mono">{formatNumbers(betLine.banker)}</span>
                </p>
              )}
              {betLine.opposed && betLine.opposed.length > 0 && (
                <p>
                  Opposed: <span className="font-mono">{formatNumbers(betLine.opposed)}</span>
                </p>
              )}
            </div>
          )}
        </div>

        {/* Collapsible Combinations List */}
        <Collapsible open={isOpen} onOpenChange={setIsOpen}>
          <CollapsibleTrigger asChild>
            <Button variant="outline" size="sm" className="w-full">
              {isOpen ? (
                <>
                  <ChevronUp className="mr-2 h-4 w-4" />
                  Hide Combinations
                </>
              ) : (
                <>
                  <ChevronDown className="mr-2 h-4 w-4" />
                  Show All {combinationCount} Combinations
                </>
              )}
            </Button>
          </CollapsibleTrigger>

          <CollapsibleContent className="mt-3">
            <div className="max-h-[400px] overflow-y-auto rounded-md border">
              <div className="grid gap-2 p-3">
                {combinations.length > 0 ? (
                  combinations.map((combo, index) => (
                    <div
                      key={index}
                      className="flex items-center justify-between rounded-md bg-muted/50 p-2 text-sm"
                    >
                      <div className="flex items-center gap-2">
                        <span className="text-muted-foreground">#{index + 1}</span>
                        <span className="font-mono font-medium">{formatNumbers(combo)}</span>
                      </div>
                      <span className="text-xs text-muted-foreground">
                        {formatAmount(amountPerCombination)}
                      </span>
                    </div>
                  ))
                ) : (
                  <div className="p-4 text-center text-sm text-muted-foreground">
                    {combinationCount} combinations
                    {isBankerBet(betType) && betType.toUpperCase().includes('ALL')
                      ? ' (too many to display)'
                      : ''}
                  </div>
                )}
              </div>
            </div>
          </CollapsibleContent>
        </Collapsible>

        {/* Compact Format Info */}
        {isCompactFormat(betLine) && (
          <div className="mt-3 rounded-md bg-blue-50 p-2 text-xs text-blue-900 dark:bg-blue-950 dark:text-blue-100">
            <p className="font-medium">Compact Storage Format</p>
            <p className="mt-1 text-blue-700 dark:text-blue-300">
              This ticket uses the new compact format, storing metadata once instead of{' '}
              {combinationCount} individual records (storage saved:{' '}
              {Math.round((1 - 1 / combinationCount) * 100)}%)
            </p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
