import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import * as z from 'zod'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Progress } from '@/components/ui/progress'
import { useToast } from '@/hooks/use-toast'
import {
  ArrowLeft,
  ArrowRight,
  Check,
  Loader2,
  Info,
  DollarSign,
  Calendar,
  Settings,
} from 'lucide-react'
import { gameService, type CreateGameRequest } from '@/services/games'

const betTypeSchema = z.object({
  id: z.string(),
  name: z.string(),
  enabled: z.boolean(),
  multiplier: z.number().min(1).max(10000),
})

const gameSchema = z.object({
  // Step 1: Basic Information
  code: z.string().min(2, 'Game code must be at least 2 characters').max(10),
  name: z.string().min(3, 'Game name must be at least 3 characters'),
  game_category: z.enum(['national', 'private']),
  description: z.string().optional(),

  // Step 2: Game Format & Bet Types
  format: z.enum(['5_by_90', '5_by_30', '6_by_90', '6_by_49', '4_by_90', '3_by_90']),
  number_range_min: z.number().min(1),
  number_range_max: z.number().min(1),
  selection_count: z.number().min(1).max(10),
  bet_types: z.array(betTypeSchema).min(1, 'At least one bet type must be enabled'),

  // Step 3: Pricing & Limits
  base_price: z.number().min(0.5, 'Minimum ticket price is ₵0.50').max(200),
  max_tickets_per_player: z.number().min(1),
  max_tickets_per_transaction: z.number().min(1),
  multi_draw_enabled: z.boolean(),
  max_draws_advance: z.number().optional(),
  bonus_number_enabled: z.boolean().optional(),

  // Step 4: Schedule
  draw_frequency: z.enum(['daily', 'weekly', 'bi_weekly', 'monthly', 'special']),
  draw_days: z.array(z.string()).optional(),
  draw_time: z
    .string()
    .min(1, 'Draw time is required')
    .regex(/^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/, 'Draw time must be in HH:MM format (e.g., 14:30)'),
  sales_cutoff_minutes: z.number().min(5).max(1440),
  start_time: z
    .string()
    .optional()
    .refine(
      val => !val || /^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/.test(val),
      'Start time must be in HH:MM format (e.g., 08:00)'
    ),
  end_time: z
    .string()
    .optional()
    .refine(
      val => !val || /^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/.test(val),
      'End time must be in HH:MM format (e.g., 20:00)'
    ),
  status: z.enum(['Draft', 'PendingApproval', 'Active', 'Suspended', 'Archived']).optional(),

  // Legacy fields for backward compatibility
  organizer: z.enum(['nla', 'rand_lottery']).optional(),
  game_format: z.string().optional(),
  game_type: z.string().optional(),
  min_stake: z.number().optional(),
  max_stake: z.number().optional(),
  weekly_schedule: z.boolean().optional(),
})

type GameFormData = z.infer<typeof gameSchema>
type BetType = z.infer<typeof betTypeSchema>

interface CreateGameWizardProps {
  isOpen: boolean
  onClose: () => void
}

const steps = [
  { id: 1, title: 'Basic Information', icon: Info },
  { id: 2, title: 'Format & Bet Types', icon: Settings },
  { id: 3, title: 'Pricing & Limits', icon: DollarSign },
  { id: 4, title: 'Schedule', icon: Calendar },
  { id: 5, title: 'Review & Submit', icon: Check },
]

const weekDays = [
  { value: 'monday', label: 'Monday' },
  { value: 'tuesday', label: 'Tuesday' },
  { value: 'wednesday', label: 'Wednesday' },
  { value: 'thursday', label: 'Thursday' },
  { value: 'friday', label: 'Friday' },
  { value: 'saturday', label: 'Saturday' },
  { value: 'sunday', label: 'Sunday' },
]

