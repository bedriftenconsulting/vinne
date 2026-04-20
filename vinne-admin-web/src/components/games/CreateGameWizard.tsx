import { useState, useRef } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import * as z from 'zod'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogDescription,
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
  Trophy,
  Calendar,
  FileText,
  Image,
  Plus,
  Trash2,
} from 'lucide-react'
import { gameService, type CreateGameRequest, type PrizeDetail } from '@/services/games'

// ─── Schema ──────────────────────────────────────────────────────────────────

const DAYS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday']

const gameSchema = z.object({
  // Step 1 – Basic Info
  name: z.string().min(3, 'Game name must be at least 3 characters'),
  code: z.string().min(2, 'Game code must be at least 2 characters').max(10),
  description: z.string().optional(),
  status: z.string().default('Draft'),

  // Step 2 – Schedule
  draw_frequency: z.enum(['daily', 'weekly', 'bi_weekly', 'monthly', 'special']),
  draw_time: z.string().min(1, 'Draw time is required'),
  draw_days: z.array(z.string()).optional(),
  sales_cutoff_minutes: z.number().min(1, 'Cutoff must be at least 1 minute'),

  // Step 3 – Dates & Tickets
  draw_date: z.string().optional(),
  base_price: z.number({ invalid_type_error: 'Enter a valid price' }).min(0.5, 'Minimum ticket price is ₵0.50'),
  total_tickets: z.number({ invalid_type_error: 'Enter a valid number' }).int().min(1, 'Total tickets must be at least 1'),
  max_tickets_per_player: z.number({ invalid_type_error: 'Enter a valid number' }).int().min(1, 'At least 1 ticket per player'),

  // Step 4 – Prize & Rules
  prize_details: z.array(z.object({
    rank: z.number().min(1),
    label: z.string().min(1, 'Prize label required'),
    description: z.string().min(1, 'Description required'),
  })).min(1, 'Add at least one prize'),
  rules: z.string().min(10, 'Please provide the game rules'),

  // Step 5 – Logo
  logo_url: z.string().optional(),
}).superRefine((data, ctx) => {
  if (data.draw_frequency === 'special' && !data.draw_date) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'Draw date is required for special draws', path: ['draw_date'] })
  }
})

type GameFormData = z.infer<typeof gameSchema>

// ─── Steps ───────────────────────────────────────────────────────────────────

const steps = [
  { id: 1, title: 'Basic Info',    icon: Info },
  { id: 2, title: 'Schedule',      icon: Calendar },
  { id: 3, title: 'Tickets',       icon: Trophy },
  { id: 4, title: 'Prize & Rules', icon: FileText },
  { id: 5, title: 'Logo & Review', icon: Image },
]

const fieldsPerStep: Record<number, (keyof GameFormData)[]> = {
  1: ['name', 'code', 'description'],
  2: ['draw_frequency', 'draw_time', 'sales_cutoff_minutes', 'draw_date'],
  3: ['base_price', 'total_tickets', 'max_tickets_per_player'],
  4: ['prize_details', 'rules'],
  5: [],
}

// ─── Props ───────────────────────────────────────────────────────────────────

interface CreateGameWizardProps {
  isOpen: boolean
  onClose: () => void
}

// ─── Component ───────────────────────────────────────────────────────────────

