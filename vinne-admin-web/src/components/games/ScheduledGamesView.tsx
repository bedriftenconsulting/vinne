import { useState, useMemo } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { CalendarPlus, ChevronLeft, ChevronRight, Search, Edit } from 'lucide-react'
import {
  startOfWeek,
  endOfWeek,
  addWeeks,
  isSameWeek,
  format,
  parseISO,
} from 'date-fns'
import { gameService, type GameSchedule, type Game } from '@/services/games'
import { GenerateScheduleDialog } from './GenerateScheduleDialog'
import { getPublicUrl } from '@/lib/utils'

// Parse proto timestamp or ISO string
function parseTs(ts: string | { seconds: number; nanos?: number } | undefined): Date | null {
  if (!ts) return null
  if (typeof ts === 'string') return parseISO(ts)
  return new Date(ts.seconds * 1000)
}

const STATUS_CLASS: Record<string, string> = {
  SCHEDULED:   'bg-blue-100 text-blue-700',
  IN_PROGRESS: 'bg-green-100 text-green-700',
  COMPLETED:   'bg-gray-100 text-gray-600',
  CANCELLED:   'bg-red-100 text-red-600',
  FAILED:      'bg-orange-100 text-orange-700',
}

const FREQ_LABEL: Record<string, string> = {
  daily:    'Daily',
  weekly:   'Weekly',
  bi_weekly:'Bi-Weekly',
  monthly:  'Monthly',
  special:  'Special',
}

interface ScheduledGamesViewProps {
  onEditSchedule?: (schedule: GameSchedule) => void
}