const availableBetTypes: BetType[] = [
  { id: 'direct_1', name: '1 Direct', enabled: true, multiplier: 40 },
  { id: 'direct_2', name: '2 Direct', enabled: true, multiplier: 240 },
  { id: 'direct_3', name: '3 Direct', enabled: true, multiplier: 1920 },
  { id: 'direct_4', name: '4 Direct', enabled: false, multiplier: 19200 },
  { id: 'direct_5', name: '5 Direct', enabled: false, multiplier: 230400 },
  { id: 'direct_6', name: '6 Direct', enabled: false, multiplier: 25000000 },
  { id: 'perm_2', name: 'Perm 2', enabled: true, multiplier: 240 },
  { id: 'perm_3', name: 'Perm 3', enabled: true, multiplier: 1920 },
  { id: 'perm_4', name: 'Perm 4', enabled: false, multiplier: 19200 },
  { id: 'perm_5', name: 'Perm 5', enabled: false, multiplier: 230400 },
  { id: 'perm_6', name: 'Perm 6', enabled: false, multiplier: 25000000 },
  { id: 'banker', name: 'Banker All', enabled: true, multiplier: 240 },
  { id: 'banker_against', name: 'Banker AG', enabled: true, multiplier: 240 },
]

const gameFormatOptions = [
  { value: '5_by_90', label: '5/90 - Pick 5 numbers from 1-90', min: 1, max: 90, selection: 5 },
  { value: '5_by_30', label: '5/30 - Pick 5 numbers from 1-30', min: 1, max: 30, selection: 5 },
  { value: '6_by_90', label: '6/90 - Pick 6 numbers from 1-90', min: 1, max: 90, selection: 6 },
  { value: '6_by_49', label: '6/49 - Pick 6 numbers from 1-49', min: 1, max: 49, selection: 6 },
  { value: '4_by_90', label: '4/90 - Pick 4 numbers from 1-90', min: 1, max: 90, selection: 4 },
  { value: '3_by_90', label: '3/90 - Pick 3 numbers from 1-90', min: 1, max: 90, selection: 3 },
]

