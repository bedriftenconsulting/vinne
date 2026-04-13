import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { HelpCircleIcon, LightbulbIcon, TargetIcon } from 'lucide-react'
import type { CreateGameRequest } from '@/services/games'

interface GameConfigGuideProps {
  formData: CreateGameRequest
}

interface GuideSection {
  title: string
  content: string
  type: 'info' | 'tip' | 'warning'
}

export function GameConfigGuide({ formData }: GameConfigGuideProps) {
  const getFieldGuidance = (): GuideSection[] => {
    const guidance: GuideSection[] = []

    // Number range guidance
    if (formData.number_range_max - formData.number_range_min + 1 > 0) {
      const range = formData.number_range_max - formData.number_range_min + 1
      if (range > 90) {
        guidance.push({
          title: 'Large Number Range',
          content: `With ${range} possible numbers, this creates very difficult odds. Consider reducing to 49-90 for better player engagement.`,
          type: 'warning',
        })
      } else if (range < 20) {
        guidance.push({
          title: 'Small Number Range',
          content: `With only ${range} numbers, odds will be very favorable. This works well for frequent draws or special games.`,
          type: 'info',
        })
      }
    }

    // Selection count guidance
    if (formData.selection_count > 0) {
      const ratio =
        formData.selection_count / (formData.number_range_max - formData.number_range_min + 1)
      if (ratio > 0.5) {
        guidance.push({
          title: 'High Selection Ratio',
          content:
            'Selecting more than half the available numbers makes the game very easy to win.',
          type: 'warning',
        })
      } else if (formData.selection_count >= 6) {
        guidance.push({
          title: 'Complex Selection',
          content: `Selecting ${formData.selection_count} numbers increases difficulty significantly. Consider prize tiers for partial matches.`,
          type: 'tip',
        })
      }
    }

    // Price guidance
    if (formData.base_price > 0) {
      if (formData.base_price > 10) {
        guidance.push({
          title: 'Premium Pricing',
          content:
            'Higher ticket prices typically reduce player participation but increase revenue per player.',
          type: 'info',
        })
      } else if (formData.base_price <= 2) {
        guidance.push({
          title: 'Affordable Entry',
          content:
            'Low ticket prices encourage broader participation, ideal for building player base.',
          type: 'tip',
        })
      }
    }

    // Game type specific guidance
    if (formData.game_type === '5_by_90') {
      guidance.push({
        title: '5/90 Direct Game',
        content:
          "Ghana's most popular format. Consider multiple draws per day (Noon Rush, Evening) for higher revenue.",
        type: 'tip',
      })
    } else if (formData.game_type === 'super_6') {
      guidance.push({
        title: 'Super 6 Premium',
        content:
          'High-value game with large jackpots. Typically drawn twice weekly with higher ticket prices.',
        type: 'info',
      })
    } else if (formData.game_type === 'perm') {
      guidance.push({
        title: 'Permutation Game',
        content:
          'Complex betting system with multiple win combinations. Ensure clear payout rules.',
        type: 'tip',
      })
    }

    // Frequency guidance
    if (formData.draw_frequency === 'daily') {
      guidance.push({
        title: 'Daily Draws',
        content:
          'Daily draws work well for popular games like 5/90. Consider specific times like Noon Rush.',
        type: 'tip',
      })
    } else if (formData.draw_frequency === 'special') {
      guidance.push({
        title: 'Special Event Draws',
        content:
          'Special draws for holidays or events. Higher ticket prices and marketing are recommended.',
        type: 'info',
      })
    } else if (formData.draw_frequency === 'monthly') {
      guidance.push({
        title: 'Monthly Draws',
        content:
          'Monthly draws allow larger jackpots to build but may reduce player engagement between draws.',
        type: 'info',
      })
    }

    // Bonus numbers guidance
    if (formData.bonus_number_enabled) {
      guidance.push({
        title: 'Bonus Numbers',
        content:
          'Bonus numbers create additional prize tiers and excitement but significantly increase odds complexity.',
        type: 'tip',
      })
    }

    // Multi-draw guidance
    if (formData.multi_draw_enabled) {
      guidance.push({
        title: 'Multi-Draw Feature',
        content:
          'Multi-draw allows players to enter multiple consecutive draws with one purchase, improving convenience and revenue.',
        type: 'info',
      })
    } else if (formData.draw_frequency === 'daily' || formData.draw_frequency === 'weekly') {
      guidance.push({
        title: 'Multi-Draw Opportunity',
        content:
          'Frequent draws benefit from multi-draw options. Players can buy tickets for multiple future draws.',
        type: 'tip',
      })
    }

    // Ticket limit guidance
    if (formData.max_tickets_per_player < 5) {
      guidance.push({
        title: 'Low Ticket Limits',
        content:
          'Very low ticket limits may restrict enthusiastic players and reduce potential revenue.',
        type: 'warning',
      })
    } else if (formData.max_tickets_per_player > 100) {
      guidance.push({
        title: 'High Ticket Limits',
        content:
          'Very high limits require responsible gaming measures and may need regulatory approval.',
        type: 'warning',
      })
    }

    return guidance
  }

  const getPopularFormats = () => [
    {
      name: '5/90 NLA Standard',
      format: '5 from 90',
      description: "Ghana NLA's core format - most popular nationwide",
    },
    { name: 'Super 6', format: '6 from 55', description: 'Premium game, GHS 100K minimum jackpot' },
    {
      name: 'Direct Betting',
      format: '2-5 exact matches',
      description: '240x to 15,000x payouts (NLA standard)',
    },
    {
      name: 'Permutation',
      format: 'Auto combinations',
      description: 'Select 3+ numbers, system creates all combinations',
    },
    {
      name: 'Banker Against All',
      format: '1 banker + 89',
      description: 'One guaranteed number, 960x payout',
    },
    {
      name: 'Noon Rush',
      format: 'Daily 1:30 PM',
      description: 'Mon-Sat midday draws (NLA schedule)',
    },
  ]

  const guidance = getFieldGuidance()

  return (
    <div className="space-y-4">
      {/* Quick Reference */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <TargetIcon className="h-5 w-5" />
            Popular Game Formats
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3">
            {getPopularFormats().map((format, index) => (
              <div
                key={index}
                className="flex items-center justify-between p-3 bg-gray-50 rounded-lg"
              >
                <div>
                  <div className="font-medium">{format.name}</div>
                  <div className="text-sm text-gray-600">{format.description}</div>
                </div>
                <Badge variant="outline" className="ml-2">
                  {format.format}
                </Badge>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Dynamic Guidance */}
      {guidance.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-lg">
              <LightbulbIcon className="h-5 w-5" />
              Configuration Guidance
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {guidance.map((guide, index) => {
              const Icon =
                guide.type === 'warning'
                  ? HelpCircleIcon
                  : guide.type === 'tip'
                    ? LightbulbIcon
                    : HelpCircleIcon

              const bgColor =
                guide.type === 'warning'
                  ? 'bg-yellow-50 border-yellow-200'
                  : guide.type === 'tip'
                    ? 'bg-blue-50 border-blue-200'
                    : 'bg-gray-50 border-gray-200'

              const iconColor =
                guide.type === 'warning'
                  ? 'text-yellow-600'
                  : guide.type === 'tip'
                    ? 'text-blue-600'
                    : 'text-gray-600'

              return (
                <div key={index} className={`p-4 rounded-lg border ${bgColor}`}>
                  <div className="flex items-start gap-3">
                    <Icon className={`h-5 w-5 mt-0.5 ${iconColor}`} />
                    <div>
                      <div className="font-medium mb-1">{guide.title}</div>
                      <div className="text-sm text-gray-700">{guide.content}</div>
                    </div>
                  </div>
                </div>
              )
            })}
          </CardContent>
        </Card>
      )}

      {/* Quick Tips */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <HelpCircleIcon className="h-5 w-5" />
            Quick Tips
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <div className="text-sm space-y-2">
            <div className="flex items-start gap-2">
              <div className="w-1 h-1 bg-gray-400 rounded-full mt-2 flex-shrink-0"></div>
              <div>For new games, start with proven formats before experimenting</div>
            </div>
            <div className="flex items-start gap-2">
              <div className="w-1 h-1 bg-gray-400 rounded-full mt-2 flex-shrink-0"></div>
              <div>Consider your target audience when setting ticket prices</div>
            </div>
            <div className="flex items-start gap-2">
              <div className="w-1 h-1 bg-gray-400 rounded-full mt-2 flex-shrink-0"></div>
              <div>Test draw frequency with player behavior patterns</div>
            </div>
            <div className="flex items-start gap-2">
              <div className="w-1 h-1 bg-gray-400 rounded-full mt-2 flex-shrink-0"></div>
              <div>Bonus numbers add complexity but also more ways to win</div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