export function CreateGameWizard({ isOpen, onClose }: CreateGameWizardProps) {
  const [currentStep, setCurrentStep] = useState(1)
  const [logoPreview, setLogoPreview] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const form = useForm<GameFormData>({
    resolver: zodResolver(gameSchema),
    defaultValues: {
      name: '',
      code: '',
      description: '',
      status: 'Draft',
      draw_frequency: 'weekly',
      draw_time: '20:00',
      draw_days: ['Friday'],
      sales_cutoff_minutes: 30,
      draw_date: '',
      base_price: 1,
      total_tickets: 1000,
      max_tickets_per_player: 10,
      prize_details: [{ rank: 1, label: '', description: '' }],
      rules: '',
      logo_url: '',
    },
  })

  const frequency = form.watch('draw_frequency')
  const drawDays = form.watch('draw_days') || []

  const toggleDrawDay = (day: string) => {
    const current = form.getValues('draw_days') || []
    if (current.includes(day)) {
      form.setValue('draw_days', current.filter(d => d !== day))
    } else {
      form.setValue('draw_days', [...current, day])
    }
  }

  const [logoFile, setLogoFile] = useState<File | null>(null)

  const createGameMutation = useMutation({
    mutationFn: async (data: CreateGameRequest) => {
      // Step 1: create the game
      const game = await gameService.createGame(data)
      // Step 2: upload logo if one was selected
      if (logoFile && game?.id) {
        try {
          await gameService.uploadGameLogo(game.id, logoFile)
        } catch (e) {
          console.warn('Logo upload failed, game still created', e)
        }
      }
      return game
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      queryClient.invalidateQueries({ queryKey: ['games-list'] })
      toast({ title: 'Game created', description: 'The game has been saved.' })
      handleClose()
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error',
        description:
          (error as { response?: { data?: { message?: string } } })?.response?.data?.message ||
          (error as Error)?.message ||
          'Failed to create game',
        variant: 'destructive',
      })
    },
  })

  const handleClose = () => {
    form.reset()
    setCurrentStep(1)
    setLogoPreview(null)
    setLogoFile(null)
    onClose()
  }

  const handleNext = async () => {
    const valid = await form.trigger(fieldsPerStep[currentStep] as (keyof GameFormData)[])
    if (!valid) return
    if (currentStep === steps.length) {
      handleSubmit()
    } else {
      setCurrentStep(s => s + 1)
    }
  }

  const handleBack = () => setCurrentStep(s => s - 1)

  const handleLogoChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setLogoFile(file)
    const reader = new FileReader()
    reader.onload = ev => {
      const url = ev.target?.result as string
      setLogoPreview(url)
      form.setValue('logo_url', url)
    }
    reader.readAsDataURL(file)
  }

  const handleSubmit = () => {
    const d = form.getValues()
    const payload: CreateGameRequest = {
      code: d.code,
      name: d.name,
      description: d.description,
      draw_frequency: d.draw_frequency,
      draw_time: d.draw_time,
      draw_days: (d.draw_frequency === 'weekly' || d.draw_frequency === 'bi_weekly') ? (d.draw_days || []) : undefined,
      sales_cutoff_minutes: d.sales_cutoff_minutes,
      base_price: d.base_price,
      total_tickets: d.total_tickets,
      max_tickets_per_player: d.max_tickets_per_player,
      multi_draw_enabled: false,
      status: d.status as 'Draft' | 'Active',
      draw_date: d.draw_date || d.end_date || undefined,
      prize_details: d.prize_details as PrizeDetail[],
      rules: d.rules,
      // Always competition — never lottery
      game_category: 'private',
      format: 'competition',
      organizer: 'winbig_africa',
      bet_types: [],
      number_range_min: 1,
      number_range_max: 100,
      selection_count: 1,
    }
    console.log('[CreateGame] Payload being sent:', JSON.stringify(payload, null, 2))
    createGameMutation.mutate(payload)
  }

  const progress = (currentStep / steps.length) * 100

  const freqLabel: Record<string, string> = {
    daily: 'Daily',
    weekly: 'Weekly',
    bi_weekly: 'Bi-Weekly',
    monthly: 'Monthly (once per month)',
    special: 'Special (one-time draw)',
  }

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Create New Game</DialogTitle>
          <DialogDescription>Fill in the details to set up a new competition</DialogDescription>
        </DialogHeader>

        {/* Progress */}
        <div className="space-y-2">
          <Progress value={progress} className="h-1.5" />
          <div className="flex justify-between">
            {steps.map(step => {
              const Icon = step.icon
              return (
                <div
                  key={step.id}
                  className={`flex flex-col items-center gap-1 ${currentStep >= step.id ? 'text-primary' : 'text-muted-foreground'}`}
                >
                  <div className={`h-7 w-7 rounded-full flex items-center justify-center text-xs font-semibold border-2 transition-colors ${currentStep > step.id ? 'bg-primary border-primary text-primary-foreground' : currentStep === step.id ? 'border-primary text-primary' : 'border-muted text-muted-foreground'}`}>
                    {currentStep > step.id ? <Check className="h-3.5 w-3.5" /> : <Icon className="h-3.5 w-3.5" />}
                  </div>
                  <span className="text-[10px] font-medium hidden sm:block">{step.title}</span>
                </div>
              )
            })}
          </div>
        </div>

        <Form {...form}>
          <form className="space-y-4 py-2">

            {/* ── Step 1: Basic Info ── */}
            {currentStep === 1 && (
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <FormField
                    control={form.control}
                    name="name"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Game Name</FormLabel>
                        <FormControl><Input placeholder="e.g. NoonRush" {...field} /></FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="code"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Game Code</FormLabel>
                        <FormControl><Input placeholder="e.g. NR01" {...field} onChange={e => field.onChange(e.target.value.toUpperCase())} /></FormControl>
                        <FormDescription>Short unique identifier</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>

                <FormField
                  control={form.control}
                  name="description"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Description <span className="text-muted-foreground font-normal">(optional)</span></FormLabel>
                      <FormControl>
                        <Textarea placeholder="Brief description..." className="resize-none" rows={2} {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

              </div>
            )}

            {/* ── Step 2: Schedule ── */}
            {currentStep === 2 && (
              <div className="space-y-5">
                {/* Frequency */}
                <FormField
                  control={form.control}
                  name="draw_frequency"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Draw Frequency</FormLabel>
                      <Select onValueChange={field.onChange} value={field.value}>
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder="Select frequency" />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          <SelectItem value="daily">Daily — draw every day</SelectItem>
                          <SelectItem value="weekly">Weekly — draw on selected day(s) each week</SelectItem>
                          <SelectItem value="bi_weekly">Bi-Weekly — draw every 2 weeks</SelectItem>
                          <SelectItem value="monthly">Monthly — one draw per month</SelectItem>
                          <SelectItem value="special">Special — one-time draw only</SelectItem>
                        </SelectContent>
                      </Select>
                      <FormDescription>
                        {frequency === 'special' && 'One draw will be scheduled for the whole period.'}
                        {frequency === 'monthly' && 'One draw per month, placed on the last Saturday (or configured draw day).'}
                        {frequency === 'daily' && 'A draw will run every day at the configured draw time.'}
                        {(frequency === 'weekly' || frequency === 'bi_weekly') && 'Select which day(s) of the week draws happen.'}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                {/* Draw days (weekly / bi-weekly only) */}
                {(frequency === 'weekly' || frequency === 'bi_weekly') && (
                  <div className="space-y-2">
                    <FormLabel>Draw Day(s)</FormLabel>
                    <div className="flex flex-wrap gap-2">
                      {DAYS.map(day => (
                        <button
                          key={day}
                          type="button"
                          onClick={() => toggleDrawDay(day)}
                          className={`px-3 py-1.5 rounded-full text-xs font-medium border transition-colors ${drawDays.includes(day) ? 'bg-primary text-primary-foreground border-primary' : 'bg-background text-foreground border-border hover:border-primary/50'}`}
                        >
                          {day.slice(0, 3)}
                        </button>
                      ))}
                    </div>
                    {drawDays.length === 0 && (
                      <p className="text-xs text-destructive">Select at least one draw day</p>
                    )}
                  </div>
                )}

                {/* Draw time + cutoff */}
                <div className="grid grid-cols-2 gap-4">
                  <FormField
                    control={form.control}
                    name="draw_time"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Draw Time</FormLabel>
                        <FormControl><Input type="time" {...field} /></FormControl>
                        <FormDescription>Time the draw runs (Ghana time, GMT+0)</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="sales_cutoff_minutes"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Sales Cutoff</FormLabel>
                        <Select
                          value={String(field.value)}
                          onValueChange={v => field.onChange(parseInt(v))}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value="15">15 min before draw</SelectItem>
                            <SelectItem value="30">30 min before draw</SelectItem>
                            <SelectItem value="60">1 hour before draw</SelectItem>
                            <SelectItem value="120">2 hours before draw</SelectItem>
                            <SelectItem value="360">6 hours before draw</SelectItem>
                            <SelectItem value="720">12 hours before draw</SelectItem>
                            <SelectItem value="1440">24 hours before draw</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormDescription>Ticket sales close this long before the draw</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>

                {/* Dates */}
                <div className="space-y-3">
                  {frequency === 'special' && (
                    <FormField
                      control={form.control}
                      name="draw_date"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Draw Date <span className="text-destructive">*</span></FormLabel>
                          <FormControl><Input type="date" {...field} /></FormControl>
                          <FormDescription>The date of this one-time draw.</FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  )}
                </div>
              </div>
            )}

            {/* ── Step 3: Tickets ── */}
            {currentStep === 3 && (
              <div className="space-y-4">
                <div className="grid grid-cols-3 gap-4">
                  <FormField
                    control={form.control}
                    name="base_price"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Ticket Price (₵)</FormLabel>
                        <FormControl>
                          <Input type="number" step="0.50" min="0.50" {...field}
                            value={field.value ?? ''}
                            onChange={e => {
                              const v = parseFloat(e.target.value)
                              field.onChange(isNaN(v) ? '' : v)
                            }} />
                        </FormControl>
                        <FormDescription>Price per ticket</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="total_tickets"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Total Tickets</FormLabel>
                        <FormControl>
                          <Input type="number" min="1" {...field}
                            value={field.value ?? ''}
                            onChange={e => {
                              const v = parseInt(e.target.value)
                              field.onChange(isNaN(v) ? '' : v)
                            }} />
                        </FormControl>
                        <FormDescription>Max tickets for sale</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name="max_tickets_per_player"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Max per Player</FormLabel>
                        <FormControl>
                          <Input type="number" min="1" {...field}
                            value={field.value ?? ''}
                            onChange={e => {
                              const v = parseInt(e.target.value)
                              field.onChange(isNaN(v) ? '' : v)
                            }} />
                        </FormControl>
                        <FormDescription>Per player limit</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              </div>
            )}

            {/* ── Step 4: Prize & Rules ── */}
            {currentStep === 4 && (
              <div className="space-y-4">
                {/* Structured prize list */}
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <p className="text-sm font-medium">Prize Details</p>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        const current = form.getValues('prize_details') || []
                        form.setValue('prize_details', [
                          ...current,
                          { rank: current.length + 1, label: '', description: '' },
                        ])
                      }}
                    >
                      <Plus className="h-3.5 w-3.5 mr-1" />Add Prize
                    </Button>
                  </div>
                  {(form.watch('prize_details') || []).map((_, idx) => (
                    <div key={idx} className="grid grid-cols-[auto_1fr_2fr_auto] gap-2 items-start">
                      {/* Rank */}
                      <div className="w-10 pt-2 text-center text-sm font-semibold text-muted-foreground">
                        #{idx + 1}
                      </div>
                      {/* Label */}
                      <Input
                        placeholder="e.g. 1st Prize"
                        value={form.watch(`prize_details.${idx}.label`) ?? ''}
                        onChange={e => {
                          const prizes = [...(form.getValues('prize_details') || [])]
                          prizes[idx] = { ...prizes[idx], label: e.target.value }
                          form.setValue('prize_details', prizes)
                        }}
                      />
                      {/* Description */}
                      <Input
                        placeholder="e.g. BMW 3 Series"
                        value={form.watch(`prize_details.${idx}.description`) ?? ''}
                        onChange={e => {
                          const prizes = [...(form.getValues('prize_details') || [])]
                          prizes[idx] = { ...prizes[idx], description: e.target.value }
                          form.setValue('prize_details', prizes)
                        }}
                      />
                      {/* Remove */}
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="text-destructive h-9 w-9"
                        disabled={(form.watch('prize_details') || []).length <= 1}
                        onClick={() => {
                          const prizes = (form.getValues('prize_details') || []).filter((_, i) => i !== idx)
                            .map((p, i) => ({ ...p, rank: i + 1 }))
                          form.setValue('prize_details', prizes)
                        }}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  ))}
                  {form.formState.errors.prize_details && (
                    <p className="text-sm text-destructive">
                      {form.formState.errors.prize_details.message ?? 'Fix prize details'}
                    </p>
                  )}
                </div>
                <FormField
                  control={form.control}
                  name="rules"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Rules</FormLabel>
                      <FormControl>
                        <Textarea
                          placeholder="e.g., 1. One ticket per transaction&#10;2. Winner must claim prize within 90 days&#10;3. Open to Ghana residents only"
                          className="resize-none"
                          rows={5}
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>Terms and conditions players must agree to</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            )}

            {/* ── Step 5: Logo & Review ── */}
            {currentStep === 5 && (
              <div className="space-y-6">
                {/* Logo upload */}
                <div className="space-y-3">
                  <p className="text-sm font-medium">Game Logo <span className="text-muted-foreground font-normal">(optional)</span></p>
                  <div
                    className="border-2 border-dashed rounded-lg p-6 flex flex-col items-center justify-center gap-3 cursor-pointer hover:border-primary/50 transition-colors"
                    onClick={() => fileInputRef.current?.click()}
                  >
                    {logoPreview ? (
                      <img src={logoPreview} alt="Logo preview" className="h-24 w-24 object-contain rounded-lg" />
                    ) : (
                      <>
                        <Image className="h-10 w-10 text-muted-foreground/40" />
                        <p className="text-sm text-muted-foreground">Click to upload logo</p>
                        <p className="text-xs text-muted-foreground/60">PNG, JPG up to 2MB</p>
                      </>
                    )}
                  </div>
                  <input ref={fileInputRef} type="file" accept="image/*" className="hidden" onChange={handleLogoChange} />
                  {logoPreview && (
                    <Button type="button" variant="ghost" size="sm" className="text-destructive"
                      onClick={() => { setLogoPreview(null); setLogoFile(null); form.setValue('logo_url', '') }}>
                      Remove logo
                    </Button>
                  )}
                </div>

                {/* Review summary */}
                <div className="rounded-lg border divide-y text-sm">
                  {[
                    { label: 'Name',          value: form.watch('name') },
                    { label: 'Code',          value: form.watch('code') },
                    { label: 'Frequency',     value: freqLabel[form.watch('draw_frequency')] },
                    { label: 'Draw Time',     value: form.watch('draw_time') },
                    ...(form.watch('draw_frequency') === 'weekly' || form.watch('draw_frequency') === 'bi_weekly'
                      ? [{ label: 'Draw Days', value: (form.watch('draw_days') || []).join(', ') }]
                      : []),
                    { label: 'Sales Cutoff',  value: `${form.watch('sales_cutoff_minutes')} min before draw` },
                    ...(form.watch('draw_frequency') === 'special'
                      ? [{ label: 'Draw Date', value: form.watch('draw_date') || '—' }]
                      : []),
                    { label: 'Ticket Price',  value: `₵${form.watch('base_price')}` },
                    { label: 'Total Tickets', value: form.watch('total_tickets')?.toLocaleString() },
                    { label: 'Max per Player',value: form.watch('max_tickets_per_player')?.toLocaleString() },
                    { label: 'Prizes',        value: `${(form.watch('prize_details') || []).length} prize(s)` },
                  ].map(row => (
                    <div key={row.label} className="flex justify-between px-4 py-2.5">
                      <span className="text-muted-foreground">{row.label}</span>
                      <span className="font-medium">{row.value || '—'}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}

          </form>
        </Form>

        <DialogFooter className="gap-2 pt-2">
          {currentStep > 1 && (
            <Button variant="outline" onClick={handleBack} disabled={createGameMutation.isPending}>
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back
            </Button>
          )}
          <div className="flex-1" />
          <Button variant="outline" onClick={handleClose} disabled={createGameMutation.isPending}>Cancel</Button>
          <Button onClick={handleNext} disabled={createGameMutation.isPending}>
            {createGameMutation.isPending ? (
              <><Loader2 className="mr-2 h-4 w-4 animate-spin" />Saving...</>
            ) : currentStep === steps.length ? (
              <><Check className="mr-2 h-4 w-4" />Create Game</>
            ) : (
              <>Next<ArrowRight className="ml-2 h-4 w-4" /></>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
