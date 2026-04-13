import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  format,
  startOfWeek,
  endOfWeek,
  eachDayOfInterval,
  isSameDay,
  addWeeks,
  subWeeks,
  startOfMonth,
  endOfMonth,
  isToday,
  isFuture,
} from 'date-fns'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { useToast } from '@/hooks/use-toast'
import {
  ChevronLeft,
  ChevronRight,
  Clock,
  Plus,
  Trash2,
  AlertCircle,
  CheckCircle,
  XCircle,
  Loader2,
  RefreshCw,
} from 'lucide-react'
import { gameService, type Game, type DrawSchedule } from '@/services/games'

interface GameCalendarProps {
  game?: Game
  viewMode?: 'week' | 'month'
}

interface ScheduleForm {
  game_id: string
  draw_date: string
  draw_time: string
  sales_cutoff_time: string
  status: 'Scheduled' | 'InProgress' | 'Completed' | 'Cancelled'
  is_special: boolean
  special_name?: string
}

const getScheduleStatusColor = (status: string) => {
  switch (status.toLowerCase()) {
    case 'scheduled':
      return 'bg-blue-100 text-blue-800 border-blue-200'
    case 'inprogress':
      return 'bg-yellow-100 text-yellow-800 border-yellow-200'
    case 'completed':
      return 'bg-green-100 text-green-800 border-green-200'
    case 'cancelled':
      return 'bg-red-100 text-red-800 border-red-200'
    default:
      return 'bg-gray-100 text-gray-800 border-gray-200'
  }
}

const getScheduleStatusIcon = (status: string) => {
  switch (status.toLowerCase()) {
    case 'scheduled':
      return <Clock className="h-3 w-3" />
    case 'inprogress':
      return <RefreshCw className="h-3 w-3 animate-spin" />
    case 'completed':
      return <CheckCircle className="h-3 w-3" />
    case 'cancelled':
      return <XCircle className="h-3 w-3" />
    default:
      return <AlertCircle className="h-3 w-3" />
  }
}

