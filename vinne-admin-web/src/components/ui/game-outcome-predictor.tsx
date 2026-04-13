import { useMemo } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { TrendingUpIcon } from 'lucide-react'
import type { CreateGameRequest } from '@/services/games'

interface GameOutcomePredictorProps {
  formData: CreateGameRequest
}

interface GameAnalysis {
  totalCombinations: number
  winProbability: number
  difficultyLevel: 'Very Easy' | 'Easy' | 'Medium' | 'Hard' | 'Very Hard'
  estimatedPlayers: number
  gameComplexity: number
  avgStake: number
}

function calculateCombinations(n: number, k: number): number {
  if (k > n) return 0
  if (k === 0) return 1

  let result = 1
  for (let i = 0; i < k; i++) {
    result *= (n - i) / (i + 1)
  }
  return Math.round(result)
}

export function GameOutcomePredictor({ formData }: GameOutcomePredictorProps) {
  // Game type specific analysis (Based on Ghana NLA Research 2025)
  const gameTypeConfig = {
    '5_by_90': {
      popularity: 1.5,
      complexityBase: 20,
      typicalMinStake: 0.5,
      typicalMaxStake: 10,
      description: "Ghana's most popular 5/90 format (NLA standard)",
      displayName: '5/90',
    },
    direct: {
      popularity: 1.3,
      complexityBase: 25,
      typicalMinStake: 0.5,
      typicalMaxStake: 20,
      description: 'Direct number betting (240x-15000x payouts)',
      displayName: 'Direct',
    },
    perm: {
      popularity: 1.1,
      complexityBase: 45,
      typicalMinStake: 1,
      typicalMaxStake: 50,
      description: 'Permutation betting (complex combinations)',
      displayName: 'Perm',
    },
    banker: {
      popularity: 0.9,
      complexityBase: 30,
      typicalMinStake: 1,
      typicalMaxStake: 30,
      description: 'Banker Against All (960x payout)',
      displayName: 'Banker',
    },
  }

  const currentGameType =
    gameTypeConfig[formData.game_format as keyof typeof gameTypeConfig] || gameTypeConfig['5_by_90']

  const analysis = useMemo((): GameAnalysis => {
    const {
      number_range_min,
      number_range_max,
      selection_count,
      min_stake = 0.5,
      max_stake = 200,
      draw_frequency,
    } = formData

    const numberRange = number_range_max - number_range_min + 1
    const totalCombinations = calculateCombinations(numberRange, selection_count)

    const winProbability = 1 / totalCombinations

    let difficultyLevel: GameAnalysis['difficultyLevel'] = 'Medium'
    if (totalCombinations < 1000) difficultyLevel = 'Very Easy'
    else if (totalCombinations < 100000) difficultyLevel = 'Easy'
    else if (totalCombinations < 10000000) difficultyLevel = 'Medium'
    else if (totalCombinations < 100000000) difficultyLevel = 'Hard'
    else difficultyLevel = 'Very Hard'

    const gameTypePopularity = currentGameType.popularity

    const basePlayers =
      difficultyLevel === 'Very Easy'
        ? 500
        : difficultyLevel === 'Easy'
          ? 2000
          : difficultyLevel === 'Medium'
            ? 5000
            : difficultyLevel === 'Hard'
              ? 10000
              : 20000

    // Calculate average stake based on min/max
    const avgStake = (min_stake + Math.min(max_stake, min_stake * 4)) / 2

    const stakeAdjustment =
      avgStake <= 1 ? 1.8 : avgStake <= 2 ? 1.5 : avgStake <= 5 ? 1.0 : avgStake <= 10 ? 0.7 : 0.5

    const frequencyMultiplier =
      draw_frequency === 'daily'
        ? 7
        : draw_frequency === 'weekly'
          ? 1
          : draw_frequency === 'bi_weekly'
            ? 0.5
            : draw_frequency === 'monthly'
              ? 0.25
              : draw_frequency === 'special'
                ? 0.1
                : 1

    const estimatedPlayers = Math.round(
      basePlayers * stakeAdjustment * frequencyMultiplier * gameTypePopularity
    )

    // Game complexity score (0-100)
    const complexityFactors = [
      currentGameType.complexityBase, // Base complexity varies by game type
      selection_count > 6 ? 15 : 0,
      numberRange > 50 ? 10 : 0,
      draw_frequency === 'daily' ? 15 : 0,
      max_stake > 50 ? 10 : 0,
    ]
    const gameComplexity = Math.min(
      100,
      complexityFactors.reduce((sum, factor) => sum + factor, 0)
    )

    return {
      totalCombinations,
      winProbability,
      difficultyLevel,
      estimatedPlayers,
      gameComplexity,
      avgStake,
    }
  }, [formData, currentGameType.complexityBase, currentGameType.popularity])

  const formatNumber = (num: number): string => {
    if (num >= 1e9) return `${(num / 1e9).toFixed(1)}B`
    if (num >= 1e6) return `${(num / 1e6).toFixed(1)}M`
    if (num >= 1e3) return `${(num / 1e3).toFixed(1)}K`
    return num.toString()
  }

  const formatProbability = (prob: number): string => {
    if (prob >= 0.01) return `${(prob * 100).toFixed(2)}%`
    return `1 in ${formatNumber(Math.round(1 / prob))}`
  }

  const getDifficultyColor = (level: string): string => {
    switch (level) {
      case 'Very Easy':
        return 'bg-green-100 text-green-800'
      case 'Easy':
        return 'bg-blue-100 text-blue-800'
      case 'Medium':
        return 'bg-yellow-100 text-yellow-800'
      case 'Hard':
        return 'bg-orange-100 text-orange-800'
      case 'Very Hard':
        return 'bg-red-100 text-red-800'
      default:
        return 'bg-gray-100 text-gray-800'
    }
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <TrendingUpIcon className="h-5 w-5" />
            Game Analysis & Predictions
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Key Metrics */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="text-center p-3 bg-gray-50 rounded-lg">
              <div className="text-2xl font-bold text-gray-900">
                {formatNumber(analysis.totalCombinations)}
              </div>
              <div className="text-sm text-gray-600">Total Combinations</div>
            </div>

            <div className="text-center p-3 bg-gray-50 rounded-lg">
              <div className="text-2xl font-bold text-gray-900">
                {formatProbability(analysis.winProbability)}
              </div>
              <div className="text-sm text-gray-600">Jackpot Odds</div>
            </div>

            <div className="text-center p-3 bg-gray-50 rounded-lg">
              <div className="text-2xl font-bold text-gray-900">
                {formatNumber(analysis.estimatedPlayers)}
              </div>
              <div className="text-sm text-gray-600">Est. Players/Draw</div>
              <div className="text-xs text-gray-500 mt-1">
                Avg {Math.min(formData.max_tickets_per_player, 5)} tickets each
              </div>
            </div>
          </div>

          {/* Difficulty & Complexity */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-medium">Difficulty Level</span>
                <Badge className={getDifficultyColor(analysis.difficultyLevel)}>
                  {analysis.difficultyLevel}
                </Badge>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div
                  className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                  style={{ width: `${Math.min(100, (analysis.totalCombinations / 1e8) * 100)}%` }}
                />
              </div>
            </div>

            <div>
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-medium">Game Complexity</span>
                <span className="text-sm text-gray-600">{analysis.gameComplexity}/100</span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div
                  className="bg-orange-500 h-2 rounded-full transition-all duration-300"
                  style={{ width: `${analysis.gameComplexity}%` }}
                />
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
