import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { CalendarPlus, Loader2 } from 'lucide-react'
import { format, startOfMonth, endOfMonth, addWeeks, getDay, addDays, parseISO, startOfWeek, isAfter } from 'date-fns'
import { gameService, type Game } from '@/services/games'
import { toast } from '@/hooks/use-toast'

// Get all Sunday-starting weeks that overlap with a given month
function getWeekStartsForMonth(month: Date): Date[] {
  const monthStart = startOfMonth(month)
  const monthEnd = endOfMonth(month)
  const weeks: Date[] = []
  // Start from the Sunday of the week containing the first day of the month
  let current = startOfWeek(monthStart, { weekStartsOn: 0 })
  while (!isAfter(current, monthEnd)) {
    weeks.push(new Date(current))
    current = addWeeks(current, 1)
  }
  return weeks
}

interface GenerateScheduleDialogProps {
  isOpen: boolean
  onClose: () => void
  selectedMonth: Date
}

const DAYS_OF_WEEK = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

function formatDrawTime(time?: string) {
  return time ? `at ${time}` : ''
}

// Given a game, compute the draw dates it will have in the given month
function getDrawDatesForGame(game: Game, month: Date): { date: Date; label: string }[] {
  const monthStart = startOfMonth(month)
  const monthEnd = endOfMonth(month)

  const gameStart = game.start_date ? parseISO(game.start_date) : null
  const gameEnd = game.end_date ? parseISO(game.end_date) : null

  // If game has explicit dates, check overlap; otherwise include it (backend schedules all active games)
  if (gameStart && gameEnd) {
    const overlaps = gameStart <= monthEnd && gameEnd >= monthStart
    if (!overlaps) return []
  }

  const timeStr = formatDrawTime(game.draw_time)
  const freq = game.draw_frequency?.toLowerCase()

  // Daily: 7 draws per week (schedule is generated week by week)
  if (freq === 'daily') {
    const draws: { date: Date; label: string }[] = []
    const weekStart = startOfWeek(monthStart, { weekStartsOn: 0 })
    for (let i = 0; i < 7; i++) {
      const current = addDays(weekStart, i)
      draws.push({ date: current, label: `${format(current, 'EEE, MMM d')}${timeStr ? ' ' + timeStr : ''}` })
    }
    return draws
  }

  // Weekly or Bi-weekly: one draw per configured draw day per week
  if (freq === 'weekly' || freq === 'bi_weekly') {
    const drawDayNames = game.draw_days?.length ? game.draw_days : ['Friday']
    const draws: { date: Date; label: string }[] = []

    for (const dayName of drawDayNames) {
      const targetDay = DAYS_OF_WEEK.findIndex(d => d.toLowerCase() === dayName.toLowerCase())
      const safeTarget = targetDay === -1 ? 5 : targetDay

      let current = new Date(monthStart)
      const daysUntil = (safeTarget - getDay(current) + 7) % 7
      current = addDays(current, daysUntil)

      let week = 1
      while (current <= monthEnd && current.getMonth() === month.getMonth()) {
        if ((!gameStart || current >= gameStart) && (!gameEnd || current <= gameEnd)) {
          draws.push({
            date: new Date(current),
            label: `Week ${week} — ${format(current, 'EEE, MMM d')}${timeStr ? ' ' + timeStr : ''}`,
          })
        }
        current = addWeeks(current, freq === 'bi_weekly' ? 2 : 1)
        week++
      }
    }

    // Sort by date
    draws.sort((a, b) => a.date.getTime() - b.date.getTime())
    return draws
  }

  // Monthly / special — exactly ONE draw for the month
  // Use game's configured draw day if set, otherwise last Saturday of the month
  let drawDate: Date
  if (game.draw_days?.length) {
    const targetDay = DAYS_OF_WEEK.findIndex(d => d.toLowerCase() === game.draw_days![0].toLowerCase())
    const safeTarget = targetDay === -1 ? 6 : targetDay // default Saturday
    let d = new Date(monthEnd)
    while (d.getDay() !== safeTarget) d = addDays(d, -1)
    drawDate = d
  } else {
    // Last Saturday of the month
    let d = new Date(monthEnd)
    while (d.getDay() !== 6) d = addDays(d, -1)
    drawDate = d
  }
  if (gameEnd && gameEnd <= monthEnd && gameEnd >= monthStart) drawDate = gameEnd
  const freqLabel = freq === 'special' ? 'Special Draw' : 'Monthly Draw'
  const label = `${format(drawDate, 'EEE, MMM d')}${timeStr ? ' ' + timeStr : ''} — ${freqLabel}`
  return [{ date: drawDate, label }]
}

