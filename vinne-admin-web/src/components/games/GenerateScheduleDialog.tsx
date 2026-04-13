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
import { CalendarPlus } from 'lucide-react'
import { startOfWeek } from 'date-fns'
import { formatInGhanaTime } from '@/lib/date-utils'
import { gameService } from '@/services/games'
import { toast } from '@/hooks/use-toast'

interface GenerateScheduleDialogProps {
  isOpen: boolean
  onClose: () => void
  selectedWeek: Date
}

export function GenerateScheduleDialog({
  isOpen,
  onClose,
  selectedWeek,
}: GenerateScheduleDialogProps) {
  const queryClient = useQueryClient()

  // Generate game schedule mutation
  const generateScheduleMutation = useMutation({
    mutationFn: () => {
      const weekStart = startOfWeek(selectedWeek, { weekStartsOn: 0 }) // Sunday
      return gameService.generateWeeklySchedule(formatInGhanaTime(weekStart, 'yyyy-MM-dd'))
    },
    onSuccess: response => {
      queryClient.invalidateQueries({ queryKey: ['draws'] })
      queryClient.invalidateQueries({ queryKey: ['gameSchedules'] })
      toast({
        title: 'Game Schedule Generated',
        description: `Successfully generated ${response.schedules_created} game schedules for the selected week.`,
      })
      onClose()
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to generate schedule. Please try again.',
        variant: 'destructive',
      })
    },
  })

  const confirmGenerateSchedule = () => {
    generateScheduleMutation.mutate()
  }

  // Get week display text
  const getWeekDisplayText = () => {
    const weekStart = startOfWeek(selectedWeek, { weekStartsOn: 0 }) // Sunday (week starts on Sunday in Ghana)
    return formatInGhanaTime(weekStart, 'MMM d, yyyy')
  }

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Generate Game Schedule</DialogTitle>
          <DialogDescription>
            This will generate game schedules for the selected week ({getWeekDisplayText()}) based
            on active games and their frequency settings.
            <br />
            <br />
            The system will:
            <br />
            • Fetch all active games from the game service
            <br />
            • Check each game's frequency (daily, weekly, bi-weekly, etc.)
            <br />
            • Generate appropriate schedule times based on draw days and times
            <br />
            • Calculate sales start/end times and cutoff periods
            <br />
            <br />
            Are you sure you want to generate schedules for this week?
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            onClick={confirmGenerateSchedule}
            disabled={generateScheduleMutation.isPending}
            className="flex items-center gap-2"
          >
            {generateScheduleMutation.isPending ? (
              <>
                <CalendarPlus className="h-4 w-4 animate-spin" />
                Generating...
              </>
            ) : (
              <>
                <CalendarPlus className="h-4 w-4" />
                Generate Schedule
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
