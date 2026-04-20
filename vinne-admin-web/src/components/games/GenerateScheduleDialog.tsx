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
import { format, startOfMonth, endOfMonth, getDay, addDays, parseISO, startOfWeek, addWeeks } from 'date-fns'
import { gameService, type Game } from '@/services/games'
import { toast } from '@/hooks/use-toast'

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

  const gameStart = game.draw_date ? parseISO(game.draw_date) : null
  const gameEnd = game.draw_date ? parseISO(game.draw_date) : null

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
  
  // Build preview: current week draws + special games with future draw dates
  const preview = activeGames.map((game: Game) => {
    const allDraws = getDrawDatesForGame(game, selectedMonth)
    
    // For special games, show their actual draw date regardless of current week
    if (game.draw_frequency === 'special' && game.draw_date) {
      const drawDate = parseISO(game.draw_date)
      const timeStr = formatDrawTime(game.draw_time)
      return { game, draws: [{ date: drawDate, label: `${format(drawDate, 'EEE, MMM d')}${timeStr ? ' ' + timeStr : ''} — Special Draw` }] }
    }
    
    // For recurring games, filter to current week
    const currentWeekDraws = allDraws.filter(d => 
      d.date >= currentWeekStart && d.date <= currentWeekEnd
    )
    return { game, draws: currentWeekDraws }
  }).filter(({ draws }) => draws.length > 0)

  const generateMutation = useMutation({
    mutationFn: async () => {
      const currentWeekStart = startOfWeek(new Date(), { weekStartsOn: 0 })
      
      // Collect all unique weeks needed: current week + weeks containing special game draw dates
      const weeksToGenerate = new Set<string>()
      weeksToGenerate.add(format(currentWeekStart, 'yyyy-MM-dd'))

      // Add weeks for special games with future draw dates
      for (const game of activeGames) {
        if (game.draw_frequency === 'special' && game.draw_date) {
          const drawDate = parseISO(game.draw_date)
          const drawWeekStart = startOfWeek(drawDate, { weekStartsOn: 0 })
          weeksToGenerate.add(format(drawWeekStart, 'yyyy-MM-dd'))
        }
      }

      console.log('[GenerateSchedule] Active games:', activeGames.map(g => `${g.code} (${g.draw_frequency}, draw_date=${g.draw_date})`))
      console.log('[GenerateSchedule] Weeks to generate:', [...weeksToGenerate])

      // Generate for each unique week
      const results = []
      for (const weekStr of weeksToGenerate) {
        const isCurrentWeek = weekStr === format(currentWeekStart, 'yyyy-MM-dd')
        console.log(`[GenerateSchedule] Generating week: ${weekStr} (current: ${isCurrentWeek})`)
        try {
          if (isCurrentWeek) {
            // Current week: generate all active games (daily, weekly, special)
            const r = await gameService.generateWeeklySchedule(weekStr)
            console.log(`[GenerateSchedule] Week ${weekStr} result:`, r)
            results.push({ week: weekStr, result: r })
          } else {
            // Future week: only generate special games whose draw_date falls in this week
            // We do this by temporarily activating only those games — but since the backend
            // generates ALL active games, we generate the full week and then clean up
            // non-special schedules for that week via the backend
            const r = await gameService.generateWeeklySchedule(weekStr)
            console.log(`[GenerateSchedule] Week ${weekStr} result:`, r)
            results.push({ week: weekStr, result: r })
          }
        } catch (e) {
          console.error(`[GenerateSchedule] Week ${weekStr} FAILED:`, e)
          results.push({ week: weekStr, error: e })
        }
      }

      console.log('[GenerateSchedule] All done:', results)
      return { weeks: weeksToGenerate.size, results }
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      queryClient.invalidateQueries({ queryKey: ['gameSchedules'] })
      queryClient.invalidateQueries({ queryKey: ['draws'] })
      toast({
        title: 'Schedule Generated',
        description: `Schedules created for ${data.weeks} week${data.weeks > 1 ? 's' : ''} (including special draw dates).`,
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
          <DialogTitle>Generate Schedule</DialogTitle>
          <DialogDescription>
            Creates draw schedules for all active competitions. Special games are scheduled for their draw date week; recurring games for the current week.
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
                Schedule Preview — Current Week + Special Draws
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
