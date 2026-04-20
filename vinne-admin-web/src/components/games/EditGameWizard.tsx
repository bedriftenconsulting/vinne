import { useState, useEffect } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import * as z from 'zod'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import {
  Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage,
} from '@/components/ui/form'
import { Progress } from '@/components/ui/progress'
import { useToast } from '@/hooks/use-toast'
import { ArrowLeft, ArrowRight, Check, Loader2, Info, Trophy, FileText, Calendar, Plus, Trash2 } from 'lucide-react'
import { gameService, type Game, type PrizeDetail } from '@/services/games'

// ─── Schema ──────────────────────────────────────────────────────────────────

const editSchema = z.object({
  // Step 1
  name: z.string().min(3, 'Name must be at least 3 characters'),
  description: z.string().optional(),
  status: z.enum(['Draft', 'Active', 'Suspended']),

  // Step 2
  draw_date: z.string().optional(),
  draw_frequency: z.enum(['daily', 'weekly', 'bi_weekly', 'monthly', 'special']),
  draw_time: z.string().optional(),
  draw_day: z.string().optional(),
  sales_cutoff_minutes: z.number().min(1),
  base_price: z.number().min(0.5, 'Minimum ₵0.50'),
  total_tickets: z.number().min(1),
  max_tickets_per_player: z.number().min(1),

  // Step 3
  prize_details: z.array(z.object({
    rank: z.number().min(1),
    label: z.string().min(1, 'Prize label required'),
    description: z.string().min(1, 'Description required'),
  })).optional(),
  rules: z.string().optional(),
}).superRefine((data, ctx) => {
  if (data.draw_frequency === 'special' && !data.draw_date) {
    ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'Draw date is required for special draws', path: ['draw_date'] })
  }
})

type EditFormData = z.infer<typeof editSchema>

const steps = [
  { id: 1, title: 'Basic Info',      icon: Info },
  { id: 2, title: 'Dates & Tickets', icon: Calendar },
  { id: 3, title: 'Prize & Rules',   icon: FileText },
  { id: 4, title: 'Review',          icon: Trophy },
]

const fieldsPerStep: Record<number, (keyof EditFormData)[]> = {
  1: ['name'],
  2: ['draw_frequency', 'base_price', 'total_tickets', 'max_tickets_per_player', 'draw_date'],
  3: [],
  4: [],
}

interface EditGameWizardProps {
  isOpen: boolean
  onClose: () => void
  game: Game | null
}