export function GenerateScheduleDialog({ isOpen, onClose, selectedMonth }: GenerateScheduleDialogProps) {
  const queryClient = useQueryClient()
  const monthLabel = format(selectedMonth, 'MMMM yyyy')

  // Fetch active games
  const { data: gamesData, isLoading: gamesLoading } = useQuery({
    queryKey: ['games'],
    queryFn: () => gameService.getGames(1, 1000),
    enabled: isOpen,
  })

  // Filter to active games only (backend schedules all active games regardless of start/end date)
  const activeGames = (gamesData?.data || []).filter((g: Game) => {
    return g.status?.toLowerCase() === 'active'
  })

  // Build preview: each active game + its draws for the CURRENT WEEK only
  const currentWeekStart = startOfWeek(new Date(), { weekStartsOn: 0 })
  const currentWeekEnd = addDays(currentWeekStart, 6)
  
  const preview = activeGames.map((game: Game) => {
    const allDraws = getDrawDatesForGame(game, selectedMonth)
    // Filter to only draws that fall in the current week
    const currentWeekDraws = allDraws.filter(d => 
      d.date >= currentWeekStart && d.date <= currentWeekEnd
    )
    return { game, draws: currentWeekDraws }
  }).filter(({ draws }) => draws.length > 0)

  const generateMutation = useMutation({
    mutationFn: async () => {
      // Generate schedule for the current week only (Sunday-Saturday)
      const currentWeekStart = startOfWeek(new Date(), { weekStartsOn: 0 })
      await gameService.generateWeeklySchedule(format(currentWeekStart, 'yyyy-MM-dd'))
      return { weeks: 1 }
    },
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      queryClient.invalidateQueries({ queryKey: ['gameSchedules'] })
      queryClient.invalidateQueries({ queryKey: ['draws'] })
      toast({
        title: 'Schedule Generated',
        description: `Schedules created for ${result.weeks} week${result.weeks !== 1 ? 's' : ''} in ${monthLabel}.`,
      })
      onClose()
    },
    onError: () => {
      toast({ title: 'Error', description: 'Failed to generate schedule.', variant: 'destructive' })
    },
  })

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Generate Schedule — Current Week</DialogTitle>
          <DialogDescription>
            Creates draw schedules for all active competitions for the current week (Sunday–Saturday).
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {gamesLoading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
            </div>
          ) : activeGames.length === 0 ? (
            <div className="rounded-lg border bg-muted/30 p-6 text-center">
              <p className="text-sm font-medium text-foreground">No active games found</p>
              <p className="text-xs text-muted-foreground mt-1">
                Activate games before generating a schedule.
              </p>
            </div>
          ) : (
            <div className="rounded-lg border bg-muted/30 p-4 space-y-4">
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                Schedule Preview — {monthLabel}
              </p>
              {preview.map(({ game, draws }) => (
                <div key={game.id} className="space-y-1.5">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">{game.name}</span>
                    <span className="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
                      {game.draw_frequency === 'daily' ? 'Daily Draw'
                        : game.draw_frequency === 'weekly' ? 'Weekly Draw'
                        : game.draw_frequency === 'bi_weekly' ? 'Bi-Weekly Draw'
                        : game.draw_frequency === 'special' ? 'Special (Once)'
                        : 'Monthly (Once)'}
                    </span>
                  </div>
                  <ul className="space-y-1 pl-2">
                    {draws.map((d, i) => (
                      <li key={i} className="flex items-center gap-2 text-sm text-muted-foreground">
                        <span className="h-1.5 w-1.5 rounded-full bg-primary shrink-0" />
                        {d.label}
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button
            onClick={() => generateMutation.mutate()}
            disabled={generateMutation.isPending || activeGames.length === 0}
            className="flex items-center gap-2"
          >
            {generateMutation.isPending ? (
              <><Loader2 className="h-4 w-4 animate-spin" />Generating...</>
            ) : (
              <><CalendarPlus className="h-4 w-4" />Generate Schedule</>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