export function ScheduledGamesView({ onEditSchedule }: ScheduledGamesViewProps) {
  const [weekStart, setWeekStart] = useState(() => startOfWeek(new Date(), { weekStartsOn: 0 }))
  const [searchTerm, setSearchTerm] = useState('')
  const [isGenerateOpen, setIsGenerateOpen] = useState(false)
  const queryClient = useQueryClient()

  const weekEnd = endOfWeek(weekStart, { weekStartsOn: 0 })
  const weekLabel = `${format(weekStart, 'MMM d')} – ${format(weekEnd, 'MMM d, yyyy')}`
  const isCurrentWeek = isSameWeek(weekStart, new Date(), { weekStartsOn: 0 })

  const { data: schedules = [], isLoading } = useQuery({
    queryKey: ['gameSchedules', format(weekStart, 'yyyy-MM-dd')],
    queryFn: () => gameService.getWeeklySchedule(format(weekStart, 'yyyy-MM-dd')),
  })

  const { data: gamesData } = useQuery({
    queryKey: ['games-list'],
    queryFn: () => gameService.getGames(1, 1000),
  })

  const suspendedGameIds = useMemo(() => {
    const games: Game[] = gamesData?.data || []
    return new Set(
      games
        .filter(g => g.status?.toUpperCase() === 'SUSPENDED')
        .map(g => g.id)
    )
  }, [gamesData])

  const filtered = useMemo(() => {
    const nonSuspended = schedules.filter((s: GameSchedule) => !suspendedGameIds.has(s.game_id))
    if (!searchTerm.trim()) return nonSuspended
    const lower = searchTerm.toLowerCase()
    return nonSuspended.filter((s: GameSchedule) =>
      s.game_name?.toLowerCase().includes(lower) ||
      s.game_id?.toLowerCase().includes(lower) ||
      s.notes?.toLowerCase().includes(lower) ||
      s.game_code?.toLowerCase().includes(lower)
    )
  }, [schedules, searchTerm, suspendedGameIds])

  // Sort by scheduled_draw ascending
  const sorted = useMemo(() =>
    [...filtered].sort((a, b) => {
      const ta = parseTs(a.scheduled_draw)?.getTime() ?? 0
      const tb = parseTs(b.scheduled_draw)?.getTime() ?? 0
      return ta - tb
    }), [filtered])

  return (
    <div className="space-y-5">
      {/* Week Navigation */}
      <div className="flex items-center justify-between gap-4">
        <Button
          variant="outline"
          size="sm"
          onClick={() => setWeekStart(w => addWeeks(w, -1))}
        >
          <ChevronLeft className="h-4 w-4 mr-1" />
          Previous Week
        </Button>

        <div className="text-center">
          <p className="text-base font-semibold text-foreground">{weekLabel}</p>
          <p className="text-xs text-muted-foreground mt-0.5">
            {isCurrentWeek ? 'Current Week' : format(weekStart, 'yyyy')}
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setIsGenerateOpen(true)}
            className="flex items-center gap-1.5"
          >
            <CalendarPlus className="h-3.5 w-3.5" />
            Generate Schedule
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setWeekStart(w => addWeeks(w, 1))}
          >
            Next Week
            <ChevronRight className="h-4 w-4 ml-1" />
          </Button>
        </div>
      </div>

      {/* Search */}
      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground h-3.5 w-3.5" />
        <Input
          placeholder="Search games by name, ID, or notes..."
          value={searchTerm}
          onChange={e => setSearchTerm(e.target.value)}
          className="pl-9 h-9 text-sm"
        />
      </div>

      {/* Heading with count */}
      {!isLoading && (
        <div className="flex items-center gap-2">
          <p className="text-sm font-semibold text-foreground">Generated Game Schedules</p>
          <span className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
            {sorted.length} schedule{sorted.length !== 1 ? 's' : ''}
          </span>
        </div>
      )}

      {/* Schedule Cards */}
      {isLoading ? (
        <div className="flex justify-center items-center h-32">
          <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary" />
        </div>
      ) : sorted.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center bg-card rounded-lg border">
          <CalendarPlus className="h-10 w-10 text-muted-foreground mb-3" />
          <p className="text-sm font-medium text-foreground">No schedules for this week</p>
          <p className="text-xs text-muted-foreground mt-1 max-w-xs">
            Click Generate Schedule to create draw slots for active games.
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {sorted.map((schedule: GameSchedule) => {
            const drawTime = parseTs(schedule.scheduled_draw)
            const salesStart = parseTs(schedule.scheduled_start)
            const salesEnd = parseTs(schedule.scheduled_end)
            const statusKey = schedule.status?.toUpperCase() ?? 'SCHEDULED'
            const statusClass = STATUS_CLASS[statusKey] ?? 'bg-muted text-muted-foreground'
            const statusLabel = statusKey.charAt(0) + statusKey.slice(1).toLowerCase().replace('_', ' ')
            const freqLabel = FREQ_LABEL[schedule.frequency?.toLowerCase() ?? ''] ?? schedule.frequency ?? 'Draw'

            return (
              <div
                key={schedule.id}
                className="bg-card rounded-xl border p-4 shadow-sm hover:shadow-md transition-shadow duration-150"
              >
                {/* Top row: name + status + next draw + edit */}
                <div className="flex items-start justify-between gap-3 mb-3">
                  <div className="flex items-center gap-2 flex-wrap min-w-0">
                    {getPublicUrl(schedule.logo_url) ? (
                      <img
                        src={getPublicUrl(schedule.logo_url)}
                        alt={schedule.game_name}
                        className="h-7 w-7 rounded object-contain shrink-0 border border-border"
                        onError={e => { (e.target as HTMLImageElement).style.display = 'none' }}
                      />
                    ) : schedule.brand_color ? (
                      <div
                        className="h-7 w-7 rounded shrink-0 border border-border"
                        style={{ backgroundColor: schedule.brand_color }}
                      />
                    ) : null}
                    <span className="text-sm font-bold text-foreground truncate">
                      {schedule.game_name || schedule.game_code || schedule.game_id?.slice(0, 8)}
                    </span>
                    <Badge className={`text-[11px] px-2 py-0.5 font-medium border-0 rounded-full ${statusClass}`}>
                      {statusLabel}
                    </Badge>
                    {drawTime && (
                      <span className="text-xs text-muted-foreground">
                        Next Draw:&nbsp;
                        <span className="font-medium text-foreground">
                          {format(drawTime, 'EEE, MMM d, yyyy')} · {format(drawTime, 'h:mm a')}
                        </span>
                      </span>
                    )}
                  </div>
                  {onEditSchedule && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => onEditSchedule(schedule)}
                      className="h-7 px-2.5 text-xs shrink-0 flex items-center gap-1"
                    >
                      <Edit className="h-3 w-3" />
                      Edit
                    </Button>
                  )}
                </div>

                {/* Game ID + draw date */}
                <p className="text-xs text-muted-foreground mb-3">
                  Game ID: <span className="font-mono">{schedule.game_id?.slice(0, 8)}</span>
                  {drawTime && (
                    <> · {format(drawTime, 'EEE, MMM d, yyyy')} · {format(drawTime, 'h:mm a')}</>
                  )}
                </p>

                {/* Grid: Sales Start | Sales End | Frequency | Schedule Enabled */}
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                  <div>
                    <p className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground mb-0.5">Sales Start</p>
                    <p className="text-xs text-foreground">
                      {salesStart ? format(salesStart, 'MMM d, h:mm a') : '—'}
                    </p>
                  </div>
                  <div>
                    <p className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground mb-0.5">Sales End</p>
                    <p className="text-xs text-foreground">
                      {salesEnd ? format(salesEnd, 'MMM d, h:mm a') : '—'}
                    </p>
                  </div>
                  <div>
                    <p className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground mb-0.5">Frequency</p>
                    <p className="text-xs text-foreground">{freqLabel}</p>
                  </div>
                  <div>
                    <p className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground mb-0.5">Schedule Enabled</p>
                    <Badge className={`text-[10px] px-1.5 border-0 ${schedule.is_active ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-600'}`}>
                      {schedule.is_active ? 'Active' : 'Inactive'}
                    </Badge>
                  </div>
                </div>

                {/* Notes */}
                {schedule.notes && (
                  <div className="mt-3 pt-3 border-t">
                    <p className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground mb-0.5">Notes</p>
                    <p className="text-xs text-muted-foreground">{schedule.notes}</p>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      <GenerateScheduleDialog
        isOpen={isGenerateOpen}
        onClose={() => {
          setIsGenerateOpen(false)
          queryClient.invalidateQueries({ queryKey: ['gameSchedules', format(weekStart, 'yyyy-MM-dd')] })
        }}
        selectedMonth={weekStart}
      />
    </div>
  )
}
