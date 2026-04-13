import { useState, useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import * as z from 'zod'
import { formatInGhanaTime } from '@/lib/date-utils'
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
import { Switch } from '@/components/ui/switch'
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
  Lock,
  AlertCircle,
} from 'lucide-react'
import { gameService, type UpdateGameRequest, type Game } from '@/services/games'
import { Alert, AlertDescription } from '@/components/ui/alert'

const betTypeSchema = z.object({
  id: z.string(),
  name: z.string(),
  enabled: z.boolean(),
  multiplier: z.number().min(1).max(100000000), // Allow up to 100 million
})

const gameSchema = z.object({
  // Step 1: Basic Information (Most fields are read-only for editing)
  code: z.string().optional(), // Read-only - no validation needed
  name: z.string().min(3, 'Game name must be at least 3 characters'),
  organizer: z.string().optional(), // Read-only - no validation needed
  game_category: z.string().optional(), // Read-only - no validation needed
  description: z.string().optional().nullable(),

  // Step 2: Game Format & Bet Types (Format is read-only)
  game_format: z.string().optional(), // Read-only - no validation needed
  number_range_min: z.number().optional(), // Read-only - no validation needed
  number_range_max: z.number().optional(), // Read-only - no validation needed
  selection_count: z.number().optional(), // Read-only - no validation needed
  bet_types: z
    .array(betTypeSchema)
    .min(1, 'Bet types array cannot be empty')
    .refine(types => types.some(t => t.enabled), {
      message: 'At least one bet type must be enabled',
    }),

  // Step 3: Pricing & Limits
  base_price: z.number().min(0.5, 'Minimum ticket price is ₵0.50').max(200),
  min_stake: z.number().min(0.5, 'Minimum stake is ₵0.50').max(200000),
  max_stake: z.number().min(0.5, 'Maximum stake is ₵0.50').max(200000),
  max_tickets_per_player: z.number().min(1),
  multi_draw_enabled: z.boolean(),
  max_draws_advance: z.number().optional().nullable(),

  // Step 4: Schedule
  draw_frequency: z.enum(['daily', 'weekly', 'bi_weekly', 'monthly', 'special']),
  draw_days: z.array(z.string()).optional().nullable(),
  draw_time: z.string().optional().nullable(),
  sales_cutoff_minutes: z.number().min(5).max(1440),
})

type GameFormData = z.infer<typeof gameSchema>
type BetType = z.infer<typeof betTypeSchema>

interface EditGameWizardProps {
  isOpen: boolean
  onClose: () => void
  game: Game | null
}

const steps = [
  { id: 1, title: 'Basic Information', icon: Info },
  { id: 2, title: 'Format & Bet Types', icon: Settings },
  { id: 3, title: 'Pricing & Limits', icon: DollarSign },
  { id: 4, title: 'Schedule', icon: Calendar },
  { id: 5, title: 'Review & Submit', icon: Check },
]

// Default bet types - matches CreateGameWizard for consistency
const defaultBetTypes: BetType[] = [
  { id: 'direct_1', name: '1 Direct', enabled: true, multiplier: 40 },
  { id: 'direct_2', name: '2 Direct', enabled: true, multiplier: 240 },
  { id: 'direct_3', name: '3 Direct', enabled: true, multiplier: 1920 },
  { id: 'direct_4', name: '4 Direct', enabled: false, multiplier: 180000 },
  { id: 'direct_5', name: '5 Direct', enabled: false, multiplier: 15000000 },
  { id: 'direct_6', name: '6 Direct', enabled: false, multiplier: 25000000 },
  { id: 'perm_2', name: 'Perm 2', enabled: true, multiplier: 240 },
  { id: 'perm_3', name: 'Perm 3', enabled: true, multiplier: 1920 },
  { id: 'perm_4', name: 'Perm 4', enabled: false, multiplier: 60000 },
  { id: 'perm_5', name: 'Perm 5', enabled: false, multiplier: 5000000 },
  { id: 'perm_6', name: 'Perm 6', enabled: false, multiplier: 25000000 },
  { id: 'banker', name: 'Banker All', enabled: true, multiplier: 240 },
  { id: 'banker_against', name: 'Banker AG', enabled: true, multiplier: 240 },
]

