import { useState } from 'react'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Check } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ColorPickerProps {
  value?: string
  onChange: (value: string) => void
  disabled?: boolean
}

const PRESET_COLORS = [
  '#FF6B6B',
  '#4ECDC4',
  '#45B7D1',
  '#FFA07A',
  '#98D8C8',
  '#F7DC6F',
  '#BB8FCE',
  '#85C1E2',
  '#F8B739',
  '#52B788',
  '#E74C3C',
  '#3498DB',
  '#9B59B6',
  '#1ABC9C',
  '#F39C12',
  '#D35400',
  '#C0392B',
  '#2980B9',
]

export function ColorPicker({ value = '#3498DB', onChange, disabled }: ColorPickerProps) {
  const [customColor, setCustomColor] = useState(value)

  const handleColorChange = (color: string) => {
    setCustomColor(color)
    onChange(color)
  }

  const isValidHex = (color: string) => {
    return /^#[0-9A-Fa-f]{6}$/.test(color)
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          className={cn(
            'w-full justify-start text-left font-normal',
            !value && 'text-muted-foreground'
          )}
          disabled={disabled}
        >
          <div className="flex items-center gap-2 w-full">
            <div
              className="h-4 w-4 rounded border border-gray-300"
              style={{ backgroundColor: value || '#FFFFFF' }}
            />
            <span className="flex-1">{value || 'Select color'}</span>
          </div>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80">
        <div className="space-y-4">
          <div>
            <Label className="text-sm font-medium mb-2 block">Preset Colors</Label>
            <div className="grid grid-cols-6 gap-2">
              {PRESET_COLORS.map(color => (
                <button
                  key={color}
                  type="button"
                  className={cn(
                    'h-8 w-8 rounded border-2 transition-all hover:scale-110',
                    value === color
                      ? 'border-primary ring-2 ring-primary ring-offset-2'
                      : 'border-gray-300'
                  )}
                  style={{ backgroundColor: color }}
                  onClick={() => handleColorChange(color)}
                  title={color}
                >
                  {value === color && <Check className="h-4 w-4 text-white mx-auto" />}
                </button>
              ))}
            </div>
          </div>

          <div>
            <Label htmlFor="custom-color" className="text-sm font-medium mb-2 block">
              Custom Color
            </Label>
            <div className="flex gap-2">
              <Input
                id="custom-color"
                type="text"
                value={customColor}
                onChange={e => setCustomColor(e.target.value.toUpperCase())}
                placeholder="#000000"
                maxLength={7}
                className={cn(!isValidHex(customColor) && customColor !== '' && 'border-red-500')}
              />
              <Button
                type="button"
                size="sm"
                onClick={() => {
                  if (isValidHex(customColor)) {
                    handleColorChange(customColor)
                  }
                }}
                disabled={!isValidHex(customColor)}
              >
                Apply
              </Button>
            </div>
            {!isValidHex(customColor) && customColor !== '' && (
              <p className="text-xs text-red-500 mt-1">Invalid hex color format (use #RRGGBB)</p>
            )}
          </div>

          <div>
            <Label htmlFor="color-picker-native" className="text-sm font-medium mb-2 block">
              Color Picker
            </Label>
            <input
              id="color-picker-native"
              type="color"
              value={value}
              onChange={e => handleColorChange(e.target.value.toUpperCase())}
              className="h-10 w-full rounded border border-gray-300 cursor-pointer"
            />
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}