export function CreateGameWizard({ isOpen, onClose }: CreateGameWizardProps) {
  const [currentStep, setCurrentStep] = useState(1)
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const form = useForm<GameFormData>({
    resolver: zodResolver(gameSchema),
    defaultValues: {
      code: '',
      name: '',
      game_category: 'national',
      format: '5_by_90',
      number_range_min: 1,
      number_range_max: 90,
      selection_count: 5,
      bet_types: availableBetTypes.filter(bt => bt.enabled), // Enable preselected bet types
      draw_frequency: 'daily',
      draw_time: '', // Initialize as empty string (required field, user must fill)
      sales_cutoff_minutes: 30,
      base_price: 1,
      max_tickets_per_player: 10,
      max_tickets_per_transaction: 10,
      multi_draw_enabled: false,
      bonus_number_enabled: false,
      draw_days: [],
      start_time: '', // Optional but initialize as empty
      end_time: '', // Optional but initialize as empty
      organizer: 'rand_lottery',
      game_format: '5_by_90',
      min_stake: 1,
      max_stake: 1000,
      weekly_schedule: false,
    },
  })

  const createGameMutation = useMutation({
    mutationFn: (data: CreateGameRequest) => gameService.createGame(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      toast({
        title: 'Success',
        description: 'Game created successfully and sent for approval',
      })
      onClose()
      form.reset()
      setCurrentStep(1)
    },
    onError: (error: unknown) => {
      const errorMessage =
        (error as { response?: { data?: { message?: string } } })?.response?.data?.message ||
        (error as Error)?.message ||
        'Failed to create game'
      toast({
        title: 'Error',
        description: errorMessage,
        variant: 'destructive',
      })
    },
  })

  const handleNext = async () => {
    const fieldsToValidate = getFieldsForStep(currentStep)
    const isValid = await form.trigger(fieldsToValidate as (keyof GameFormData)[])

    if (isValid) {
      if (currentStep === 5) {
        handleSubmit()
      } else {
        setCurrentStep(currentStep + 1)
      }
    }
  }

  const handleBack = () => {
    setCurrentStep(currentStep - 1)
  }

  // Helper function to convert time string to GMT
  // Note: Ghana is in GMT (UTC+0), so no actual conversion needed
  // This function validates and normalizes the time format
  const convertTimeToGMT = (timeString?: string): string | undefined => {
    // Return undefined for truly empty/missing values
    if (!timeString || timeString.trim() === '') return undefined

    // Validate format HH:MM
    const timeRegex = /^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/
    if (!timeRegex.test(timeString)) {
      console.error(`Invalid time format: ${timeString}`)
      return undefined
    }

    // Ghana is in GMT (UTC+0), so we just normalize the format to HH:MM
    const [hours, minutes] = timeString.split(':')
    const normalizedHours = hours.padStart(2, '0')
    const normalizedMinutes = minutes.padStart(2, '0')

    return `${normalizedHours}:${normalizedMinutes}`
  }

  const handleSubmit = () => {
    const formData = form.getValues()

    // Filter and format bet types for the backend
    const enabledBetTypes = formData.bet_types
      .filter(bt => bt.enabled)
      .map(bt => ({
        id: bt.id,
        name: bt.name,
        enabled: true,
        multiplier: bt.multiplier,
      }))

    const payload: CreateGameRequest = {
      code: formData.code,
      name: formData.name,
      description: formData.description,
      game_category: formData.game_category,
      format: formData.format,
      bet_types: enabledBetTypes,
      number_range_min: formData.number_range_min,
      number_range_max: formData.number_range_max,
      selection_count: formData.selection_count,
      draw_frequency: formData.draw_frequency,
      draw_days:
        formData.draw_frequency === 'weekly' || formData.draw_frequency === 'bi_weekly'
          ? formData.draw_days
          : undefined,
      draw_time: convertTimeToGMT(formData.draw_time),
      sales_cutoff_minutes: formData.sales_cutoff_minutes,
      base_price: formData.base_price,
      max_tickets_per_player: formData.max_tickets_per_player,
      max_tickets_per_transaction: formData.max_tickets_per_transaction,
      multi_draw_enabled: formData.multi_draw_enabled,
      max_draws_advance: formData.multi_draw_enabled ? formData.max_draws_advance : undefined,
      bonus_number_enabled: formData.bonus_number_enabled,
      start_time: convertTimeToGMT(formData.start_time),
      end_time: convertTimeToGMT(formData.end_time),
      status: formData.status || 'Draft',
      // Legacy fields for backward compatibility
      organizer: formData.organizer,
      game_format: formData.game_format,
      game_type: formData.game_type,
      min_stake: formData.min_stake,
      max_stake: formData.max_stake,
      weekly_schedule: formData.weekly_schedule,
    }

    console.log('Payload being sent to backend:', JSON.stringify(payload, null, 2))
    console.log('Bet types in payload:', payload.bet_types)

    createGameMutation.mutate(payload)
  }

  const handleFormatChange = (newFormat: string) => {
    const selectedFormat = gameFormatOptions.find(f => f.value === newFormat)
    if (selectedFormat) {
      form.setValue(
        'format',
        newFormat as '5_by_90' | '5_by_30' | '6_by_90' | '6_by_49' | '4_by_90' | '3_by_90'
      )
      form.setValue('game_format', newFormat)
      form.setValue('number_range_min', selectedFormat.min)
      form.setValue('number_range_max', selectedFormat.max)
      form.setValue('selection_count', selectedFormat.selection)
    }
  }

  const handleBetTypeToggle = (betTypeId: string) => {
    const currentBetTypes = form.getValues('bet_types')
    const updatedBetTypes = currentBetTypes.map(bt =>
      bt.id === betTypeId ? { ...bt, enabled: !bt.enabled } : bt
    )

    // If no bet types exist for this id, add it
    if (!currentBetTypes.find(bt => bt.id === betTypeId)) {
      const newBetType = availableBetTypes.find(bt => bt.id === betTypeId)
      if (newBetType) {
        updatedBetTypes.push({ ...newBetType, enabled: true })
      }
    }

    form.setValue('bet_types', updatedBetTypes)
  }

  const handleMultiplierChange = (betTypeId: string, multiplier: number) => {
    const currentBetTypes = form.getValues('bet_types')
    const updatedBetTypes = currentBetTypes.map(bt =>
      bt.id === betTypeId ? { ...bt, multiplier } : bt
    )
    form.setValue('bet_types', updatedBetTypes)
  }

  const getFieldsForStep = (step: number): (keyof GameFormData)[] => {
    switch (step) {
      case 1:
        return ['code', 'name', 'game_category', 'description']
      case 2:
        return ['format', 'number_range_min', 'number_range_max', 'selection_count', 'bet_types']
      case 3:
        return [
          'base_price',
          'max_tickets_per_player',
          'max_tickets_per_transaction',
          'multi_draw_enabled',
        ]
      case 4:
        return ['draw_frequency', 'draw_time', 'sales_cutoff_minutes']
      default:
        return []
    }
  }

  const progressPercentage = (currentStep / steps.length) * 100

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Create New Game</DialogTitle>
          <DialogDescription>Complete all steps to create a new lottery game</DialogDescription>
        </DialogHeader>

        <div className="space-y-6">
          {/* Progress Bar */}
          <div className="space-y-2">
            <Progress value={progressPercentage} className="h-2" />
            <div className="flex justify-between">
              {steps.map(step => {
                const Icon = step.icon
                return (
                  <div
                    key={step.id}
                    className={`flex items-center gap-2 text-sm ${
                      step.id === currentStep
                        ? 'text-primary font-medium'
                        : step.id < currentStep
                          ? 'text-muted-foreground'
                          : 'text-muted-foreground/50'
                    }`}
                  >
                    <Icon className="h-4 w-4" />
                    <span className="hidden sm:inline">{step.title}</span>
                  </div>
                )
              })}
            </div>
          </div>

          <Form {...form}>
            <form className="space-y-4">
              {/* Step 1: Basic Information */}
              {currentStep === 1 && (
                <div className="space-y-4">
                  <FormField
                    control={form.control}
                    name="code"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Game Code</FormLabel>
                        <FormControl>
                          <Input
                            placeholder="e.g., MON590"
                            {...field}
                            value={field.value as string}
                          />
                        </FormControl>
                        <FormDescription>
                          Unique identifier for the game (2-10 characters)
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="name"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Game Name</FormLabel>
                        <FormControl>
                          <Input
                            placeholder="e.g., Monday Special"
                            {...field}
                            value={field.value as string}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="game_category"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Game Category</FormLabel>
                        <Select onValueChange={field.onChange} defaultValue={field.value as string}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="Select game category" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value="national">National</SelectItem>
                            <SelectItem value="private">Private</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          National games are regulated by NLA, Private games are for specific
                          operators
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="description"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Description (Optional)</FormLabel>
                        <FormControl>
                          <Textarea
                            placeholder="Enter game description..."
                            className="resize-none"
                            {...field}
                            value={field.value as string}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              )}

              {/* Step 2: Format & Bet Types */}
              {currentStep === 2 && (
                <div className="space-y-6">
                  <FormField
                    control={form.control}
                    name="format"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Game Format</FormLabel>
                        <Select
                          onValueChange={handleFormatChange}
                          defaultValue={field.value as string}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="Select game format" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            {gameFormatOptions.map(format => (
                              <SelectItem key={format.value} value={format.value}>
                                {format.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          The format defines how many numbers players pick and from what range
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  {/* Display current format details */}
                  <div className="rounded-lg border p-4 bg-muted/50">
                    <div className="grid grid-cols-3 gap-4 text-sm">
                      <div>
                        <span className="text-muted-foreground">Numbers to Select:</span>
                        <div className="font-medium">{form.watch('selection_count')}</div>
                      </div>
                      <div>
                        <span className="text-muted-foreground">Number Range:</span>
                        <div className="font-medium">
                          {form.watch('number_range_min')} - {form.watch('number_range_max')}
                        </div>
                      </div>
                      <div>
                        <span className="text-muted-foreground">Format:</span>
                        <div className="font-medium">{form.watch('format')}</div>
                      </div>
                    </div>
                  </div>

                  {/* Bet Types Configuration */}
                  <div className="space-y-4">
                    <div>
                      <h3 className="font-semibold mb-2">Available Bet Types</h3>
                      <p className="text-sm text-muted-foreground mb-4">
                        Select which bet types players can use for this game and configure their win
                        multipliers.
                      </p>
                    </div>

                    <div className="space-y-3">
                      {availableBetTypes.map(betType => {
                        const currentBetTypes = form.watch('bet_types')
                        const currentBetType = currentBetTypes.find(bt => bt.id === betType.id)
                        const isEnabled = currentBetType?.enabled || false
                        const currentMultiplier = currentBetType?.multiplier || betType.multiplier

                        return (
                          <div
                            key={betType.id}
                            className="flex items-center justify-between rounded-lg border p-4"
                          >
                            <div className="space-y-0.5">
                              <div className="flex items-center space-x-3">
                                <input
                                  type="checkbox"
                                  checked={isEnabled}
                                  onChange={() => handleBetTypeToggle(betType.id)}
                                  className="rounded border-gray-300"
                                />
                                <div>
                                  <div className="font-medium">{betType.name}</div>
                                  <div className="text-sm text-muted-foreground">
                                    Default multiplier: {betType.multiplier}x
                                  </div>
                                </div>
                              </div>
                            </div>
                            {isEnabled && (
                              <div className="flex items-center space-x-2">
                                <span className="text-sm text-muted-foreground">Multiplier:</span>
                                <Input
                                  type="number"
                                  value={currentMultiplier}
                                  onChange={e =>
                                    handleMultiplierChange(betType.id, parseInt(e.target.value))
                                  }
                                  className="w-20"
                                  min="1"
                                  max="10000"
                                />
                                <span className="text-sm text-muted-foreground">x</span>
                              </div>
                            )}
                          </div>
                        )
                      })}
                    </div>

                    {form.getValues('bet_types').filter(bt => bt.enabled).length === 0 && (
                      <div className="rounded-lg border border-destructive/50 p-4 text-sm text-destructive">
                        Please enable at least one bet type for this game.
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Step 3: Pricing & Limits */}
              {currentStep === 3 && (
                <div className="space-y-4">
                  <div className="grid grid-cols-2 gap-4">
                    <FormField
                      control={form.control}
                      name="base_price"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Base Ticket Price (₵)</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              step="0.50"
                              min="0.50"
                              max="200"
                              {...field}
                              value={field.value as number}
                              onChange={e => field.onChange(parseFloat(e.target.value))}
                            />
                          </FormControl>
                          <FormDescription>Base price for a single ticket</FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="max_tickets_per_player"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Max Tickets Per Player</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              min="1"
                              max="100"
                              {...field}
                              value={field.value as number}
                              onChange={e => field.onChange(parseInt(e.target.value))}
                            />
                          </FormControl>
                          <FormDescription>Maximum tickets per player</FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <FormField
                      control={form.control}
                      name="max_tickets_per_transaction"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Max Tickets Per Transaction</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              min="1"
                              max="50"
                              {...field}
                              value={field.value as number}
                              onChange={e => field.onChange(parseInt(e.target.value))}
                            />
                          </FormControl>
                          <FormDescription>Maximum tickets per single transaction</FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="multi_draw_enabled"
                      render={({ field }) => (
                        <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                          <div className="space-y-0.5">
                            <FormLabel className="text-base">Multi-Draw</FormLabel>
                            <FormDescription>
                              Allow players to buy tickets for multiple draws
                            </FormDescription>
                          </div>
                          <FormControl>
                            <input
                              type="checkbox"
                              checked={field.value as boolean}
                              onChange={e => field.onChange(e.target.checked)}
                              className="rounded border-gray-300"
                            />
                          </FormControl>
                        </FormItem>
                      )}
                    />
                  </div>

                  {form.watch('multi_draw_enabled') && (
                    <FormField
                      control={form.control}
                      name="max_draws_advance"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Max Advance Draws</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              min="1"
                              max="30"
                              {...field}
                              value={field.value as number}
                              onChange={e => field.onChange(parseInt(e.target.value))}
                            />
                          </FormControl>
                          <FormDescription>
                            Maximum number of future draws players can purchase
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  )}
                </div>
              )}

              {/* Step 4: Schedule */}
              {currentStep === 4 && (
                <div className="space-y-4">
                  <FormField
                    control={form.control}
                    name="draw_frequency"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Draw Frequency</FormLabel>
                        <Select onValueChange={field.onChange} defaultValue={field.value as string}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder="Select frequency" />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value="daily">Daily</SelectItem>
                            <SelectItem value="weekly">Weekly</SelectItem>
                            <SelectItem value="bi_weekly">Bi-Weekly</SelectItem>
                            <SelectItem value="monthly">Monthly</SelectItem>
                            <SelectItem value="special">Special</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  {(form.watch('draw_frequency') === 'weekly' ||
                    form.watch('draw_frequency') === 'bi_weekly') && (
                    <FormField
                      control={form.control}
                      name="draw_days"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Draw Days</FormLabel>
                          <div className="grid grid-cols-2 gap-2">
                            {weekDays.map(day => (
                              <label
                                key={day.value}
                                className="flex items-center space-x-2 cursor-pointer"
                              >
                                <input
                                  type="checkbox"
                                  value={day.value}
                                  checked={(field.value as string[])?.includes(day.value)}
                                  onChange={e => {
                                    const currentDays = (field.value as string[]) || []
                                    const updatedDays = e.target.checked
                                      ? [...currentDays, day.value]
                                      : currentDays.filter((d: string) => d !== day.value)
                                    field.onChange(updatedDays)
                                  }}
                                  className="rounded border-gray-300"
                                />
                                <span className="text-sm">{day.label}</span>
                              </label>
                            ))}
                          </div>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  )}

                  <FormField
                    control={form.control}
                    name="draw_time"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          Draw Time (GMT) <span className="text-destructive">*</span>
                        </FormLabel>
                        <FormControl>
                          <Input
                            type="time"
                            {...field}
                            value={field.value as string}
                            required
                            placeholder="HH:MM"
                          />
                        </FormControl>
                        <FormDescription>
                          When will the draw take place? (Required - Ghana is in GMT timezone)
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="sales_cutoff_minutes"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Sales Cutoff (minutes before draw)</FormLabel>
                        <FormControl>
                          <Input
                            type="number"
                            {...field}
                            value={field.value as number}
                            onChange={e => field.onChange(parseInt(e.target.value))}
                          />
                        </FormControl>
                        <FormDescription>
                          How many minutes before the draw should sales stop
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              )}

              {/* Step 5: Review & Submit */}
              {currentStep === 5 && (
                <div className="space-y-4">
                  <div className="rounded-lg border p-4 space-y-3">
                    <h3 className="font-semibold">Review Game Configuration</h3>

                    <div className="space-y-2">
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Code:</span>
                        <span className="font-medium">{form.watch('code')}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Name:</span>
                        <span className="font-medium">{form.watch('name')}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Category:</span>
                        <span className="font-medium capitalize">
                          {form.watch('game_category')}
                        </span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Format:</span>
                        <span className="font-medium">{form.watch('format')}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Base Price:</span>
                        <span className="font-medium">₵{form.watch('base_price')}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Number Range:</span>
                        <span className="font-medium">
                          {form.watch('number_range_min')} - {form.watch('number_range_max')}
                        </span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Selection Count:</span>
                        <span className="font-medium">{form.watch('selection_count')}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Draw Frequency:</span>
                        <span className="font-medium capitalize">
                          {form.watch('draw_frequency').replace('_', ' ')}
                        </span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Enabled Bet Types:</span>
                        <span className="font-medium">
                          {form
                            .getValues('bet_types')
                            .filter(bt => bt.enabled)
                            .map(bt => bt.name)
                            .join(', ') || 'None'}
                        </span>
                      </div>
                    </div>
                  </div>

                  <div className="rounded-lg bg-muted p-4">
                    <p className="text-sm text-muted-foreground">
                      Once submitted, this game will be sent for approval. You will be notified when
                      the game is approved and ready to be activated.
                    </p>
                  </div>
                </div>
              )}
            </form>
          </Form>
        </div>

        <DialogFooter className="gap-2">
          {currentStep > 1 && (
            <Button variant="outline" onClick={handleBack} disabled={createGameMutation.isPending}>
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back
            </Button>
          )}

          <div className="flex-1" />

          <Button variant="outline" onClick={onClose} disabled={createGameMutation.isPending}>
            Cancel
          </Button>

          <Button onClick={handleNext} disabled={createGameMutation.isPending}>
            {createGameMutation.isPending ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Creating...
              </>
            ) : currentStep === 5 ? (
              <>
                <Check className="mr-2 h-4 w-4" />
                Create Game
              </>
            ) : (
              <>
                Next
                <ArrowRight className="ml-2 h-4 w-4" />
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