export function EditGameWizard({ isOpen, onClose, game }: EditGameWizardProps) {
  const [currentStep, setCurrentStep] = useState(1)
  const { toast } = useToast()
  const queryClient = useQueryClient()

  // Fields that are locked once a game is ACTIVE (to prevent disrupting live competitions)
  const isActive = game?.status?.toUpperCase() === 'ACTIVE'
  const locked = isActive // lock critical fields when active

  const form = useForm<EditFormData>({
    resolver: zodResolver(editSchema),
    defaultValues: {
      name: '', description: '', status: 'Draft',
      draw_date: '',
      draw_frequency: 'daily', draw_time: '20:00', draw_day: 'Friday',
      sales_cutoff_minutes: 30,
      base_price: 1, total_tickets: 1000, max_tickets_per_player: 10,
      prize_details: [{ rank: 1, label: '', description: '' }], rules: '',
    },
  })

  // Fetch fresh game data by ID so dates/prizes always reflect the latest DB state.
  // gcTime: 0 means React Query never keeps stale data for this query.
  const { data: freshGame, refetch: refetchGame } = useQuery({
    queryKey: ['game', game?.id],
    queryFn: () => gameService.getGame(game!.id),
    enabled: !!game?.id && isOpen,
    staleTime: 0,
    gcTime: 0,
  })

  // When the dialog opens, remove any stale cached entry so the query fetches
  // fresh data from the network. We skip removal if the cache was just seeded by
  // onSuccess (i.e., the data is less than 3 seconds old) to avoid a race condition.
  useEffect(() => {
    if (!isOpen || !game?.id) return
    const query = queryClient.getQueryState(['game', game.id])
    const isRecentlySet = query?.dataUpdatedAt && (Date.now() - query.dataUpdatedAt) < 3000
    if (!isRecentlySet) {
      queryClient.removeQueries({ queryKey: ['game', game.id] })
    }
  }, [isOpen, game?.id, queryClient])

  const toDateInput = (val: string | undefined) => val ? val.split('T')[0] : ''

  // Reset step to 1 whenever a new game is opened
  useEffect(() => {
    if (game) setCurrentStep(1)
  }, [game])

  // Populate form — prefer fresh API data over potentially-stale list data
  useEffect(() => {
    const g = freshGame ?? game
    if (!g) return
    form.reset({
      name: g.name || '',
      description: g.description || '',
      status: (g.status as 'Draft' | 'Active' | 'Suspended') || 'Draft',
      draw_date: toDateInput(g.draw_date || g.end_date),
      draw_frequency: (g.draw_frequency as EditFormData['draw_frequency']) || 'daily',
      draw_time: g.draw_time || '20:00',
      draw_day: g.draw_days?.[0] || 'Friday',
      sales_cutoff_minutes: g.sales_cutoff_minutes || 30,
      base_price: g.base_price || 1,
      total_tickets: g.total_tickets || 1000,
      max_tickets_per_player: g.max_tickets_per_player || 10,
      prize_details: (g.prize_details && g.prize_details.length > 0)
        ? g.prize_details
        : [{ rank: 1, label: '', description: '' }],
      rules: g.rules || '',
    })
  }, [freshGame, game, form])

  const updateMutation = useMutation({
    mutationFn: async (data: EditFormData) => {
      if (!game?.id) throw new Error('No game ID')
      return gameService.updateGame(game.id, {
        name: data.name,
        description: data.description,
        base_price: data.base_price,
        total_tickets: data.total_tickets,
        max_tickets_per_player: data.max_tickets_per_player,
        draw_frequency: data.draw_frequency,
        draw_days: data.draw_day ? [data.draw_day] : [],
        draw_time: data.draw_time,
        sales_cutoff_minutes: data.sales_cutoff_minutes,
        draw_date: freq === 'special' ? (data.draw_date || '') : undefined,
        prize_details: data.prize_details as PrizeDetail[] | undefined,
        rules: data.rules,
      })
    },
    onSuccess: (updatedGame) => {
      // Immediately seed the RQ cache with the response data so the
      // form repopulates correctly next time the dialog opens — no extra round-trip
      if (game?.id && updatedGame) {
        queryClient.setQueryData(['game', game.id], updatedGame)
      }
      queryClient.invalidateQueries({ queryKey: ['games'] })
      queryClient.invalidateQueries({ queryKey: ['games-list'] })
      // Also remove the per-game entry so the next dialog open fetches from DB,
      // bypassing any stale Redis cache that hasn't propagated yet
      if (game?.id) {
        queryClient.removeQueries({ queryKey: ['game', game.id] })
      }
      toast({ title: 'Competition updated successfully' })
      onClose(); setCurrentStep(1)
    },
    onError: (error: unknown) => {
      const msg = (error as { response?: { data?: { message?: string } } })?.response?.data?.message
      toast({ title: 'Error', description: msg || 'Failed to update', variant: 'destructive' })
    },
  })

  const handleNext = async () => {
    const valid = await form.trigger(fieldsPerStep[currentStep])
    if (!valid) return
    if (currentStep === steps.length) {
      updateMutation.mutate(form.getValues())
    } else {
      setCurrentStep(s => s + 1)
    }
  }

  const progress = ((currentStep - 1) / (steps.length - 1)) * 100
  const freq = form.watch('draw_frequency')
  const showDrawDay = freq === 'weekly' || freq === 'bi_weekly'

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Edit Competition</DialogTitle>
          <DialogDescription>Update competition settings</DialogDescription>
        </DialogHeader>

        {/* Progress */}
        <div className="space-y-2">
          <Progress value={progress} className="h-1.5" />
          <div className="flex justify-between">
            {steps.map(step => {
              const Icon = step.icon
              return (
                <div key={step.id} className={`flex items-center gap-1.5 text-xs ${
                  step.id === currentStep ? 'text-primary font-medium'
                  : step.id < currentStep ? 'text-muted-foreground'
                  : 'text-muted-foreground/40'
                }`}>
                  <Icon className="h-3.5 w-3.5" />
                  <span className="hidden sm:inline">{step.title}</span>
                </div>
              )
            })}
          </div>
        </div>

        <Form {...form}>
          <form className="space-y-4 pt-2">

            {/* ── Step 1: Basic Info ── */}
            {currentStep === 1 && (
              <div className="space-y-4">
                <FormField control={form.control} name="name" render={({ field }) => (
                  <FormItem>
                    <FormLabel>Competition Name</FormLabel>
                    <FormControl><Input {...field} /></FormControl>
                    <FormMessage />
                  </FormItem>
                )} />

                <FormField control={form.control} name="description" render={({ field }) => (
                  <FormItem>
                    <FormLabel>Description <span className="text-muted-foreground font-normal">(optional)</span></FormLabel>
                    <FormControl>
                      <Textarea className="resize-none" rows={2} {...field} value={field.value ?? ''} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )} />

              </div>
            )}

            {/* ── Step 2: Dates & Tickets ── */}
            {currentStep === 2 && (
              <div className="space-y-4">
                {/* Lock banner for active games */}
                {locked && (
                  <div className="flex items-start gap-2 rounded-lg border border-yellow-500/30 bg-yellow-500/10 px-3 py-2.5">
                    <span className="text-yellow-500 mt-0.5">🔒</span>
                    <div>
                      <p className="text-sm font-medium text-yellow-500">Some fields are locked</p>
                      <p className="text-xs text-muted-foreground">Draw frequency, price, draw date and ticket count cannot be changed while the game is Active to protect existing ticket holders.</p>
                    </div>
                  </div>
                )}

                {/* Special games: one Draw Date only */}
                {freq === 'special' && (
                  <FormField control={form.control} name="draw_date" render={({ field }) => (
                    <FormItem>
                      <FormLabel>Draw Date <span className="text-destructive">*</span></FormLabel>
                      <FormControl><Input type="date" {...field} value={field.value ?? ''} disabled={locked} /></FormControl>
                      {locked && <p className="text-xs text-yellow-500">🔒 Locked — game is active</p>}
                      {!locked && <p className="text-xs text-muted-foreground">The date this one-time draw takes place.</p>}
                      <FormMessage />
                    </FormItem>
                  )} />
                )}

                <div className="grid grid-cols-2 gap-4">
                  <FormField control={form.control} name="draw_frequency" render={({ field }) => (
                    <FormItem>
                      <FormLabel>Draw Frequency {locked && <span className="text-yellow-500 text-xs">🔒</span>}</FormLabel>
                      <Select onValueChange={field.onChange} value={field.value} disabled={locked}>
                        <FormControl><SelectTrigger><SelectValue /></SelectTrigger></FormControl>
                        <SelectContent>
                          <SelectItem value="daily">Daily</SelectItem>
                          <SelectItem value="weekly">Weekly</SelectItem>
                          <SelectItem value="bi_weekly">Bi-Weekly</SelectItem>
                          <SelectItem value="monthly">Monthly</SelectItem>
                          <SelectItem value="special">Special (once)</SelectItem>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )} />
                  <FormField control={form.control} name="draw_time" render={({ field }) => (
                    <FormItem>
                      <FormLabel>Draw Time {locked && <span className="text-yellow-500 text-xs">🔒</span>}</FormLabel>
                      <FormControl><Input type="time" {...field} value={field.value ?? ''} disabled={locked} /></FormControl>
                      <FormDescription>Ghana time (GMT+0)</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )} />
                </div>

                {showDrawDay && (
                  <FormField control={form.control} name="draw_day" render={({ field }) => (
                    <FormItem>
                      <FormLabel>Draw Day</FormLabel>
                      <Select onValueChange={field.onChange} value={field.value}>
                        <FormControl><SelectTrigger><SelectValue /></SelectTrigger></FormControl>
                        <SelectContent>
                          {['Monday','Tuesday','Wednesday','Thursday','Friday','Saturday','Sunday'].map(d => (
                            <SelectItem key={d} value={d}>{d}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )} />
                )}

                <FormField control={form.control} name="sales_cutoff_minutes" render={({ field }) => (
                  <FormItem>
                    <FormLabel>Sales Cutoff</FormLabel>
                    <Select value={String(field.value)} onValueChange={v => field.onChange(parseInt(v))}>
                      <FormControl><SelectTrigger><SelectValue /></SelectTrigger></FormControl>
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
                    <FormMessage />
                  </FormItem>
                )} />

                <div className="grid grid-cols-3 gap-4">
                  <FormField control={form.control} name="base_price" render={({ field }) => (
                    <FormItem>
                      <FormLabel>Ticket Price (₵) {locked && <span className="text-yellow-500 text-xs">🔒</span>}</FormLabel>
                      <FormControl>
                        <Input type="number" step="0.50" min="0.50" {...field}
                          value={field.value ?? ''}
                          disabled={locked}
                          onChange={e => { const v = parseFloat(e.target.value); field.onChange(isNaN(v) ? '' : v) }} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )} />
                  <FormField control={form.control} name="total_tickets" render={({ field }) => (
                    <FormItem>
                      <FormLabel>Total Tickets {locked && <span className="text-yellow-500 text-xs">🔒</span>}</FormLabel>
                      <FormControl>
                        <Input type="number" min="1" {...field}
                          value={field.value ?? ''}
                          disabled={locked}
                          onChange={e => { const v = parseInt(e.target.value); field.onChange(isNaN(v) ? '' : v) }} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )} />
                  <FormField control={form.control} name="max_tickets_per_player" render={({ field }) => (
                    <FormItem>
                      <FormLabel>Max per Player</FormLabel>
                      <FormControl>
                        <Input type="number" min="1" {...field}
                          value={field.value ?? ''}
                          onChange={e => { const v = parseInt(e.target.value); field.onChange(isNaN(v) ? '' : v) }} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )} />
                </div>
              </div>
            )}

            {/* ── Step 3: Prize & Rules ── */}
            {currentStep === 3 && (
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
                      <div className="w-10 pt-2 text-center text-sm font-semibold text-muted-foreground">
                        #{idx + 1}
                      </div>
                      <Input
                        placeholder="e.g. 1st Prize"
                        value={form.watch(`prize_details.${idx}.label`) ?? ''}
                        onChange={e => {
                          const prizes = [...(form.getValues('prize_details') || [])]
                          prizes[idx] = { ...prizes[idx], label: e.target.value }
                          form.setValue('prize_details', prizes)
                        }}
                      />
                      <Input
                        placeholder="e.g. BMW 3 Series"
                        value={form.watch(`prize_details.${idx}.description`) ?? ''}
                        onChange={e => {
                          const prizes = [...(form.getValues('prize_details') || [])]
                          prizes[idx] = { ...prizes[idx], description: e.target.value }
                          form.setValue('prize_details', prizes)
                        }}
                      />
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
                </div>
                <FormField control={form.control} name="rules" render={({ field }) => (
                  <FormItem>
                    <FormLabel>Rules</FormLabel>
                    <FormControl>
                      <Textarea placeholder="e.g., 1. One ticket per transaction&#10;2. Open to Ghana residents only"
                        className="resize-none" rows={5} {...field} value={field.value ?? ''} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )} />
              </div>
            )}

            {/* ── Step 4: Review ── */}
            {currentStep === 4 && (
              <div className="space-y-4">
                <div className="rounded-lg border divide-y text-sm">
                  {[
                    { label: 'Name',          value: form.watch('name') },
                    { label: 'Frequency',     value: form.watch('draw_frequency')?.replace('_', '-') },
                    { label: 'Draw Time',     value: form.watch('draw_time') },
                    ...(showDrawDay ? [{ label: 'Draw Day', value: form.watch('draw_day') }] : []),
                    ...(freq === 'special' ? [
                      { label: 'Draw Date', value: form.watch('draw_date') || '—' },
                    ] : []),
                    { label: 'Ticket Price',  value: `₵${form.watch('base_price')}` },
                    { label: 'Total Tickets', value: form.watch('total_tickets')?.toLocaleString() },
                    { label: 'Max per Player',value: form.watch('max_tickets_per_player')?.toLocaleString() },
                  ].map(row => (
                    <div key={row.label} className="flex justify-between px-4 py-2.5">
                      <span className="text-muted-foreground">{row.label}</span>
                      <span className="font-medium">{row.value || '—'}</span>
                    </div>
                  ))}
                </div>
                <p className="text-xs text-muted-foreground bg-muted/50 rounded-lg p-3">
                  Changes will be applied immediately.
                </p>
              </div>
            )}

          </form>
        </Form>

        <DialogFooter className="gap-2 pt-2">
          {currentStep > 1 && (
            <Button variant="outline" onClick={() => setCurrentStep(s => s - 1)} disabled={updateMutation.isPending}>
              <ArrowLeft className="mr-2 h-4 w-4" />Back
            </Button>
          )}
          <div className="flex-1" />
          <Button variant="outline" onClick={onClose} disabled={updateMutation.isPending}>Cancel</Button>
          <Button onClick={handleNext} disabled={updateMutation.isPending}>
            {updateMutation.isPending ? (
              <><Loader2 className="mr-2 h-4 w-4 animate-spin" />Saving...</>
            ) : currentStep === steps.length ? (
              <><Check className="mr-2 h-4 w-4" />Save Changes</>
            ) : (
              <>Next<ArrowRight className="ml-2 h-4 w-4" /></>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
