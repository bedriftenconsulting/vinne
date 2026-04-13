import * as React from 'react'
import { format } from 'date-fns'
import { Calendar as CalendarIcon, Clock } from 'lucide-react'

import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface DateTimePickerProps {
  value?: Date | string
  onChange?: (date: Date | undefined) => void
  placeholder?: string
  className?: string
  disabled?: boolean
}

export function DateTimePicker({
  value,
  onChange,
  placeholder = 'Pick a date and time',
  className,
  disabled = false,
}: DateTimePickerProps) {
  const [isOpen, setIsOpen] = React.useState(false)

  const dateValue = React.useMemo(() => {
    if (!value) return undefined
    if (value instanceof Date) return value
    return new Date(value)
  }, [value])

  const [time, setTime] = React.useState(() => {
    if (dateValue) {
      return {
        hours: dateValue.getHours().toString().padStart(2, '0'),
        minutes: dateValue.getMinutes().toString().padStart(2, '0'),
      }
    }
    return { hours: '00', minutes: '00' }
  })

  React.useEffect(() => {
    if (dateValue) {
      setTime({
        hours: dateValue.getHours().toString().padStart(2, '0'),
        minutes: dateValue.getMinutes().toString().padStart(2, '0'),
      })
    }
  }, [dateValue])

  const handleDateSelect = (selectedDate: Date | undefined) => {
    if (!selectedDate) {
      onChange?.(undefined)
      return
    }

    const newDate = new Date(selectedDate)
    newDate.setHours(parseInt(time.hours) || 0)
    newDate.setMinutes(parseInt(time.minutes) || 0)
    newDate.setSeconds(0)
    newDate.setMilliseconds(0)

    onChange?.(newDate)
  }

  const handleTimeChange = (type: 'hours' | 'minutes', value: string) => {
    const numValue = parseInt(value) || 0
    const maxValue = type === 'hours' ? 23 : 59
    const clampedValue = Math.min(Math.max(0, numValue), maxValue)
    const paddedValue = clampedValue.toString().padStart(2, '0')

    const newTime = { ...time, [type]: paddedValue }
    setTime(newTime)

    if (dateValue) {
      const newDate = new Date(dateValue)
      newDate.setHours(type === 'hours' ? clampedValue : parseInt(time.hours) || 0)
      newDate.setMinutes(type === 'minutes' ? clampedValue : parseInt(time.minutes) || 0)
      newDate.setSeconds(0)
      newDate.setMilliseconds(0)

      onChange?.(newDate)
    }
  }

  return (
    <Popover open={isOpen} onOpenChange={setIsOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          className={cn(
            'w-full justify-start text-left font-normal',
            !dateValue && 'text-muted-foreground',
            className
          )}
          disabled={disabled}
        >
          <CalendarIcon className="mr-2 h-4 w-4" />
          {dateValue ? (
            <span>
              {format(dateValue, 'PPP')} at {format(dateValue, 'HH:mm')}
            </span>
          ) : (
            <span>{placeholder}</span>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-auto p-0" align="start">
        <div className="p-3 border-b">
          <Calendar mode="single" selected={dateValue} onSelect={handleDateSelect} initialFocus />
        </div>
        <div className="p-3 space-y-2">
          <Label className="text-sm font-medium flex items-center gap-2">
            <Clock className="h-4 w-4" />
            Time
          </Label>
          <div className="flex items-center gap-2">
            <div className="flex-1">
              <Label htmlFor="hours" className="text-xs text-muted-foreground">
                Hours
              </Label>
              <Input
                id="hours"
                type="number"
                min="0"
                max="23"
                value={time.hours}
                onChange={e => handleTimeChange('hours', e.target.value)}
                onBlur={e => {
                  if (!e.target.value) {
                    handleTimeChange('hours', '0')
                  }
                }}
                className="text-center"
              />
            </div>
            <span className="text-2xl font-bold mt-5">:</span>
            <div className="flex-1">
              <Label htmlFor="minutes" className="text-xs text-muted-foreground">
                Minutes
              </Label>
              <Input
                id="minutes"
                type="number"
                min="0"
                max="59"
                value={time.minutes}
                onChange={e => handleTimeChange('minutes', e.target.value)}
                onBlur={e => {
                  if (!e.target.value) {
                    handleTimeChange('minutes', '0')
                  }
                }}
                className="text-center"
              />
            </div>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}
