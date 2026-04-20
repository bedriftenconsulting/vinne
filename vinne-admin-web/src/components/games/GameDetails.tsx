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
  DollarSign,
  Calendar,
  Trophy,
  Clock,
  Edit,
  Pause,
  Archive,
  AlertCircle,
  CheckCircle,
  FileText,
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
}: GameDetailsProps) {
  const [activeTab, setActiveTab] = useState('overview')

  // Fetch fresh game data so prize_details and dates are always up-to-date
  const { data: freshGame, isLoading: gameLoading } = useQuery({
    queryKey: ['game-detail', game.id],
    queryFn: () => gameService.getGame(game.id),
    enabled: isOpen,
    staleTime: 0,
  })

  // Fetch scheduled draws for this game (from this week onwards)
  const todayStr = new Date().toISOString().split('T')[0]
  const { data: schedules } = useQuery({
    queryKey: ['weekly-schedule-detail', game.id, todayStr],
    queryFn: () => gameService.getWeeklySchedule(todayStr),
    enabled: isOpen,
    staleTime: 0,
  })

  // Always use freshGame — avoids stale list data that may lack draw_date
  const g = freshGame ?? game

  // Draws belonging to this game
  const gameSchedules = (schedules || []).filter(s => s.game_id === game.id)

  const getStatusBadge = (status: string) => {
    const s = status.toUpperCase()
    const map: Record<string, { variant: 'default' | 'secondary' | 'outline' | 'destructive'; icon: React.ComponentType<{ className?: string }>; label: string }> = {
      ACTIVE:    { variant: 'default',     icon: CheckCircle, label: 'Active' },
      DRAFT:     { variant: 'secondary',   icon: Edit,        label: 'Draft' },
      SUSPENDED: { variant: 'destructive', icon: Pause,       label: 'Suspended' },
      ARCHIVED:  { variant: 'secondary',   icon: Archive,     label: 'Archived' },
    }
    const cfg = map[s] || { variant: 'secondary', icon: AlertCircle, label: status }
    const Icon = cfg.icon
    return (
      <Badge variant={cfg.variant} className="gap-1">
        <Icon className="h-3 w-3" />
        {cfg.label}
      </Badge>
    )
  }

  const freqLabel = (f: string) =>
    f === 'bi_weekly' ? 'Bi-Weekly' : f.charAt(0).toUpperCase() + f.slice(1).replace('_', ' ')

  const fmtDate = (d: string | undefined) =>
    d ? formatInGhanaTime(d, 'PPP') : '—'

  const needsDates = g.draw_frequency === 'special' || g.draw_frequency === 'monthly'
  const drawDate = g.draw_date || g.end_date

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-5xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <div className="flex items-center justify-between">
            <div>
              <DialogTitle className="text-2xl">{g.name}</DialogTitle>
              <DialogDescription className="mt-2">
                Game Code: {g.code} • Version: {g.version ?? 1}
              </DialogDescription>
            </div>
            <div className="flex items-center gap-2">
              {getStatusBadge(g.status)}
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
              <FileText className="h-4 w-4" />
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

          {/* ── Overview ── */}
          <TabsContent value="overview" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Competition Information</CardTitle>
                <CardDescription>Basic details about this competition</CardDescription>
              </CardHeader>
              <CardContent>
                {gameLoading && !freshGame ? (
                  <div className="text-sm text-muted-foreground py-4">Loading…</div>
                ) : (
                  <>
                    <div className="grid grid-cols-2 gap-6">
                      <div className="space-y-4">
                        <div>
                          <p className="text-sm text-muted-foreground">Draw Frequency</p>
                          <p className="font-medium">{freqLabel(g.draw_frequency)}</p>
                        </div>
                        {needsDates && (
                          <>
                            <div>
                              <p className="text-sm text-muted-foreground">Draw Date</p>
                              <p className="font-medium">{fmtDate(drawDate)}</p>
                            </div>
                          </>
                        )}
                        <div>
                          <p className="text-sm text-muted-foreground">Created</p>
                          <p className="font-medium">{fmtDate(g.created_at)}</p>
                        </div>
                      </div>
                      <div className="space-y-4">
                        <div>
                          <p className="text-sm text-muted-foreground">Sales Cutoff</p>
                          <p className="font-medium">{g.sales_cutoff_minutes} minutes before draw</p>
                        </div>
                        <div>
                          <p className="text-sm text-muted-foreground">Total Tickets</p>
                          <p className="font-medium">{g.total_tickets?.toLocaleString() ?? '—'}</p>
                        </div>
                        <div>
                          <p className="text-sm text-muted-foreground">Tickets Sold</p>
                          <p className="font-medium">{g.sold_tickets?.toLocaleString() ?? '0'}</p>
                        </div>
                        <div>
                          <p className="text-sm text-muted-foreground">Last Updated</p>
                          <p className="font-medium">{fmtDate(g.updated_at)}</p>
                        </div>
                      </div>
                    </div>
                    {g.description && (
                      <>
                        <Separator className="my-4" />
                        <div>
                          <p className="text-sm text-muted-foreground mb-1">Description</p>
                          <p className="text-sm">{g.description}</p>
                        </div>
                      </>
                    )}
                  </>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Performance</CardTitle>
                <CardDescription>Ticket sales metrics</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-3 gap-4">
                  <div className="text-center">
                    <div className="text-2xl font-bold">{g.sold_tickets ?? 0}</div>
                    <p className="text-xs text-muted-foreground">Tickets Sold</p>
                  </div>
                  <div className="text-center">
                    <div className="text-2xl font-bold">{g.total_tickets ?? '—'}</div>
                    <p className="text-xs text-muted-foreground">Total Available</p>
                  </div>
                  <div className="text-center">
                    <div className="text-2xl font-bold">
                      ₵{((g.sold_tickets ?? 0) * (g.base_price ?? 0)).toFixed(2)}
                    </div>
                    <p className="text-xs text-muted-foreground">Revenue</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* ── Rules ── */}
          <TabsContent value="rules" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Competition Rules</CardTitle>
                <CardDescription>Terms and conditions for participants</CardDescription>
              </CardHeader>
              <CardContent>
                {g.rules ? (
                  <p className="text-sm whitespace-pre-wrap">{g.rules}</p>
                ) : (
                  <p className="text-sm text-muted-foreground">No rules configured for this competition.</p>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          {/* ── Pricing ── */}
          <TabsContent value="pricing" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Pricing & Limits</CardTitle>
                <CardDescription>Ticket pricing and purchase limits</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 gap-6">
                  <div className="space-y-4">
                    <div>
                      <p className="text-sm text-muted-foreground">Ticket Price</p>
                      <p className="text-2xl font-bold">
                        ₵{(g.base_price ?? g.ticket_price ?? 0).toFixed(2)}
                      </p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Max Tickets per Player</p>
                      <p className="text-lg font-medium">{g.max_tickets_per_player}</p>
                    </div>
                  </div>
                  <div className="space-y-4">
                    <div>
                      <p className="text-sm text-muted-foreground">Total Tickets Available</p>
                      <p className="text-lg font-medium">{g.total_tickets?.toLocaleString() ?? '—'}</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Total Revenue Potential</p>
                      <p className="text-lg font-medium">
                        ₵{((g.total_tickets ?? 0) * (g.base_price ?? 0)).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                      </p>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* ── Schedule ── */}
          <TabsContent value="schedule" className="space-y-4">
            <Card>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle>Draw Schedule</CardTitle>
                    <CardDescription>Timing configuration and upcoming draws</CardDescription>
                  </div>
                  <Button size="sm" onClick={onManageSchedule}>
                    <Calendar className="mr-2 h-4 w-4" />
                    Manage Schedule
                  </Button>
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <p className="text-sm text-muted-foreground">Frequency</p>
                    <p className="font-medium">{freqLabel(g.draw_frequency)}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Draw Time</p>
                    <p className="font-medium">{g.draw_time || 'Not set'}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Sales Cutoff</p>
                    <p className="font-medium">{g.sales_cutoff_minutes} min before draw</p>
                  </div>
                </div>

                {needsDates && (
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-sm text-muted-foreground">Draw Date</p>
                      <p className="font-medium">{fmtDate(drawDate)}</p>
                    </div>
                  </div>
                )}

                {g.draw_days && g.draw_days.length > 0 && (
                  <div>
                    <p className="text-sm text-muted-foreground mb-2">Draw Days</p>
                    <div className="flex gap-2 flex-wrap">
                      {g.draw_days.map(day => (
                        <Badge key={day} variant="secondary">
                          {day.charAt(0).toUpperCase() + day.slice(1)}
                        </Badge>
                      ))}
                    </div>
                  </div>
                )}

                <Separator />

                <div>
                  <p className="font-medium mb-3">Scheduled Draws</p>
                  {gameSchedules.length === 0 ? (
                    <p className="text-sm text-muted-foreground">No draws scheduled yet. Use Manage Schedule to generate them.</p>
                  ) : (
                    <div className="space-y-2">
                      {gameSchedules.slice(0, 5).map(s => {
                        const drawTime = typeof s.scheduled_draw === 'string'
                          ? s.scheduled_draw
                          : new Date((s.scheduled_draw as { seconds: number }).seconds * 1000).toISOString()
                        return (
                          <div key={s.id} className="flex items-center justify-between p-3 rounded-lg border">
                            <div>
                              <p className="text-sm font-medium">{formatInGhanaTime(drawTime, 'PPP')}</p>
                              <p className="text-xs text-muted-foreground">{formatInGhanaTime(drawTime, 'p')} Ghana time</p>
                            </div>
                            <Badge variant={s.is_active ? 'default' : 'secondary'}>
                              {s.status || (s.is_active ? 'Scheduled' : 'Inactive')}
                            </Badge>
                          </div>
                        )
                      })}
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* ── Prizes ── */}
          <TabsContent value="prizes" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Prize Structure</CardTitle>
                <CardDescription>Competition prizes by rank</CardDescription>
              </CardHeader>
              <CardContent>
                {!g.prize_details || g.prize_details.length === 0 ? (
                  <div className="text-center py-8">
                    <Trophy className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                    <p className="text-muted-foreground">No prizes configured for this competition.</p>
                    <Button variant="outline" className="mt-4" onClick={onEdit}>
                      Edit Competition to Add Prizes
                    </Button>
                  </div>
                ) : (
                  <div className="space-y-3">
                    {g.prize_details.map(prize => (
                      <div key={prize.rank} className="flex items-start gap-4 p-3 rounded-lg border">
                        <div className="flex items-center justify-center w-10 h-10 rounded-full bg-primary/10 text-primary font-bold shrink-0">
                          #{prize.rank}
                        </div>
                        <div>
                          <p className="font-medium">{prize.label}</p>
                          {prize.description && (
                            <p className="text-sm text-muted-foreground">{prize.description}</p>
                          )}
                        </div>
                      </div>
                    ))}
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
