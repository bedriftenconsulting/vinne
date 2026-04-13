import React, { useState, useRef, useEffect } from 'react'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

interface NumberInputSlotsProps {
  value: number[]
  onChange: (numbers: number[]) => void
  onValidationChange?: (isValid: boolean, hasDuplicates: boolean) => void
  disabled?: boolean
  className?: string
  slotCount?: number
  min?: number
  max?: number
  placeholder?: string
}

export function NumberInputSlots({
  value,
  onChange,
  onValidationChange,
  disabled = false,
  className,
  slotCount = 5,
  min = 1,
  max = 90,
  placeholder = '00',
}: NumberInputSlotsProps) {
  const [inputValues, setInputValues] = useState<string[]>(
    Array.from({ length: slotCount }, (_, i) => value[i]?.toString() || '')
  )
  const inputRefs = useRef<(HTMLInputElement | null)[]>([])

  useEffect(() => {
    setInputValues(Array.from({ length: slotCount }, (_, i) => value[i]?.toString() || ''))
  }, [value, slotCount])

  const handleInputChange = (index: number, inputValue: string) => {
    // Only allow numeric input
    const numericValue = inputValue.replace(/[^0-9]/g, '')

    // Limit to 2 digits max
    const limitedValue = numericValue.slice(0, 2)

    const newInputValues = [...inputValues]
    newInputValues[index] = limitedValue
    setInputValues(newInputValues)

    // Convert to numbers and validate range
    const numbers: number[] = []
    for (let i = 0; i < slotCount; i++) {
      const val = newInputValues[i]
      if (val && val !== '') {
        const num = parseInt(val, 10)
        if (num >= min && num <= max) {
          numbers[i] = num
        }
      }
    }

    // Filter out undefined values and check for duplicates
    const validNumbers = numbers.filter(n => n !== undefined)
    const uniqueNumbers = [...new Set(validNumbers)]
    const hasDuplicatesInArray = validNumbers.length !== uniqueNumbers.length

    // Check if all filled slots have valid numbers (range and no duplicates)
    const allValidNumbers =
      validNumbers.length > 0 &&
      validNumbers.every(n => n >= min && n <= max) &&
      !hasDuplicatesInArray

    // Notify parent about validation state
    onValidationChange?.(allValidNumbers, hasDuplicatesInArray)

    // Always call onChange to keep parent state updated
    onChange(validNumbers)

    // Auto-advance to next input
    if (limitedValue.length === 2 && index < slotCount - 1) {
      const nextInput = inputRefs.current[index + 1]
      if (nextInput) {
        nextInput.focus()
        nextInput.select()
      }
    }
  }

  const handleKeyDown = (index: number, event: React.KeyboardEvent) => {
    // Handle backspace - move to previous input if current is empty
    if (event.key === 'Backspace' && inputValues[index] === '' && index > 0) {
      const prevInput = inputRefs.current[index - 1]
      if (prevInput) {
        prevInput.focus()
        prevInput.select()
      }
    }

    // Handle arrow keys
    if (event.key === 'ArrowLeft' && index > 0) {
      event.preventDefault()
      inputRefs.current[index - 1]?.focus()
    }
    if (event.key === 'ArrowRight' && index < slotCount - 1) {
      event.preventDefault()
      inputRefs.current[index + 1]?.focus()
    }
  }

  const handleFocus = (event: React.FocusEvent<HTMLInputElement>) => {
    event.target.select()
  }

  const isValidNumber = (index: number): boolean => {
    const val = inputValues[index]
    if (!val || val === '') return true // Empty is valid (not filled yet)
    const num = parseInt(val, 10)
    return num >= min && num <= max
  }

  const hasDuplicate = (index: number): boolean => {
    const val = inputValues[index]
    if (!val || val === '') return false // Empty slots can't be duplicates

    const num = parseInt(val, 10)
    if (isNaN(num)) return false

    // Check if this number appears in any other slot
    return inputValues.some((otherVal, otherIndex) => {
      if (otherIndex === index) return false // Don't compare with self
      if (!otherVal || otherVal === '') return false
      const otherNum = parseInt(otherVal, 10)
      return !isNaN(otherNum) && otherNum === num
    })
  }

  const getValidationMessage = (index: number): string => {
    const val = inputValues[index]
    if (!val || val === '') return ''

    if (hasDuplicate(index)) return 'Duplicate'
    if (!isValidNumber(index)) return `${min}-${max}`

    return ''
  }

  return (
    <div className={cn('flex gap-3', className)}>
      {Array.from({ length: slotCount }, (_, index) => (
        <div key={index} className="relative">
          <Input
            ref={el => {
              inputRefs.current[index] = el
            }}
            type="text"
            inputMode="numeric"
            pattern="[0-9]*"
            maxLength={2}
            value={inputValues[index]}
            onChange={e => handleInputChange(index, e.target.value)}
            onKeyDown={e => handleKeyDown(index, e)}
            onFocus={handleFocus}
            disabled={disabled}
            placeholder={placeholder}
            className={cn(
              'h-16 w-16 text-center text-lg font-bold rounded-xl border-2 transition-all duration-200',
              'focus:ring-2 focus:ring-blue-500 focus:border-blue-500',
              (hasDuplicate(index) || !isValidNumber(index)) && 'border-red-500 bg-red-50',
              inputValues[index] &&
                isValidNumber(index) &&
                !hasDuplicate(index) &&
                'border-green-500 bg-green-50',
              disabled && 'opacity-50 cursor-not-allowed'
            )}
          />
          {getValidationMessage(index) && (
            <div className="absolute -bottom-6 left-0 right-0 text-xs text-red-500 text-center">
              {getValidationMessage(index)}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}

export default NumberInputSlots