export function EditGameWizard({ isOpen, onClose, game }: EditGameWizardProps) {
  const [currentStep, setCurrentStep] = useState(1)
  const { toast } = useToast()
  const queryClient = useQueryClient()

  const form = useForm<GameFormData>({
    resolver: zodResolver(gameSchema),
    defaultValues: {
      code: '',
      name: '',
      organizer: 'rand_lottery',
      game_category: 'national',
      description: '',
      game_format: '5_by_90',
      number_range_min: 1,
      number_range_max: 90,
      selection_count: 5,
      bet_types: defaultBetTypes,
      base_price: 1,
      min_stake: 1,
      max_stake: 1000,
      max_tickets_per_player: 10,
      multi_draw_enabled: false,
      max_draws_advance: 10,
      draw_frequency: 'daily',
      draw_days: [],
      draw_time: '', // Match CreateGameWizard - empty by default
      sales_cutoff_minutes: 30,
    },
  })

  // Update form when game prop changes
  useEffect(() => {
    if (game) {
      console.log('EditGameWizard: Loading game data', {
        gameId: game.id,
        gameName: game.name,
        betTypes: game.bet_types,
        betTypesLength: game.bet_types?.length,
      })

      // Map existing bet types to the form structure
      let mappedBetTypes: BetType[]

      if (game.bet_types && game.bet_types.length > 0) {
        // Use existing bet types from the game
        mappedBetTypes = game.bet_types.map(bt => ({
          id: bt.id || '',
          name: bt.name || '',
          enabled: bt.enabled ?? false,
          multiplier: bt.multiplier || 1,
        }))
        console.log('EditGameWizard: Using existing bet types', mappedBetTypes)
      } else {
        // Use default bet types if none exist
        mappedBetTypes = defaultBetTypes.map(bt => ({ ...bt }))
        console.log('EditGameWizard: Using default bet types', mappedBetTypes)
      }

      console.log('EditGameWizard: About to reset form with bet types', {
        mappedBetTypesCount: mappedBetTypes.length,
        enabledCount: mappedBetTypes.filter(bt => bt.enabled).length,
        mappedBetTypes,
      })

      form.reset({
        code: game.code || '',
        name: game.name || '',
        organizer: game.organizer || 'rand_lottery',
        game_category: game.game_category || 'national',
        description: game.description || '',
        game_format: game.game_format || '5_by_90',
        number_range_min: game.number_range_min || 1,
        number_range_max: game.number_range_max || 90,
        selection_count: game.selection_count || 5,
        bet_types: mappedBetTypes,
        base_price: game.base_price || 1,
        min_stake: game.min_stake || 1,
        max_stake: game.max_stake || 1000,
        max_tickets_per_player: game.max_tickets_per_player || 10,
        multi_draw_enabled: game.multi_draw_enabled || false,
        max_draws_advance: game.max_draws_advance || 10,
        draw_frequency:
          (game.draw_frequency?.toLowerCase() as
            | 'special'
            | 'daily'
            | 'weekly'
            | 'bi_weekly'
            | 'monthly') || 'daily',
        draw_days: game.draw_days || [],
        draw_time: game.draw_time || '', // Use actual value or empty string
        sales_cutoff_minutes: game.sales_cutoff_minutes || 30,
      })

      setCurrentStep(1)

      console.log('EditGameWizard: Form reset complete', {
        betTypes: form.getValues('bet_types'),
        scheduleValues: {
          draw_frequency: form.getValues('draw_frequency'),
          draw_time: form.getValues('draw_time'),
          sales_cutoff_minutes: form.getValues('sales_cutoff_minutes'),
          draw_days: form.getValues('draw_days'),
        },
        allValues: form.getValues(),
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game])

  // Helper function to convert time string to GMT
  const convertTimeToGMT = (timeString?: string | null): string | undefined => {
    if (!timeString) return undefined

    // Create a date object with today's date and the selected time in GMT
    const today = new Date()
    const [hours, minutes] = timeString.split(':')
    const gmtDate = new Date(
      Date.UTC(
        today.getFullYear(),
        today.getMonth(),
        today.getDate(),
        parseInt(hours),
        parseInt(minutes)
      )
    )

    // Format back to HH:mm in GMT
    return formatInGhanaTime(gmtDate, 'HH:mm')
  }

  const updateMutation = useMutation({
    mutationFn: async (data: GameFormData) => {
      if (!game?.id) throw new Error('No game ID')

      console.log('EditGameWizard: Preparing update data', {
        gameId: game.id,
        formData: data,
        originalDrawTime: data.draw_time,
        convertedDrawTime: convertTimeToGMT(data.draw_time),
      })

      const updateData: UpdateGameRequest = {
        name: data.name,
        description: data.description || undefined,
        bet_types: data.bet_types,
        base_price: data.base_price,
        min_stake: data.min_stake,
        max_stake: data.max_stake,
        max_tickets_per_player: data.max_tickets_per_player,
        multi_draw_enabled: data.multi_draw_enabled,
        max_draws_advance: data.max_draws_advance || undefined,
        draw_frequency: data.draw_frequency, // Already in lowercase format
        draw_days: data.draw_days || undefined,
        draw_time: convertTimeToGMT(data.draw_time),
        sales_cutoff_minutes: data.sales_cutoff_minutes,
      }

      console.log('EditGameWizard: Sending update request', {
        gameId: game.id,
        updateData,
      })

      return gameService.updateGame(game.id, updateData)
    },
    onSuccess: response => {
      console.log('EditGameWizard: Update successful', response)
      queryClient.invalidateQueries({ queryKey: ['games'] })
      toast({
        title: 'Success',
        description: 'Game updated successfully',
      })
      onClose()
      form.reset()
      setCurrentStep(1)
    },
    onError: (error: unknown) => {
      console.error('EditGameWizard: Update failed', error)
      const axiosError = error as { response?: { data?: { message?: string } } }
      toast({
        title: 'Error',
        description: axiosError.response?.data?.message || 'Failed to update game',
        variant: 'destructive',
      })
    },
  })

  const handleNext = async () => {
    const fieldsToValidate = getFieldsForStep(currentStep)
    const currentBetTypes = form.getValues('bet_types')
    console.log('EditGameWizard: handleNext called', {
      currentStep,
      fieldsToValidate,
      betTypes: currentBetTypes,
      betTypesCount: currentBetTypes?.length,
      enabledBetTypes: currentBetTypes?.filter(bt => bt.enabled),
      enabledCount: currentBetTypes?.filter(bt => bt.enabled).length,
      formValues: form.getValues(),
      formErrors: form.formState.errors,
    })

    // Only validate if there are fields to validate for this step
    let isValid = true
    if (fieldsToValidate.length > 0) {
      isValid = await form.trigger(fieldsToValidate as Parameters<typeof form.trigger>[0])
      console.log('EditGameWizard: Validation result', {
        isValid,
        errors: form.formState.errors,
        betTypesAfterValidation: form.getValues('bet_types'),
      })
    } else {
      console.log('EditGameWizard: No fields to validate, advancing...')
    }

    if (isValid) {
      if (currentStep === 5) {
        // Submit the form - do full validation before submitting
        const allFieldsValid = await form.trigger()
        if (allFieldsValid) {
          const formData = form.getValues()
          updateMutation.mutate(formData)
        } else {
          toast({
            title: 'Validation Error',
            description: 'Please review all fields before submitting',
            variant: 'destructive',
          })
        }
      } else {
        setCurrentStep(currentStep + 1)
      }
    } else {
      // Show validation errors to user
      const errors = form.formState.errors
      console.error('EditGameWizard: Validation failed', errors)

      // Special logging for bet_types errors
      if (errors.bet_types) {
        console.error('EditGameWizard: Bet types error details:', {
          errorType: typeof errors.bet_types,
          error: errors.bet_types,
          message: errors.bet_types.message,
          arrayErrors: errors.bet_types.root || errors.bet_types,
        })

        // Log each individual bet type error
        if (Array.isArray(errors.bet_types)) {
          errors.bet_types.forEach((err: { multiplier?: { message?: string } }, index: number) => {
            if (err) {
              const currentBetType = form.getValues(`bet_types.${index}`)
              console.error(`Bet type ${index} error:`, {
                error: err,
                currentValue: currentBetType,
                multiplierError: err.multiplier,
              })
            }
          })
        }
      }

      // Build detailed error message
      const errorMessages: string[] = []
      fieldsToValidate.forEach(fieldName => {
        const error = errors[fieldName]
        if (error) {
          const fieldLabel = fieldName.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
          const errorMsg = error.message || JSON.stringify(error)
          errorMessages.push(`${fieldLabel}: ${errorMsg}`)
          console.error(`Field ${fieldName} error:`, error)
        }
      })

      toast({
        title: 'Validation Error',
        description:
          errorMessages.length > 0
            ? errorMessages.join('. ')
            : 'Please fix the errors before continuing',
        variant: 'destructive',
      })
    }
  }

  const handlePrevious = () => {
    setCurrentStep(currentStep - 1)
  }

  const getFieldsForStep = (step: number): (keyof GameFormData)[] => {
    switch (step) {
      case 1:
        return ['name'] // Only validate required fields
      case 2:
        return ['bet_types']
      case 3:
        return ['base_price', 'min_stake', 'max_stake', 'max_tickets_per_player']
      case 4:
        return ['draw_frequency', 'sales_cutoff_minutes'] // draw_time is optional
      default:
        return []
    }
  }

  const getStepProgress = () => {
    return ((currentStep - 1) / (steps.length - 1)) * 100
  }

  const renderStep = () => {
    switch (currentStep) {
      case 1:
        return renderBasicInfo()
      case 2:
        return renderFormatAndBetTypes()
      case 3:
        return renderPricingAndLimits()
      case 4:
        return renderSchedule()
      case 5:
        return renderReview()
      default:
        return null
    }
  }

  const renderBasicInfo = () => (
    <div className="space-y-4">
      <Alert>
        <Lock className="h-4 w-4" />
        <AlertDescription>
          Some fields are read-only after game creation and require approval to change.
        </AlertDescription>
      </Alert>

      <div className="grid grid-cols-2 gap-4">
        <FormField
          control={form.control}
          name="code"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Game Code</FormLabel>
              <FormControl>
                <div className="relative">
                  <Input
                    {...field}
                    value={field.value as string}
                    disabled
                    className="pr-8 bg-gray-50"
                  />
                  <Lock className="absolute right-2 top-2.5 h-4 w-4 text-gray-400" />
                </div>
              </FormControl>
              <FormDescription>Cannot be changed after creation</FormDescription>
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
                  {...field}
                  value={field.value as string}
                  placeholder="e.g., National Lotto 5/90"
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <FormField
          control={form.control}
          name="organizer"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Organizer</FormLabel>
              <FormControl>
                <div className="relative">
                  <Input
                    {...field}
                    value={
                      (field.value === 'nla'
                        ? 'National Lottery Authority'
                        : 'Spiel') as string
                    }
                    disabled
                    className="pr-8 bg-gray-50"
                  />
                  <Lock className="absolute right-2 top-2.5 h-4 w-4 text-gray-400" />
                </div>
              </FormControl>
              <FormDescription>Cannot be changed after creation</FormDescription>
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="game_category"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Category</FormLabel>
              <FormControl>
                <div className="relative">
                  <Input
                    {...field}
                    value={(field.value === 'national' ? 'National' : 'Private') as string}
                    disabled
                    className="pr-8 bg-gray-50 capitalize"
                  />
                  <Lock className="absolute right-2 top-2.5 h-4 w-4 text-gray-400" />
                </div>
              </FormControl>
              <FormDescription>Cannot be changed after creation</FormDescription>
            </FormItem>
          )}
        />
      </div>

      <FormField
        control={form.control}
        name="description"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Description (Optional)</FormLabel>
            <FormControl>
              <Textarea
                {...field}
                value={(field.value || '') as string}
                placeholder="Enter game description..."
                className="min-h-[100px]"
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  )

  const renderFormatAndBetTypes = () => {
    const betTypes = form.watch('bet_types')
    console.log('EditGameWizard: Rendering bet types', {
      betTypesCount: betTypes?.length,
      betTypes: betTypes,
    })

    return (
      <div className="space-y-4">
        <Alert>
          <Lock className="h-4 w-4" />
          <AlertDescription>
            Game format and number range cannot be changed after creation.
          </AlertDescription>
        </Alert>

        <div className="grid grid-cols-2 gap-4">
          <FormField
            control={form.control}
            name="game_format"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Game Format</FormLabel>
                <FormControl>
                  <div className="relative">
                    <Input
                      {...field}
                      value={(field.value as string)?.replace(/_/g, ' ').toUpperCase() || ''}
                      disabled
                      className="pr-8 bg-gray-50"
                    />
                    <Lock className="absolute right-2 top-2.5 h-4 w-4 text-gray-400" />
                  </div>
                </FormControl>
                <FormDescription>Cannot be changed after creation</FormDescription>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="selection_count"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Selection Count</FormLabel>
                <FormControl>
                  <div className="relative">
                    <Input
                      {...field}
                      value={field.value as number}
                      type="number"
                      disabled
                      className="pr-8 bg-gray-50"
                    />
                    <Lock className="absolute right-2 top-2.5 h-4 w-4 text-gray-400" />
                  </div>
                </FormControl>
                <FormDescription>Numbers to select per ticket</FormDescription>
              </FormItem>
            )}
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <FormField
            control={form.control}
            name="number_range_min"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Min Number</FormLabel>
                <FormControl>
                  <div className="relative">
                    <Input
                      {...field}
                      value={field.value as number}
                      type="number"
                      disabled
                      className="pr-8 bg-gray-50"
                    />
                    <Lock className="absolute right-2 top-2.5 h-4 w-4 text-gray-400" />
                  </div>
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="number_range_max"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Max Number</FormLabel>
                <FormControl>
                  <div className="relative">
                    <Input
                      {...field}
                      value={field.value as number}
                      type="number"
                      disabled
                      className="pr-8 bg-gray-50"
                    />
                    <Lock className="absolute right-2 top-2.5 h-4 w-4 text-gray-400" />
                  </div>
                </FormControl>
              </FormItem>
            )}
          />
        </div>

        <FormField
          control={form.control}
          name="bet_types"
          render={() => (
            <FormItem>
              <FormLabel>Bet Types</FormLabel>
              <div className="border rounded-lg p-4 space-y-3">
                {form.watch('bet_types')?.map((betType, index) => (
                  <div key={betType.id} className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <Switch
                        checked={betType.enabled}
                        onCheckedChange={checked => {
                          const currentBetTypes = [...form.getValues('bet_types')]
                          currentBetTypes[index] = { ...currentBetTypes[index], enabled: checked }
                          form.setValue('bet_types', currentBetTypes, { shouldValidate: true })
                        }}
                      />
                      <span className="font-medium">{betType.name}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-sm text-gray-500">Multiplier:</span>
                      <Input
                        type="number"
                        value={betType.multiplier}
                        onChange={e => {
                          const currentBetTypes = [...form.getValues('bet_types')]
                          currentBetTypes[index] = {
                            ...currentBetTypes[index],
                            multiplier: parseInt(e.target.value) || 1,
                          }
                          form.setValue('bet_types', currentBetTypes, { shouldValidate: true })
                        }}
                        className="w-24"
                        min={1}
                        max={100000000}
                      />
                    </div>
                  </div>
                ))}
              </div>
              <FormDescription>
                Enable bet types and set their multipliers. At least one must be enabled.
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>
    )
  }

  const renderPricingAndLimits = () => (
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
                  {...field}
                  value={field.value as number}
                  type="number"
                  step="0.5"
                  onChange={e => field.onChange(parseFloat(e.target.value))}
                />
              </FormControl>
              <FormDescription>Base price per ticket</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="max_tickets_per_player"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Max Tickets per Player</FormLabel>
              <FormControl>
                <Input
                  {...field}
                  value={field.value as number}
                  type="number"
                  onChange={e => field.onChange(parseInt(e.target.value))}
                />
              </FormControl>
              <FormDescription>Maximum tickets per transaction</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <FormField
          control={form.control}
          name="min_stake"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Minimum Stake (₵)</FormLabel>
              <FormControl>
                <Input
                  {...field}
                  value={field.value as number}
                  type="number"
                  step="0.5"
                  onChange={e => field.onChange(parseFloat(e.target.value))}
                />
              </FormControl>
              <FormDescription>Minimum bet amount</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="max_stake"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Maximum Stake (₵)</FormLabel>
              <FormControl>
                <Input
                  {...field}
                  value={field.value as number}
                  type="number"
                  step="0.5"
                  onChange={e => field.onChange(parseFloat(e.target.value))}
                />
              </FormControl>
              <FormDescription>Maximum bet amount</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <div className="space-y-4 border rounded-lg p-4">
        <FormField
          control={form.control}
          name="multi_draw_enabled"
          render={({ field }) => (
            <FormItem className="flex items-center justify-between">
              <div>
                <FormLabel>Multi-Draw Enabled</FormLabel>
                <FormDescription>Allow players to enter multiple draws</FormDescription>
              </div>
              <FormControl>
                <Switch checked={field.value as boolean} onCheckedChange={field.onChange} />
              </FormControl>
            </FormItem>
          )}
        />

        {form.watch('multi_draw_enabled') && (
          <FormField
            control={form.control}
            name="max_draws_advance"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Max Draws in Advance</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    type="number"
                    value={(field.value as string) || ''}
                    onChange={e => field.onChange(parseInt(e.target.value) || null)}
                  />
                </FormControl>
                <FormDescription>Maximum number of future draws</FormDescription>
              </FormItem>
            )}
          />
        )}
      </div>
    </div>
  )

  const renderSchedule = () => {
    const drawFrequency = form.watch('draw_frequency')

    return (
      <div className="space-y-4">
        <FormField
          control={form.control}
          name="draw_frequency"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Draw Frequency</FormLabel>
              <Select onValueChange={field.onChange} value={field.value as string}>
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

        {(drawFrequency === 'weekly' || drawFrequency === 'bi_weekly') && (
          <FormField
            control={form.control}
            name="draw_days"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Draw Days</FormLabel>
                <div className="grid grid-cols-7 gap-2">
                  {[
                    { short: 'Mon', full: 'monday' },
                    { short: 'Tue', full: 'tuesday' },
                    { short: 'Wed', full: 'wednesday' },
                    { short: 'Thu', full: 'thursday' },
                    { short: 'Fri', full: 'friday' },
                    { short: 'Sat', full: 'saturday' },
                    { short: 'Sun', full: 'sunday' },
                  ].map(day => (
                    <label key={day.full} className="flex items-center gap-1 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={(field.value as string[])?.includes(day.full) || false}
                        onChange={e => {
                          const currentDays = (field.value as string[]) || []
                          if (e.target.checked) {
                            field.onChange([...currentDays, day.full])
                          } else {
                            field.onChange(currentDays.filter((d: string) => d !== day.full))
                          }
                        }}
                        className="rounded"
                      />
                      <span className="text-sm">{day.short}</span>
                    </label>
                  ))}
                </div>
                <FormDescription>Select days for draws</FormDescription>
              </FormItem>
            )}
          />
        )}

        <div className="grid grid-cols-2 gap-4">
          <FormField
            control={form.control}
            name="draw_time"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Draw Time (GMT)</FormLabel>
                <FormControl>
                  <Input {...field} type="time" value={(field.value as string) || ''} />
                </FormControl>
                <FormDescription>Enter the draw time in GMT timezone</FormDescription>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="sales_cutoff_minutes"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Sales Cutoff (minutes)</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    value={field.value as number}
                    type="number"
                    onChange={e => field.onChange(parseInt(e.target.value))}
                  />
                </FormControl>
                <FormDescription>Minutes before draw to stop sales</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
      </div>
    )
  }

  const renderReview = () => {
    const values = form.getValues()
    const enabledBetTypes = values.bet_types.filter(bt => bt.enabled)

    return (
      <div className="space-y-4">
        <Alert>
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>
            Please review your changes before submitting. Some changes may require approval.
          </AlertDescription>
        </Alert>

        <div className="space-y-3">
          <div className="border rounded-lg p-4 space-y-3">
            <h4 className="font-semibold text-sm">Basic Information</h4>
            <div className="grid grid-cols-2 gap-2 text-sm">
              <div>
                <span className="text-gray-500">Code:</span>{' '}
                <span className="font-medium">{values.code}</span>
              </div>
              <div>
                <span className="text-gray-500">Name:</span>{' '}
                <span className="font-medium">{values.name}</span>
              </div>
              <div>
                <span className="text-gray-500">Category:</span>{' '}
                <span className="font-medium capitalize">{values.game_category}</span>
              </div>
              <div>
                <span className="text-gray-500">Format:</span>{' '}
                <span className="font-medium">
                  {values.game_format?.replace(/_/g, ' ').toUpperCase()}
                </span>
              </div>
            </div>
          </div>

          <div className="border rounded-lg p-4 space-y-3">
            <h4 className="font-semibold text-sm">Bet Types</h4>
            <div className="space-y-1 text-sm">
              {enabledBetTypes.map(bt => (
                <div key={bt.id} className="flex justify-between">
                  <span>{bt.name}</span>
                  <span className="text-gray-500">×{bt.multiplier}</span>
                </div>
              ))}
            </div>
          </div>

          <div className="border rounded-lg p-4 space-y-3">
            <h4 className="font-semibold text-sm">Pricing & Limits</h4>
            <div className="grid grid-cols-2 gap-2 text-sm">
              <div>
                <span className="text-gray-500">Base Price:</span>{' '}
                <span className="font-medium">₵{values.base_price}</span>
              </div>
              <div>
                <span className="text-gray-500">Stake Range:</span>{' '}
                <span className="font-medium">
                  ₵{values.min_stake} - ₵{values.max_stake}
                </span>
              </div>
              <div>
                <span className="text-gray-500">Max Tickets:</span>{' '}
                <span className="font-medium">{values.max_tickets_per_player}</span>
              </div>
              <div>
                <span className="text-gray-500">Multi-Draw:</span>{' '}
                <span className="font-medium">{values.multi_draw_enabled ? 'Yes' : 'No'}</span>
              </div>
            </div>
          </div>

          <div className="border rounded-lg p-4 space-y-3">
            <h4 className="font-semibold text-sm">Schedule</h4>
            <div className="grid grid-cols-2 gap-2 text-sm">
              <div>
                <span className="text-gray-500">Frequency:</span>{' '}
                <span className="font-medium capitalize">
                  {values.draw_frequency.replace('_', '-')}
                </span>
              </div>
              <div>
                <span className="text-gray-500">Draw Time:</span>{' '}
                <span className="font-medium">{values.draw_time || 'Not set'}</span>
              </div>
              <div>
                <span className="text-gray-500">Sales Cutoff:</span>{' '}
                <span className="font-medium">{values.sales_cutoff_minutes} mins</span>
              </div>
              {values.draw_days && values.draw_days.length > 0 && (
                <div>
                  <span className="text-gray-500">Days:</span>{' '}
                  <span className="font-medium capitalize">{values.draw_days.join(', ')}</span>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Edit Game Configuration</DialogTitle>
          <DialogDescription>
            Update game settings. Some fields are read-only and may require approval.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-6">
          {/* Progress Bar */}
          <div className="space-y-2">
            <Progress value={getStepProgress()} className="h-2" />
            <div className="flex justify-between">
              {steps.map(step => {
                const Icon = step.icon
                return (
                  <div
                    key={step.id}
                    className={`flex flex-col items-center gap-1 ${
                      currentStep >= step.id ? 'text-primary' : 'text-gray-400'
                    }`}
                  >
                    <Icon className="h-5 w-5" />
                    <span className="text-xs font-medium">{step.title}</span>
                  </div>
                )
              })}
            </div>
          </div>

          {/* Form Content */}
          <Form {...form}>
            <form className="space-y-6">{renderStep()}</form>
          </Form>
        </div>

        <DialogFooter>
          <div className="flex justify-between w-full">
            <Button
              variant="outline"
              onClick={handlePrevious}
              disabled={currentStep === 1 || updateMutation.isPending}
            >
              <ArrowLeft className="h-4 w-4 mr-2" />
              Previous
            </Button>

            <Button type="button" onClick={handleNext} disabled={updateMutation.isPending}>
              {updateMutation.isPending ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  Updating...
                </>
              ) : currentStep === 5 ? (
                <>
                  <Check className="h-4 w-4 mr-2" />
                  Update Game
                </>
              ) : (
                <>
                  Next
                  <ArrowRight className="h-4 w-4 ml-2" />
                </>
              )}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
