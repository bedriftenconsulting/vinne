import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ArrowLeft, Calendar, Clock, DollarSign, Trophy, Ticket } from 'lucide-react'
import { gameService } from '@/services/games'
import { ticketService } from '@/services/tickets'
import { formatInGhanaTime } from '@/lib/date-utils'

export const Route = createFileRoute('/schedule/$scheduleId')({
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  beforeLoad: ({ context }: any) => {
    // Check if user is authenticated
    if (!context.auth?.isAuthenticated) {
      throw redirect({
        to: '/login',
        search: {
          redirect: `/games/schedule/$scheduleId`,
        },
      })
    }
  },
  component: ScheduledGameDetails,
})

function ScheduledGameDetails() {
  const { scheduleId } = Route.useParams()
  const navigate = useNavigate()

  // Fetch the scheduled game
  const {
    data: schedule,
    isLoading: scheduleLoading,
    error: scheduleError,
  } = useQuery({
    queryKey: ['gameSchedule', scheduleId],
    queryFn: () => gameService.getScheduleById(scheduleId),
  })

  // Fetch the game details
  const { data: game, isLoading: gameLoading } = useQuery({
    queryKey: ['game', schedule?.game_id],
    queryFn: () => gameService.getGame(schedule!.game_id),
    enabled: !!schedule?.game_id,
  })

  // Fetch tickets for this schedule
  const { data: ticketsData, isLoading: ticketsLoading } = useQuery({
    queryKey: ['tickets', scheduleId],
    queryFn: () => ticketService.getTickets({ schedule_id: scheduleId, limit: 100 }),
    enabled: !!scheduleId,
  })

  // Helper function to convert timestamp object to Date
  const convertTimestamp = (
    timestamp: string | { seconds: number } | Date | null | undefined
  ): Date | null => {
    if (!timestamp) return null
    if (typeof timestamp === 'object' && 'seconds' in timestamp) {
      return new Date(timestamp.seconds * 1000)
    }
    const date = new Date(timestamp)
    return isNaN(date.getTime()) ? null : date
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

  if (scheduleLoading || gameLoading) {
    return (
      <AdminLayout>
        <div className="p-6 space-y-6">
          <div className="flex justify-center items-center h-64">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
          </div>
        </div>
      </AdminLayout>
    )
  }

  if (scheduleError || !schedule) {
    return (
      <AdminLayout>
        <div className="p-6">
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Calendar className="h-12 w-12 text-gray-400 mb-4" />
              <h3 className="text-lg font-semibold text-gray-900">Scheduled Game Not Found</h3>
              <p className="text-sm text-gray-500 mb-4">
                The scheduled game you're looking for doesn't exist or you don't have access to it.
              </p>
              <Button onClick={() => navigate({ to: '/games' })}>
                <ArrowLeft className="h-4 w-4 mr-2" />
                Back to Games
              </Button>
            </CardContent>
          </Card>
        </div>
      </AdminLayout>
    )
  }

  return (
    <AdminLayout>
      <div className="p-6 space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <div className="flex items-center gap-2 mb-2">
              <Button variant="ghost" size="sm" onClick={() => navigate({ to: '/games' })}>
                <ArrowLeft className="h-4 w-4 mr-2" />
                Back to Games
              </Button>
            </div>
            <h1 className="text-3xl font-bold tracking-tight">
              {schedule.game_name || `Game Schedule #${schedule.id.slice(0, 8)}`}
            </h1>
            <p className="text-muted-foreground">Scheduled Game Details</p>
          </div>
          <div className="flex gap-2">
            <Badge className={getStatusBadgeClass(schedule.status)}>
              {getStatusText(schedule.status)}
            </Badge>
            {schedule.is_active ? (
              <Badge className="bg-green-100 text-green-800">Active</Badge>
            ) : (
              <Badge variant="outline">Inactive</Badge>
            )}
          </div>
        </div>

        {/* Schedule Details Cards */}
        <div className="grid gap-6 md:grid-cols-2">
          {/* Scheduled Dates & Times */}
          <Card>
            <CardHeader>
              <CardTitle>Schedule Information</CardTitle>
              <CardDescription>Draw timing and sales window</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-start gap-3">
                <Calendar className="h-5 w-5 text-muted-foreground mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm font-medium text-muted-foreground">
                    Scheduled Draw Date & Time
                  </p>
                  <p className="text-lg font-semibold">
                    {convertTimestamp(schedule.scheduled_draw)
                      ? formatInGhanaTime(
                          convertTimestamp(schedule.scheduled_draw)!,
                          'EEEE, MMMM d, yyyy'
                        )
                      : '-'}
                  </p>
                  <p className="text-lg font-semibold">
                    {convertTimestamp(schedule.scheduled_draw)
                      ? formatInGhanaTime(convertTimestamp(schedule.scheduled_draw)!, 'h:mm a')
                      : '-'}
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <Clock className="h-5 w-5 text-muted-foreground mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm font-medium text-muted-foreground">Sales Start</p>
                  <p className="text-lg font-semibold">
                    {convertTimestamp(schedule.scheduled_start)
                      ? formatInGhanaTime(
                          convertTimestamp(schedule.scheduled_start)!,
                          'MMM d, yyyy h:mm a'
                        )
                      : '-'}
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <Clock className="h-5 w-5 text-muted-foreground mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm font-medium text-muted-foreground">Sales Cutoff / End</p>
                  <p className="text-lg font-semibold">
                    {convertTimestamp(schedule.scheduled_end)
                      ? formatInGhanaTime(
                          convertTimestamp(schedule.scheduled_end)!,
                          'MMM d, yyyy h:mm a'
                        )
                      : '-'}
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <Calendar className="h-5 w-5 text-muted-foreground mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm font-medium text-muted-foreground">Frequency</p>
                  <p className="text-lg font-semibold capitalize">{schedule.frequency}</p>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Game Information */}
          {game && (
            <Card>
              <CardHeader>
                <CardTitle>Game Information</CardTitle>
                <CardDescription>Details about the lottery game</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">Game Code</p>
                    <p className="text-lg font-semibold">{game.code}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">Category</p>
                    <p className="text-lg font-semibold capitalize">{game.game_category}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">Format</p>
                    <p className="text-lg font-semibold">
                      {game.game_format?.replace(/_/g, '/').toUpperCase()}
                    </p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">Number Range</p>
                    <p className="text-lg font-semibold">
                      {game.number_range_min}-{game.number_range_max}
                    </p>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <DollarSign className="h-5 w-5 text-muted-foreground mt-0.5" />
                  <div className="flex-1">
                    <p className="text-sm font-medium text-muted-foreground">Base Price</p>
                    <p className="text-lg font-semibold">
                      GH₵ {(game.base_price || game.min_stake || 0).toFixed(2)}
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}
        </div>

        {/* Notes Section */}
        {schedule.notes && (
          <Card>
            <CardHeader>
              <CardTitle>Notes</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm">{schedule.notes}</p>
            </CardContent>
          </Card>
        )}

        {/* Tickets Section */}
        <Card>
          <CardHeader>
            <CardTitle>Tickets Purchased</CardTitle>
            <CardDescription>
              {ticketsData
                ? `${ticketsData.total_count} ticket${ticketsData.total_count !== 1 ? 's' : ''} sold for this scheduled game`
                : 'Loading tickets...'}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {ticketsLoading ? (
              <div className="flex justify-center items-center py-12">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
              </div>
            ) : ticketsData && ticketsData.data.length > 0 ? (
              <div className="space-y-4">
                <div className="rounded-lg border overflow-hidden">
                  <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-gray-50">
                      <tr>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Ticket #
                        </th>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Numbers
                        </th>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Bet Type
                        </th>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Amount
                        </th>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Status
                        </th>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Purchase Date
                        </th>
                      </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                      {ticketsData.data.map(ticket => (
                        <tr key={ticket.id} className="hover:bg-gray-50">
                          <td className="px-4 py-4 whitespace-nowrap">
                            <div className="flex items-center">
                              <Ticket className="h-4 w-4 text-gray-400 mr-2" />
                              <span className="text-sm font-medium text-gray-900">
                                {ticket.serial_number}
                              </span>
                            </div>
                          </td>
                          <td className="px-4 py-4">
                            <div className="flex flex-col gap-2">
                              {(() => {
                                // Extract banker numbers from bet_lines
                                const bankerNumbers =
                                  ticket.bet_lines
                                    ?.flatMap(line => line.banker || [])
                                    .filter((num, idx, arr) => arr.indexOf(num) === idx) || []

                                // Extract opposed numbers from bet_lines
                                const opposedNumbers =
                                  ticket.bet_lines
                                    ?.flatMap(line => line.opposed || [])
                                    .filter((num, idx, arr) => arr.indexOf(num) === idx) || []

                                const hasBanker = bankerNumbers.length > 0
                                const hasOpposed = opposedNumbers.length > 0
                                const hasSelected =
                                  ticket.selected_numbers && ticket.selected_numbers.length > 0

                                return (
                                  <>
                                    {hasBanker && (
                                      <div className="flex flex-wrap gap-1 items-center">
                                        <span className="text-xs font-medium text-gray-600 mr-1">
                                          Banker:
                                        </span>
                                        {bankerNumbers.map((num, idx) => (
                                          <span
                                            key={idx}
                                            className="inline-flex items-center justify-center w-7 h-7 rounded-full bg-green-100 text-green-800 text-xs font-semibold border border-green-300"
                                          >
                                            {num}
                                          </span>
                                        ))}
                                      </div>
                                    )}
                                    {hasOpposed && (
                                      <div className="flex flex-wrap gap-1 items-center">
                                        <span className="text-xs font-medium text-gray-600 mr-1">
                                          Opposed:
                                        </span>
                                        {opposedNumbers.map((num, idx) => (
                                          <span
                                            key={idx}
                                            className="inline-flex items-center justify-center w-7 h-7 rounded-full bg-red-100 text-red-800 text-xs font-semibold border border-red-300"
                                          >
                                            {num}
                                          </span>
                                        ))}
                                      </div>
                                    )}
                                    {hasSelected && ticket.selected_numbers && (
                                      <div className="flex flex-wrap gap-1 items-center">
                                        {hasBanker && (
                                          <span className="text-xs font-medium text-gray-600 mr-1">
                                            Selected:
                                          </span>
                                        )}
                                        {ticket.selected_numbers.map((num, idx) => (
                                          <span
                                            key={idx}
                                            className="inline-flex items-center justify-center w-7 h-7 rounded-full bg-blue-100 text-blue-800 text-xs font-semibold"
                                          >
                                            {num}
                                          </span>
                                        ))}
                                      </div>
                                    )}
                                    {!hasSelected && !hasBanker && !hasOpposed && (
                                      <span className="text-sm text-gray-400">-</span>
                                    )}
                                  </>
                                )
                              })()}
                            </div>
                          </td>
                          <td className="px-4 py-4 whitespace-nowrap">
                            <div className="flex flex-wrap gap-1">
                              {ticket.bet_lines && ticket.bet_lines.length > 0 ? (
                                [...new Set(ticket.bet_lines.map(line => line.bet_type))].map(
                                  (betType, idx) => (
                                    <span
                                      key={idx}
                                      className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800 capitalize"
                                    >
                                      {betType?.toLowerCase()}
                                    </span>
                                  )
                                )
                              ) : (
                                <span className="text-sm text-gray-400">-</span>
                              )}
                            </div>
                          </td>
                          <td className="px-4 py-4 whitespace-nowrap">
                            <span className="text-sm font-medium text-gray-900">
                              GH₵ {(Number(ticket.total_amount) / 100).toFixed(2)}
                            </span>
                          </td>
                          <td className="px-4 py-4 whitespace-nowrap">
                            <Badge
                              className={
                                ticket.status === 'won'
                                  ? 'bg-green-100 text-green-800'
                                  : ticket.status === 'lost'
                                    ? 'bg-red-100 text-red-800'
                                    : ticket.status === 'cancelled'
                                      ? 'bg-gray-100 text-gray-800'
                                      : 'bg-blue-100 text-blue-800'
                              }
                            >
                              {ticket.status}
                            </Badge>
                          </td>
                          <td className="px-4 py-4 whitespace-nowrap text-sm text-gray-500">
                            {convertTimestamp(ticket.created_at)
                              ? formatInGhanaTime(
                                  convertTimestamp(ticket.created_at)!,
                                  'MMM d, yyyy h:mm a'
                                )
                              : '-'}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            ) : (
              <div className="text-center py-12 border rounded-lg border-dashed">
                <Trophy className="mx-auto h-12 w-12 text-gray-400" />
                <h3 className="mt-2 text-sm font-medium text-gray-900">No tickets yet</h3>
                <p className="mt-1 text-sm text-gray-500">
                  Tickets will appear here once players start purchasing for this scheduled draw.
                </p>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </AdminLayout>
  )
}