export function GameCalendar({ game, viewMode = 'week' }: GameCalendarProps) {
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const [currentDate, setCurrentDate] = useState(new Date())
  const [, setSelectedDate] = useState<Date | null>(null)
  const [isScheduleDialogOpen, setIsScheduleDialogOpen] = useState(false)
  const [editingSchedule, setEditingSchedule] = useState<DrawSchedule | null>(null)
  const [selectedGame, setSelectedGame] = useState<string>(game?.id || '')

  const [scheduleForm, setScheduleForm] = useState<ScheduleForm>({
    game_id: game?.id || '',
    draw_date: '',
    draw_time: '19:00',
    sales_cutoff_time: '18:30',
    status: 'Scheduled',
    is_special: false,
    special_name: '',
  })

  // Fetch all games if no specific game is provided
  const { data: gamesData } = useQuery({
    queryKey: ['games'],
    queryFn: () => gameService.getGames(),
    enabled: !game,
  })

  const games = game ? [game] : gamesData?.data || []

  // Calculate date range based on view mode
  const dateRange =
    viewMode === 'week'
      ? {
          start: startOfWeek(currentDate, { weekStartsOn: 1 }),
          end: endOfWeek(currentDate, { weekStartsOn: 1 }),
        }
      : {
          start: startOfMonth(currentDate),
          end: endOfMonth(currentDate),
        }

  // Fetch schedules for the date range
  const { data: schedules, isLoading } = useQuery({
    queryKey: ['draw-schedules', selectedGame || 'all', dateRange.start, dateRange.end],
    queryFn: () =>
      gameService.getDrawSchedules({
        game_id: selectedGame || undefined,
      }),
  })

  // Mutations
  const createScheduleMutation = useMutation({
    mutationFn: (data: ScheduleForm) => gameService.createDrawSchedule(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['draw-schedules'] })
      toast({ title: 'Success', description: 'Draw schedule created successfully' })
      setIsScheduleDialogOpen(false)
      resetForm()
    },
    onError: (error: Error) => {
      toast({
        title: 'Error',
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        description: (error as any).response?.data?.message || 'Failed to create schedule',
        variant: 'destructive',
      })
    },
  })

  const updateScheduleMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<DrawSchedule> }) =>
      gameService.updateDrawSchedule(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['draw-schedules'] })
      toast({ title: 'Success', description: 'Draw schedule updated successfully' })
      setEditingSchedule(null)
      setIsScheduleDialogOpen(false)
      resetForm()
    },
    onError: (error: Error) => {
      toast({
        title: 'Error',
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        description: (error as any).response?.data?.message || 'Failed to update schedule',
        variant: 'destructive',
      })
    },
  })

  const deleteScheduleMutation = useMutation({
    mutationFn: (id: string) => gameService.deleteDrawSchedule(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['draw-schedules'] })
      toast({ title: 'Success', description: 'Draw schedule deleted successfully' })
    },
    onError: (error: Error) => {
      toast({
        title: 'Error',
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        description: (error as any).response?.data?.message || 'Failed to delete schedule',
        variant: 'destructive',
      })
    },
  })

  const resetForm = () => {
    setScheduleForm({
      game_id: game?.id || '',
      draw_date: '',
      draw_time: '19:00',
      sales_cutoff_time: '18:30',
      status: 'Scheduled',
      is_special: false,
      special_name: '',
    })
  }

  const handlePreviousPeriod = () => {
    if (viewMode === 'week') {
      setCurrentDate(subWeeks(currentDate, 1))
    } else {
      setCurrentDate(new Date(currentDate.getFullYear(), currentDate.getMonth() - 1))
    }
  }

  const handleNextPeriod = () => {
    if (viewMode === 'week') {
      setCurrentDate(addWeeks(currentDate, 1))
    } else {
      setCurrentDate(new Date(currentDate.getFullYear(), currentDate.getMonth() + 1))
    }
  }

  const handleToday = () => {
    setCurrentDate(new Date())
  }

  const handleDateClick = (date: Date) => {
    setSelectedDate(date)
    setScheduleForm({
      ...scheduleForm,
      draw_date: format(date, 'yyyy-MM-dd'),
    })
    setIsScheduleDialogOpen(true)
  }

  const handleEditSchedule = (schedule: DrawSchedule) => {
    setEditingSchedule(schedule)
    setScheduleForm({
      game_id: schedule.game_id,
      draw_date: format(new Date(schedule.draw_date), 'yyyy-MM-dd'),
      draw_time: schedule.draw_time,
      sales_cutoff_time: schedule.sales_cutoff_time,
      status: schedule.status,
      is_special: schedule.is_special,
      special_name: schedule.special_name,
    })
    setIsScheduleDialogOpen(true)
  }

  const handleSaveSchedule = () => {
    if (editingSchedule) {
      updateScheduleMutation.mutate({ id: editingSchedule.id, data: scheduleForm })
    } else {
      createScheduleMutation.mutate(scheduleForm)
    }
  }

  const getSchedulesForDate = (date: Date) => {
    if (!schedules) return []
    return schedules.filter((schedule: DrawSchedule) =>
      isSameDay(new Date(schedule.draw_date), date)
    )
  }

  const days = eachDayOfInterval(dateRange)

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>Game Draw Calendar</CardTitle>
            <CardDescription>
              Manage draw schedules for {game ? game.name : 'all games'}
            </CardDescription>
          </div>
          <div className="flex items-center gap-2">
            {!game && (
              <Select value={selectedGame} onValueChange={setSelectedGame}>
                <SelectTrigger className="w-[200px]">
                  <SelectValue placeholder="All Games" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">All Games</SelectItem>
                  {games.map(g => (
                    <SelectItem key={g.id} value={g.id}>
                      {g.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setSelectedDate(new Date())
                setIsScheduleDialogOpen(true)
              }}
            >
              <Plus className="mr-2 h-4 w-4" />
              Add Schedule
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {/* Calendar Navigation */}
        <div className="mb-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Button variant="outline" size="icon" onClick={handlePreviousPeriod}>
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <Button variant="outline" size="sm" onClick={handleToday}>
              Today
            </Button>
            <Button variant="outline" size="icon" onClick={handleNextPeriod}>
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
          <h3 className="text-lg font-semibold">
            {viewMode === 'week'
              ? `Week of ${format(dateRange.start, 'MMM d')} - ${format(dateRange.end, 'MMM d, yyyy')}`
              : format(currentDate, 'MMMM yyyy')}
          </h3>
          <div className="flex items-center gap-2">
            <Badge variant="outline" className="gap-1">
              <div className="h-2 w-2 rounded-full bg-blue-500" />
              Scheduled
            </Badge>
            <Badge variant="outline" className="gap-1">
              <div className="h-2 w-2 rounded-full bg-yellow-500" />
              In Progress
            </Badge>
            <Badge variant="outline" className="gap-1">
              <div className="h-2 w-2 rounded-full bg-green-500" />
              Completed
            </Badge>
          </div>
        </div>

        {/* Calendar Grid */}
        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : viewMode === 'week' ? (
          <div className="grid grid-cols-7 gap-2">
            {/* Week Day Headers */}
            {['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'].map(day => (
              <div key={day} className="text-center text-sm font-medium text-muted-foreground py-2">
                {day}
              </div>
            ))}

            {/* Week Days */}
            {days.map(day => {
              const daySchedules = getSchedulesForDate(day)
              const isCurrentDay = isToday(day)
              const isFutureDay = isFuture(day)

              return (
                <div
                  key={day.toISOString()}
                  className={`min-h-[120px] rounded-lg border p-2 ${
                    isCurrentDay ? 'border-primary bg-primary/5' : 'border-border'
                  } ${isFutureDay ? 'cursor-pointer hover:bg-accent' : 'bg-muted/30'}`}
                  onClick={() => isFutureDay && handleDateClick(day)}
                >
                  <div className="mb-2 flex items-center justify-between">
                    <span className={`text-sm font-medium ${isCurrentDay ? 'text-primary' : ''}`}>
                      {format(day, 'd')}
                    </span>
                    {isFutureDay && (
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6"
                        onClick={e => {
                          e.stopPropagation()
                          handleDateClick(day)
                        }}
                      >
                        <Plus className="h-3 w-3" />
                      </Button>
                    )}
                  </div>

                  <div className="space-y-1">
                    {daySchedules.slice(0, 3).map(schedule => (
                      <div
                        key={schedule.id}
                        className={`group relative rounded px-1 py-0.5 text-xs ${getScheduleStatusColor(schedule.status)} cursor-pointer`}
                        onClick={e => {
                          e.stopPropagation()
                          handleEditSchedule(schedule)
                        }}
                      >
                        <div className="flex items-center gap-1">
                          {getScheduleStatusIcon(schedule.status)}
                          <span className="truncate">
                            {schedule.draw_time} -{' '}
                            {games.find(g => g.id === schedule.game_id)?.name || 'Unknown'}
                          </span>
                        </div>
                        {schedule.is_special && (
                          <Badge
                            variant="secondary"
                            className="absolute -top-1 -right-1 h-4 px-1 text-[10px]"
                          >
                            Special
                          </Badge>
                        )}
                      </div>
                    ))}
                    {daySchedules.length > 3 && (
                      <div className="text-xs text-muted-foreground text-center">
                        +{daySchedules.length - 3} more
                      </div>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        ) : (
          // Month View
          <div className="grid grid-cols-7 gap-1">
            {/* Month Day Headers */}
            {['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'].map(day => (
              <div key={day} className="text-center text-sm font-medium text-muted-foreground py-2">
                {day}
              </div>
            ))}

            {/* Month Days */}
            {days.map(day => {
              const daySchedules = getSchedulesForDate(day)
              const isCurrentDay = isToday(day)
              const isFutureDay = isFuture(day)
              const isCurrentMonth = day.getMonth() === currentDate.getMonth()

              return (
                <div
                  key={day.toISOString()}
                  className={`min-h-[80px] rounded border p-1 ${
                    isCurrentDay ? 'border-primary bg-primary/5' : 'border-border'
                  } ${!isCurrentMonth ? 'opacity-50' : ''} ${
                    isFutureDay && isCurrentMonth ? 'cursor-pointer hover:bg-accent' : 'bg-muted/30'
                  }`}
                  onClick={() => isFutureDay && isCurrentMonth && handleDateClick(day)}
                >
                  <div className="mb-1 flex items-center justify-between">
                    <span className={`text-xs font-medium ${isCurrentDay ? 'text-primary' : ''}`}>
                      {format(day, 'd')}
                    </span>
                    {daySchedules.length > 0 && (
                      <Badge variant="secondary" className="h-4 px-1 text-[10px]">
                        {daySchedules.length}
                      </Badge>
                    )}
                  </div>

                  <div className="space-y-0.5">
                    {daySchedules.slice(0, 2).map(schedule => (
                      <div
                        key={schedule.id}
                        className={`h-1.5 w-full rounded-sm ${
                          schedule.status === 'Completed'
                            ? 'bg-green-500'
                            : schedule.status === 'InProgress'
                              ? 'bg-yellow-500'
                              : 'bg-blue-500'
                        }`}
                        title={`${schedule.draw_time} - ${games.find(g => g.id === schedule.game_id)?.name}`}
                      />
                    ))}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </CardContent>

      {/* Add/Edit Schedule Dialog */}
      <Dialog
        open={isScheduleDialogOpen}
        onOpenChange={open => {
          setIsScheduleDialogOpen(open)
          if (!open) {
            setEditingSchedule(null)
            resetForm()
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingSchedule ? 'Edit Draw Schedule' : 'Add Draw Schedule'}
            </DialogTitle>
            <DialogDescription>
              {editingSchedule
                ? 'Update the draw schedule details'
                : 'Schedule a new draw for the selected date'}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Game</Label>
              <Select
                value={scheduleForm.game_id}
                onValueChange={value => setScheduleForm({ ...scheduleForm, game_id: value })}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select a game" />
                </SelectTrigger>
                <SelectContent>
                  {games.map(g => (
                    <SelectItem key={g.id} value={g.id}>
                      {g.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Draw Date</Label>
                <Input
                  type="date"
                  value={scheduleForm.draw_date}
                  onChange={e => setScheduleForm({ ...scheduleForm, draw_date: e.target.value })}
                />
              </div>

              <div className="space-y-2">
                <Label>Draw Time</Label>
                <Input
                  type="time"
                  value={scheduleForm.draw_time}
                  onChange={e => setScheduleForm({ ...scheduleForm, draw_time: e.target.value })}
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label>Sales Cutoff Time</Label>
              <Input
                type="time"
                value={scheduleForm.sales_cutoff_time}
                onChange={e =>
                  setScheduleForm({ ...scheduleForm, sales_cutoff_time: e.target.value })
                }
              />
              <p className="text-xs text-muted-foreground">
                Time when ticket sales will stop for this draw
              </p>
            </div>

            <div className="flex items-center justify-between rounded-lg border p-4">
              <div className="space-y-0.5">
                <Label>Special Draw</Label>
                <p className="text-xs text-muted-foreground">
                  Mark this as a special draw with increased prizes
                </p>
              </div>
              <Switch
                checked={scheduleForm.is_special}
                onCheckedChange={checked =>
                  setScheduleForm({ ...scheduleForm, is_special: checked })
                }
              />
            </div>

            {scheduleForm.is_special && (
              <div className="space-y-2">
                <Label>Special Draw Name</Label>
                <Input
                  placeholder="e.g., Christmas Special, New Year Bonanza"
                  value={scheduleForm.special_name || ''}
                  onChange={e => setScheduleForm({ ...scheduleForm, special_name: e.target.value })}
                />
              </div>
            )}
          </div>

          <DialogFooter>
            {editingSchedule && (
              <Button
                variant="destructive"
                onClick={() => {
                  deleteScheduleMutation.mutate(editingSchedule.id)
                  setIsScheduleDialogOpen(false)
                  setEditingSchedule(null)
                }}
              >
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            )}
            <div className="flex-1" />
            <Button variant="outline" onClick={() => setIsScheduleDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={handleSaveSchedule}
              disabled={createScheduleMutation.isPending || updateScheduleMutation.isPending}
            >
              {createScheduleMutation.isPending || updateScheduleMutation.isPending ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Saving...
                </>
              ) : editingSchedule ? (
                'Update Schedule'
              ) : (
                'Create Schedule'
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}
