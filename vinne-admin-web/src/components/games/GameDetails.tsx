import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Info,
  Settings,
  DollarSign,
  Calendar,
  Trophy,
  Clock,
  Edit,
  Pause,
  Archive,
  AlertCircle,
  CheckCircle,
} from 'lucide-react'
import { formatInGhanaTime } from '@/lib/date-utils'
import { gameService, type Game } from '@/services/games'

interface GameDetailsProps {
  game: Game
  isOpen: boolean
  onClose: () => void
  onEdit: () => void
  onManageSchedule: () => void
  onManagePrizes: () => void
}

export function GameDetails({
  game,
  isOpen,
  onClose,
  onEdit,
  onManageSchedule,
  onManagePrizes,
}: GameDetailsProps) {
  const [activeTab, setActiveTab] = useState('overview')

  // Fetch additional game data
  const { data: prizeStructure } = useQuery({
    queryKey: ['prize-structure', game.id],
    queryFn: () => gameService.getPrizeStructure(game.id),
    enabled: isOpen,
  })

  const { data: upcomingDraws } = useQuery({
    queryKey: ['upcoming-draws', game.id],
    queryFn: () => gameService.getUpcomingDraws(game.id),
    enabled: isOpen,
  })

  const getStatusBadge = (status: string) => {
    const statusMap: {
      [key: string]: {
        variant: 'default' | 'secondary' | 'outline' | 'destructive'
        icon: React.ComponentType<{ className?: string }>
        label: string
      }
    } = {
      active: { variant: 'default', icon: CheckCircle, label: 'Active' },
      draft: { variant: 'secondary', icon: Edit, label: 'Draft' },
      pending_approval: { variant: 'outline', icon: Clock, label: 'Pending Approval' },
      suspended: { variant: 'destructive', icon: Pause, label: 'Suspended' },
      archived: { variant: 'secondary', icon: Archive, label: 'Archived' },
    }

    const normalizedStatus = status
      .toLowerCase()
      .replace(/([A-Z])/g, '_$1')
      .toLowerCase()
    const config = statusMap[normalizedStatus] || {
      variant: 'secondary',
      icon: AlertCircle,
      label: status,
    }
    const Icon = config.icon

    return (
      <Badge variant={config.variant} className="gap-1">
        <Icon className="h-3 w-3" />
        {config.label}
      </Badge>
    )
  }

  const getGameTypeLabel = (type: string) => {
    const typeMap: { [key: string]: string } = {
      national: 'National Game',
      private: 'Private Game',
      '5_by_90': '5/90 Game',
      direct: 'Direct Game',
      perm: 'Perm Game',
      banker: 'Banker Game',
      super_6: 'Super 6',
      midweek: 'Midweek Special',
      aseda: 'Aseda',
      bonanza: 'Bonanza',
      noon_rush: 'Noon Rush',
      evening: 'Evening Draw',
    }
    return typeMap[type] || type
  }

  const getDrawFrequencyLabel = (frequency: string) => {
    return frequency.charAt(0).toUpperCase() + frequency.slice(1).replace('_', ' ')
  }

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-5xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <div className="flex items-center justify-between">
            <div>
              <DialogTitle className="text-2xl">{game.name}</DialogTitle>
              <DialogDescription className="mt-2">
                Game Code: {game.code} • Version: {game.version}
              </DialogDescription>
            </div>
            <div className="flex items-center gap-2">
              {getStatusBadge(game.status)}
              <Button variant="outline" size="sm" onClick={onEdit}>
                <Edit className="mr-2 h-4 w-4" />
                Edit
              </Button>
            </div>
          </div>
        </DialogHeader>

        <Tabs value={activeTab} onValueChange={setActiveTab} className="mt-6">
          <TabsList className="grid w-full grid-cols-5">
            <TabsTrigger value="overview" className="gap-2">
              <Info className="h-4 w-4" />
              Overview
            </TabsTrigger>
            <TabsTrigger value="rules" className="gap-2">
              <Settings className="h-4 w-4" />
              Rules
            </TabsTrigger>
            <TabsTrigger value="pricing" className="gap-2">
              <DollarSign className="h-4 w-4" />
              Pricing
            </TabsTrigger>
            <TabsTrigger value="schedule" className="gap-2">
              <Calendar className="h-4 w-4" />
              Schedule
            </TabsTrigger>
            <TabsTrigger value="prizes" className="gap-2">
              <Trophy className="h-4 w-4" />
              Prizes
            </TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Game Information</CardTitle>
                <CardDescription>Basic details about this lottery game</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-3">
                    <div>
                      <p className="text-sm text-muted-foreground">Game Type</p>
                      <p className="font-medium">
                        {getGameTypeLabel(game.game_type || game.type || 'unknown')}
                      </p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Created Date</p>
                      <p className="font-medium">
                        {formatInGhanaTime(new Date(game.created_at), 'PPP')}
                      </p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Last Updated</p>
                      <p className="font-medium">
                        {formatInGhanaTime(new Date(game.updated_at), 'PPP')}
                      </p>
                    </div>
                  </div>
                  <div className="space-y-3">
                    <div>
                      <p className="text-sm text-muted-foreground">Draw Frequency</p>
                      <p className="font-medium">{getDrawFrequencyLabel(game.draw_frequency)}</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Sales Cutoff</p>
                      <p className="font-medium">{game.sales_cutoff_minutes} minutes before draw</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Multi-Draw</p>
                      <p className="font-medium">
                        {game.multi_draw_enabled ? (
                          <span className="text-green-600">
                            Enabled (Max: {game.max_multi_draws || 'N/A'})
                          </span>
                        ) : (
                          <span className="text-muted-foreground">Disabled</span>
                        )}
                      </p>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Performance Metrics</CardTitle>
                <CardDescription>Game statistics and performance indicators</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-4 gap-4">
                  <div className="text-center">
                    <div className="text-2xl font-bold">0</div>
                    <p className="text-xs text-muted-foreground">Total Tickets Sold</p>
                  </div>
                  <div className="text-center">
                    <div className="text-2xl font-bold">₵0</div>
                    <p className="text-xs text-muted-foreground">Total Revenue</p>
                  </div>
                  <div className="text-center">
                    <div className="text-2xl font-bold">0</div>
                    <p className="text-xs text-muted-foreground">Total Draws</p>
                  </div>
                  <div className="text-center">
                    <div className="text-2xl font-bold">₵0</div>
                    <p className="text-xs text-muted-foreground">Total Payouts</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="rules" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Game Rules Configuration</CardTitle>
                <CardDescription>Number selection rules</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm text-muted-foreground">Number Range</p>
                    <p className="text-lg font-medium">
                      {game.number_range_min} - {game.number_range_max}
                    </p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Selection Count</p>
                    <p className="text-lg font-medium">{game.selection_count} numbers</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="pricing" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Pricing & Limits</CardTitle>
                <CardDescription>Ticket pricing and purchase limits</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-3">
                    <div>
                      <p className="text-sm text-muted-foreground">Base Ticket Price</p>
                      <p className="text-2xl font-bold">
                        ₵{(game.base_price || game.ticket_price || 0).toFixed(2)}
                      </p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Max Tickets per Player</p>
                      <p className="text-lg font-medium">{game.max_tickets_per_player}</p>
                    </div>
                  </div>
                  <div className="space-y-3">
                    <div>
                      <p className="text-sm text-muted-foreground">Price Range</p>
                      <p className="text-lg font-medium">₵0.50 - ₵200.00</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Max Tickets per Transaction</p>
                      <p className="text-lg font-medium">{game.max_tickets_per_player}</p>
                    </div>
                  </div>
                </div>

                {game.multi_draw_enabled && (
                  <>
                    <Separator />
                    <div className="rounded-lg bg-muted p-4">
                      <p className="font-medium mb-2">Multi-Draw Settings</p>
                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <p className="text-sm text-muted-foreground">Status</p>
                          <p className="text-sm font-medium text-green-600">Enabled</p>
                        </div>
                        <div>
                          <p className="text-sm text-muted-foreground">Maximum Advance Draws</p>
                          <p className="text-sm font-medium">{game.max_multi_draws || 'Not set'}</p>
                        </div>
                      </div>
                    </div>
                  </>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="schedule" className="space-y-4">
            <Card>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle>Draw Schedule</CardTitle>
                    <CardDescription>Upcoming draws and timing configuration</CardDescription>
                  </div>
                  <Button size="sm" onClick={onManageSchedule}>
                    <Calendar className="mr-2 h-4 w-4" />
                    Manage Schedule
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div className="grid grid-cols-3 gap-4">
                    <div>
                      <p className="text-sm text-muted-foreground">Frequency</p>
                      <p className="font-medium">{getDrawFrequencyLabel(game.draw_frequency)}</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Draw Time</p>
                      <p className="font-medium">{game.draw_time || 'Not set'}</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Sales Cutoff</p>
                      <p className="font-medium">{game.sales_cutoff_minutes} min before draw</p>
                    </div>
                  </div>

                  {game.draw_days && game.draw_days.length > 0 && (
                    <div>
                      <p className="text-sm text-muted-foreground mb-2">Draw Days</p>
                      <div className="flex gap-2">
                        {game.draw_days.map(day => (
                          <Badge key={day} variant="secondary">
                            {day.charAt(0).toUpperCase() + day.slice(1)}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  )}

                  <Separator />

                  <div>
                    <p className="font-medium mb-3">Upcoming Draws</p>
                    {!upcomingDraws || upcomingDraws.length === 0 ? (
                      <p className="text-sm text-muted-foreground">No upcoming draws scheduled</p>
                    ) : (
                      <div className="space-y-2">
                        {upcomingDraws.slice(0, 5).map(draw => (
                          <div
                            key={draw.id}
                            className="flex items-center justify-between p-2 rounded-lg border"
                          >
                            <div>
                              <p className="text-sm font-medium">
                                {formatInGhanaTime(new Date(draw.draw_date), 'PPP')}
                              </p>
                              <p className="text-xs text-muted-foreground">
                                Draw at {draw.draw_time} • Sales close at {draw.sales_cutoff_time}
                              </p>
                            </div>
                            {draw.is_special && (
                              <Badge variant="default">{draw.special_name || 'Special Draw'}</Badge>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="prizes" className="space-y-4">
            <Card>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle>Prize Structure</CardTitle>
                    <CardDescription>
                      Prize pool configuration and tier distribution
                    </CardDescription>
                  </div>
                  <Button size="sm" onClick={onManagePrizes}>
                    <Trophy className="mr-2 h-4 w-4" />
                    Manage Prizes
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                {!prizeStructure ? (
                  <div className="text-center py-8">
                    <Trophy className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                    <p className="text-muted-foreground">No prize structure configured</p>
                    <Button variant="outline" className="mt-4" onClick={onManagePrizes}>
                      Configure Prize Structure
                    </Button>
                  </div>
                ) : (
                  <div className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <p className="text-sm text-muted-foreground">Prize Pool</p>
                        <p className="text-lg font-medium">50% of sales</p>
                      </div>
                      <div>
                        <p className="text-sm text-muted-foreground">Rollover</p>
                        <p className="text-lg font-medium">
                          {prizeStructure ? 'Enabled' : 'Disabled'}
                        </p>
                      </div>
                    </div>

                    <div>
                      <p className="text-sm text-muted-foreground">Guaranteed Minimum Jackpot</p>
                      <p className="text-lg font-medium">₵10,000</p>
                    </div>

                    <Separator />

                    <div>
                      <p className="font-medium mb-3">Prize Tiers</p>
                      <p className="text-sm text-muted-foreground">
                        Configure prize tiers in the prize management section
                      </p>
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
