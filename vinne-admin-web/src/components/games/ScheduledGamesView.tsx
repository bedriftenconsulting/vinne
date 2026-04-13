import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Calendar, CalendarPlus, Edit as EditIcon, Search } from 'lucide-react'
import { startOfWeek, endOfWeek, addWeeks, isSameWeek } from 'date-fns'
import { toZonedTime } from 'date-fns-tz'
import { gameService, type GameSchedule } from '@/services/games'
import { formatInGhanaTime, GHANA_TIMEZONE, protoTimestampToDate } from '@/lib/date-utils'

interface ScheduledGamesViewProps {
  onGenerateSchedule: (selectedWeek: Date) => void
  onEditSchedule: (schedule: GameSchedule) => void
}

export function ScheduledGamesView({
  onGenerateSchedule,
  onEditSchedule,
}: ScheduledGamesViewProps) {
  const [selectedWeek, setSelectedWeek] = useState(new Date())
  const [searchTerm, setSearchTerm] = useState('')

  // Fetch game schedules for the selected week
  const { data: gameSchedules, isLoading: schedulesLoading } = useQuery({
    queryKey: ['gameSchedules', selectedWeek],
    queryFn: async () => {
      const weekStart = startOfWeek(selectedWeek, { weekStartsOn: 0 }) // Sunday (week starts on Sunday in Ghana)
      const weekStartStr = formatInGhanaTime(weekStart, 'yyyy-MM-dd')
      try {
        const schedules = await gameService.getWeeklySchedule(weekStartStr)
        return schedules
      } catch (error) {
        console.error('Error fetching game schedules:', error)
        return []
      }
    },
  })

  // Helper function to convert timestamp object to Date (use utility)
  const convertTimestamp = protoTimestampToDate

  // Get game schedules for the selected week, sorted by next draw date
  const getWeekGameSchedules = () => {
    if (!gameSchedules) return []

    const weekStart = startOfWeek(selectedWeek, { weekStartsOn: 0 }) // Sunday (week starts on Sunday in Ghana)
    const weekEnd = endOfWeek(selectedWeek, { weekStartsOn: 0 }) // Saturday
    const now = toZonedTime(new Date(), GHANA_TIMEZONE)
    const searchLower = searchTerm.toLowerCase()

    return gameSchedules
      .filter((schedule: GameSchedule) => {
        const scheduleDate = convertTimestamp(schedule.scheduled_draw)
        const inWeekRange = scheduleDate >= weekStart && scheduleDate <= weekEnd

        // If no search term, only apply week filter
        if (!searchTerm.trim()) return inWeekRange

        // Apply search filter
        const matchesSearch =
          schedule.game_name?.toLowerCase().includes(searchLower) ||
          schedule.game_id?.toLowerCase().includes(searchLower) ||
          schedule.id?.toLowerCase().includes(searchLower) ||
          schedule.notes?.toLowerCase().includes(searchLower)

        return inWeekRange && matchesSearch
      })
      .sort((a: GameSchedule, b: GameSchedule) => {
        const dateA = convertTimestamp(a.scheduled_draw)
        const dateB = convertTimestamp(b.scheduled_draw)

        // Prioritize upcoming draws (future dates first)
        if (dateA >= now && dateB < now) return -1
        if (dateA < now && dateB >= now) return 1

        // For both future or both past, sort by date (ascending for future, descending for past)
        if (dateA >= now && dateB >= now) {
          return dateA.getTime() - dateB.getTime() // Soonest first
        } else {
          return dateB.getTime() - dateA.getTime() // Most recent first
        }
      })
  }

  // Get badge styling based on status
  const getStatusBadgeClass = (status?: string) => {
    switch (status?.toUpperCase()) {
      case 'CANCELLED':
        return 'bg-red-100 text-red-800'
      case 'COMPLETED':
        return 'bg-blue-100 text-blue-800'
      case 'IN_PROGRESS':
        return 'bg-yellow-100 text-yellow-800'
      case 'FAILED':
        return 'bg-gray-100 text-gray-800'
      case 'SCHEDULED':
      default:
        return 'bg-green-100 text-green-800'
    }
  }

  // Get descriptive status text
  const getStatusText = (status?: string) => {
    switch (status?.toUpperCase()) {
      case 'CANCELLED':
        return 'Cancelled'
      case 'COMPLETED':
        return 'Draw Completed'
      case 'IN_PROGRESS':
        return 'Draw in Progress'
      case 'FAILED':
        return 'Draw Failed'
      case 'SCHEDULED':
        return 'Scheduled'
      default:
        return status || 'Scheduled'
    }
  }

  // Format frequency for display
  const formatFrequency = (frequency: string) => {
    const frequencyMap: Record<string, string> = {
      daily: 'Daily',
      weekly: 'Weekly',
      bi_weekly: 'Bi-Weekly',
      monthly: 'Monthly',
      special: 'Special',
      // Handle legacy values
      DAILY: 'Daily',
      WEEKLY: 'Weekly',
      ONCE: 'One-Time',
      ONE_TIME: 'One-Time',
    }
    return frequencyMap[frequency] || frequency
  }

  // Navigate week
  const navigateWeek = (direction: 'prev' | 'next') => {
    setSelectedWeek(prev => addWeeks(prev, direction === 'next' ? 1 : -1))
  }

  // Get week display text
  const getWeekDisplayText = () => {
    const weekStart = startOfWeek(selectedWeek, { weekStartsOn: 0 }) // Sunday (week starts on Sunday in Ghana)
    const weekEnd = endOfWeek(selectedWeek, { weekStartsOn: 0 }) // Saturday
    return `${formatInGhanaTime(weekStart, 'MMM d')} - ${formatInGhanaTime(weekEnd, 'MMM d, yyyy')}`
  }

  return (
    <div className="space-y-6">
      {/* Week Navigation */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
        <Button variant="outline" onClick={() => navigateWeek('prev')} className="sm:w-auto">
          ← Previous Week
        </Button>
        <div className="text-center">
          <h3 className="text-lg font-semibold whitespace-nowrap">{getWeekDisplayText()}</h3>
          <p className="text-sm text-muted-foreground whitespace-nowrap">
            {isSameWeek(selectedWeek, toZonedTime(new Date(), GHANA_TIMEZONE))
              ? 'Current Week'
              : formatInGhanaTime(selectedWeek, 'yyyy')}
          </p>
        </div>
        <div className="flex flex-col sm:flex-row gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => onGenerateSchedule(selectedWeek)}
            className="flex items-center justify-center gap-2"
          >
            <CalendarPlus className="h-4 w-4" />
            <span className="hidden sm:inline">Generate Schedule</span>
            <span className="sm:hidden">Generate</span>
          </Button>
          <Button variant="outline" onClick={() => navigateWeek('next')} className="sm:w-auto">
            Next Week →
          </Button>
        </div>
      </div>

      {/* Search Bar */}
      <div className="relative max-w-md">
        <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 h-4 w-4" />
        <Input
          placeholder="Search games by name, ID, or notes..."
          value={searchTerm}
          onChange={e => setSearchTerm(e.target.value)}
          className="pl-10"
        />
      </div>

      {/* Weekly Schedule */}
      <div className="space-y-4">
        {schedulesLoading ? (
          <div className="flex justify-center items-center h-32">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
          </div>
        ) : getWeekGameSchedules().length > 0 ? (
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <h4 className="text-lg font-semibold">Generated Game Schedules</h4>
              <Badge variant="secondary">{getWeekGameSchedules().length} schedules</Badge>
            </div>
            <div className="grid gap-4">
              {getWeekGameSchedules().map((schedule: GameSchedule) => {
                const scheduleDate = convertTimestamp(schedule.scheduled_draw)
                const now = toZonedTime(new Date(), GHANA_TIMEZONE)
                const isUpcoming = scheduleDate >= now
                const twoHoursFromNow = new Date(now.getTime() + 2 * 60 * 60 * 1000)
                const isNextDraw = scheduleDate >= now && scheduleDate <= twoHoursFromNow
                const cardBgColor = isUpcoming
                  ? 'bg-blue-50 hover:bg-blue-100 border-blue-200'
                  : 'bg-gray-50 hover:bg-gray-100 border-gray-200'

                return (
                  <div
                    key={schedule.id}
                    className={`border rounded-lg p-3 sm:p-4 space-y-3 transition-colors ${cardBgColor}`}
                  >
                    <div className="flex flex-col sm:flex-row items-start sm:justify-between gap-3">
                      <div className="space-y-1 min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <Link
                            to="/schedule/$scheduleId"
                            params={{ scheduleId: schedule.id }}
                            className={`text-base sm:text-lg font-semibold ${isUpcoming ? 'text-blue-700 hover:text-blue-900' : 'text-gray-700 hover:text-gray-900'} hover:underline break-words`}
                          >
                            {isNextDraw && '🔔 '}
                            {schedule.game_name || `Game Schedule #${schedule.id.slice(0, 8)}`}
                          </Link>
                          <Badge className={getStatusBadgeClass(schedule.status)}>
                            {getStatusText(schedule.status)}
                          </Badge>
                          {isNextDraw && (
                            <Badge className="bg-blue-100 text-blue-800">Next Draw</Badge>
                          )}
                        </div>
                        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs sm:text-sm text-muted-foreground">
                          <span className="whitespace-nowrap">
                            Game ID: {schedule.game_id.slice(0, 8)}
                          </span>
                          <span className="hidden sm:inline">•</span>
                          <span className="whitespace-nowrap">
                            {formatInGhanaTime(
                              convertTimestamp(schedule.scheduled_draw),
                              'EEE, MMM d, yyyy'
                            )}
                          </span>
                          <span className="hidden sm:inline">•</span>
                          <span className="whitespace-nowrap">
                            {formatInGhanaTime(convertTimestamp(schedule.scheduled_draw), 'h:mm a')}
                          </span>
                        </div>
                      </div>
                      {schedule.status?.toUpperCase() !== 'COMPLETED' &&
                        schedule.status?.toUpperCase() !== 'IN_PROGRESS' && (
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => onEditSchedule(schedule)}
                            className="flex items-center gap-1 self-start sm:self-auto shrink-0"
                          >
                            <EditIcon className="h-4 w-4" />
                            <span className="hidden sm:inline">Edit</span>
                          </Button>
                        )}
                    </div>

                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3 sm:gap-4 text-xs sm:text-sm">
                      <div className="min-w-0">
                        <span className="text-muted-foreground block">Sales Start:</span>
                        <div className="font-medium break-words">
                          {formatInGhanaTime(
                            convertTimestamp(schedule.scheduled_start),
                            'MMM d, h:mm a'
                          )}
                        </div>
                      </div>
                      <div className="min-w-0">
                        <span className="text-muted-foreground block">Sales End:</span>
                        <div className="font-medium break-words">
                          {formatInGhanaTime(
                            convertTimestamp(schedule.scheduled_end),
                            'MMM d, h:mm a'
                          )}
                        </div>
                      </div>
                      <div className="min-w-0">
                        <span className="text-muted-foreground block">Frequency:</span>
                        <div className="font-medium">{formatFrequency(schedule.frequency)}</div>
                      </div>
                      <div className="min-w-0">
                        <span className="text-muted-foreground block">Schedule Enabled:</span>
                        <div className="font-medium mt-1">
                          {schedule.is_active ? (
                            <Badge className="bg-green-100 text-green-800">Active</Badge>
                          ) : (
                            <Badge variant="outline">Inactive</Badge>
                          )}
                        </div>
                      </div>
                    </div>
                    {schedule.notes && (
                      <div className="mt-2 min-w-0">
                        <span className="text-muted-foreground text-xs sm:text-sm">Notes:</span>
                        <p className="text-xs sm:text-sm mt-1 break-words">{schedule.notes}</p>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        ) : (
          <div className="text-center py-12 border rounded-lg border-dashed">
            <Calendar className="mx-auto h-12 w-12 text-gray-400" />
            <h3 className="mt-2 text-sm font-medium text-gray-900">No schedules this week</h3>
            <p className="mt-1 text-sm text-gray-500">
              No game schedules are available for the selected week. Try generating a schedule
              first.
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
