import { useState, useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
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
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { DateTimePicker } from '@/components/ui/date-time-picker'
import { AlertCircle, Edit } from 'lucide-react'
import { formatInGhanaTime } from '@/lib/date-utils'
import { gameService, type GameSchedule } from '@/services/games'
import { toast } from '@/hooks/use-toast'

interface EditScheduleDialogProps {
  isOpen: boolean
  onClose: () => void
  schedule: GameSchedule | null
}

export function EditScheduleDialog({ isOpen, onClose, schedule }: EditScheduleDialogProps) {
  const [editFormData, setEditFormData] = useState({
    scheduled_end: '',
    scheduled_draw: '',
    status: '',
    is_active: true,
    notes: '',
  })
  const [validationError, setValidationError] = useState<string>('')
  const queryClient = useQueryClient()

  // Helper function to convert timestamp object to Date
  const convertTimestamp = (timestamp: string | { seconds: number } | Date): Date => {
    if (typeof timestamp === 'object' && 'seconds' in timestamp) {
      return new Date(timestamp.seconds * 1000)
    }
    return new Date(timestamp)
  }

  // Validate that draw time is after sales end time and cannot be the same
  const validateScheduleTimes = (scheduledEnd: string, scheduledDraw: string): string => {
    if (!scheduledEnd || !scheduledDraw) return ''

    const endTime = new Date(scheduledEnd)
    const drawTime = new Date(scheduledDraw)

    if (drawTime.getTime() === endTime.getTime()) {
      return 'Draw time cannot be the same as sales end time'
    }

    if (drawTime < endTime) {
      return 'Draw time must be after sales end time'
    }

    return ''
  }

  // Initialize form data when schedule changes
  useEffect(() => {
    if (schedule) {
      const scheduledEnd = formatInGhanaTime(
        convertTimestamp(schedule.scheduled_end),
        "yyyy-MM-dd'T'HH:mm"
      )
      const scheduledDraw = formatInGhanaTime(
        convertTimestamp(schedule.scheduled_draw),
        "yyyy-MM-dd'T'HH:mm"
      )
      setEditFormData({
        scheduled_end: scheduledEnd,
        scheduled_draw: scheduledDraw,
        status: schedule.status || 'SCHEDULED',
        is_active: schedule.is_active,
        notes: schedule.notes || '',
      })
      setValidationError('')
    }
  }, [schedule])

  // Update scheduled game mutation
  const updateScheduleMutation = useMutation({
    mutationFn: ({
      scheduleId,
      data,
    }: {
      scheduleId: string
      data: {
        scheduled_end?: string
        scheduled_draw?: string
        status?: 'SCHEDULED' | 'IN_PROGRESS' | 'COMPLETED' | 'CANCELLED' | 'FAILED'
        is_active?: boolean
        notes?: string
      }
    }) => gameService.updateScheduledGame(scheduleId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['gameSchedules'] })
      toast({
        title: 'Schedule Updated',
        description: 'The game schedule has been successfully updated.',
      })
      onClose()
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to update schedule. Please try again.',
        variant: 'destructive',
      })
    },
  })

  const confirmUpdateSchedule = () => {
    if (!schedule) return

    const updateData: {
      scheduled_end?: string
      scheduled_draw?: string
      status?: 'SCHEDULED' | 'IN_PROGRESS' | 'COMPLETED' | 'CANCELLED' | 'FAILED'
      is_active?: boolean
      notes?: string
    } = {}

    // Only include changed fields
    if (editFormData.scheduled_end) {
      updateData.scheduled_end = new Date(editFormData.scheduled_end).toISOString()
    }
    if (editFormData.scheduled_draw) {
      updateData.scheduled_draw = new Date(editFormData.scheduled_draw).toISOString()
    }
    if (editFormData.status) {
      updateData.status = editFormData.status as
        | 'SCHEDULED'
        | 'IN_PROGRESS'
        | 'COMPLETED'
        | 'CANCELLED'
        | 'FAILED'
    }
    updateData.is_active = editFormData.is_active
    if (editFormData.notes) {
      updateData.notes = editFormData.notes
    }

    updateScheduleMutation.mutate({
      scheduleId: schedule.id,
      data: updateData,
    })
  }

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Edit Game Schedule</DialogTitle>
          <DialogDescription>
            Update the schedule details for{' '}
            {schedule?.game_name || `Schedule #${schedule?.id.slice(0, 8)}`}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="scheduled-end">Sales End Time (GMT)</Label>
              <DateTimePicker
                value={editFormData.scheduled_end}
                onChange={date => {
                  const newScheduledEnd = date ? date.toISOString() : ''
                  setEditFormData(prev => ({
                    ...prev,
                    scheduled_end: newScheduledEnd,
                  }))
                  // Validate after updating
                  const error = validateScheduleTimes(newScheduledEnd, editFormData.scheduled_draw)
                  setValidationError(error)
                }}
                placeholder="Pick sales end time"
              />
              <p className="text-xs text-muted-foreground">Time is in GMT timezone</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="scheduled-draw">Draw Time (GMT)</Label>
              <DateTimePicker
                value={editFormData.scheduled_draw}
                onChange={date => {
                  const newScheduledDraw = date ? date.toISOString() : ''
                  setEditFormData(prev => ({
                    ...prev,
                    scheduled_draw: newScheduledDraw,
                  }))
                  // Validate after updating
                  const error = validateScheduleTimes(editFormData.scheduled_end, newScheduledDraw)
                  setValidationError(error)
                }}
                placeholder="Pick draw time"
              />
              <p className="text-xs text-muted-foreground">Time is in GMT timezone</p>
            </div>
          </div>
          {validationError && (
            <div className="flex items-center gap-2 text-sm text-red-600">
              <AlertCircle className="h-4 w-4" />
              <span>{validationError}</span>
            </div>
          )}

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="status">Status</Label>
              <Select
                value={editFormData.status}
                onValueChange={value => setEditFormData(prev => ({ ...prev, status: value }))}
              >
                <SelectTrigger id="status">
                  <SelectValue placeholder="Select status" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="SCHEDULED">Scheduled</SelectItem>
                  <SelectItem value="IN_PROGRESS">In Progress</SelectItem>
                  <SelectItem value="COMPLETED">Completed</SelectItem>
                  <SelectItem value="CANCELLED">Cancelled</SelectItem>
                  <SelectItem value="FAILED">Failed</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="is-active">Active Status</Label>
              <Select
                value={editFormData.is_active ? 'true' : 'false'}
                onValueChange={value =>
                  setEditFormData(prev => ({ ...prev, is_active: value === 'true' }))
                }
              >
                <SelectTrigger id="is-active">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="true">Active</SelectItem>
                  <SelectItem value="false">Inactive</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="notes">Notes</Label>
            <Textarea
              id="notes"
              placeholder="Add any notes about this schedule..."
              value={editFormData.notes}
              onChange={e => setEditFormData(prev => ({ ...prev, notes: e.target.value }))}
              rows={3}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            onClick={confirmUpdateSchedule}
            disabled={updateScheduleMutation.isPending || !!validationError}
            className="flex items-center gap-2"
          >
            {updateScheduleMutation.isPending ? (
              <>
                <Edit className="h-4 w-4 animate-spin" />
                Updating...
              </>
            ) : (
              <>
                <Edit className="h-4 w-4" />
                Update Schedule
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
