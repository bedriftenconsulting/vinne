/* eslint-disable @typescript-eslint/no-explicit-any */
import React, { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import * as z from 'zod'
import { useDebounce } from '@/hooks/use-debounce'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { toast } from '@/hooks/use-toast'
import { agentService } from '@/services/agents'
import { walletService } from '@/services/wallet'
import {
  Wallet,
  Search,
  DollarSign,
  CreditCard,
  Users,
  Store,
  X,
  ArrowUpRight,
  Gift,
  Undo2,
  AlertTriangle,
} from 'lucide-react'
import { startOfDay, endOfDay, differenceInHours } from 'date-fns'
import { formatInGhanaTime } from '@/lib/date-utils'

// Form validation schema
const creditFormSchema = z.object({
  entityType: z.enum(['agent', 'retailer']),
  entityId: z.string().min(1, 'Please select an agent or retailer'),
  amount: z.number().min(0.01, 'Amount must be greater than 0'),
  notes: z.string().optional(),
})

type CreditFormData = z.infer<typeof creditFormSchema>

export default function WalletCredits() {
  const queryClient = useQueryClient()
  const [searchTerm, setSearchTerm] = useState('')
  const debouncedSearchTerm = useDebounce(searchTerm, 300)
  const [selectedEntity, setSelectedEntity] = useState<{
    id: string
    name: string
    code?: string
    agent_code?: string
    retailer_code?: string
    phone_number?: string
    commission_percentage?: number
  } | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  // Reversal dialog state
  const [reversalDialog, setReversalDialog] = useState<{
    open: boolean
    transaction: any | null
  }>({ open: false, transaction: null })
  const [reversalReason, setReversalReason] = useState('')
  const [reversalConfirmed, setReversalConfirmed] = useState(false)

  const {
    register,
    handleSubmit,
    watch,
    reset,
    setValue,
    formState: { errors },
  } = useForm<CreditFormData>({
    resolver: zodResolver(creditFormSchema),
    defaultValues: {
      entityType: 'agent',
    },
  })

  const entityType = watch('entityType')
  const amount = watch('amount')

  // Calculate grossed up amount where base amount is 70% of final amount
  const calculateGrossedUp = useMemo(() => {
    if (amount > 0) {
      // Base amount is 70% of the final grossed-up amount
      // Formula: final amount = base amount / 0.7
      const finalAmount = amount / 0.7
      const grossUpAmount = finalAmount - amount
      const grossUpPercentage = 30
      return {
        baseAmount: amount.toFixed(2),
        grossUpAmount: grossUpAmount.toFixed(2),
        totalAmount: finalAmount.toFixed(2),
        grossUpPercentage,
      }
    }
    return {
      baseAmount: '0.00',
      grossUpAmount: '0.00',
      totalAmount: '0.00',
      grossUpPercentage: 30,
    }
  }, [amount])

  // Fetch agents with search
  const { data: agents } = useQuery({
    queryKey: ['agents-for-credit', debouncedSearchTerm],
    queryFn: async () => {
      const response = await agentService.getAgents(
        1,
        100,
        debouncedSearchTerm ? { name: debouncedSearchTerm } : {}
      )
      return response.data || []
    },
    enabled: entityType === 'agent',
  })

  // Fetch retailers with search
  const { data: retailers } = useQuery({
    queryKey: ['retailers-for-credit', debouncedSearchTerm],
    queryFn: async () => {
      const response = await agentService.getRetailers(
        1,
        100,
        debouncedSearchTerm ? { name: debouncedSearchTerm } : {}
      )
      return response.data || []
    },
    enabled: entityType === 'retailer',
  })

  // Fetch recent credit transactions - ONLY CREDIT type transactions
  const { data: recentCredits, refetch: refetchCredits } = useQuery({
    queryKey: ['recent-wallet-credits'],
    queryFn: async () => {
      try {
        // Fetch all transactions with CREDIT filter from the global transactions endpoint
        const response = await walletService.getAllTransactions({
          transaction_types: ['CREDIT'], // Only fetch CREDIT transactions
          statuses: ['COMPLETED'], // Only show completed credits
          page: 1,
          page_size: 50, // Get enough to show recent activity
          sort_by: 'created_at',
          sort_order: 'desc',
        })

        // Map transactions to include entity names from wallet_owner_name
        // Fallback to wallet_owner_id if name is not available
        return response.transactions.map(tx => {
          const fallbackName = tx.wallet_owner_id
            ? `ID: ${tx.wallet_owner_id.substring(0, 8)}...`
            : 'Unknown'

          return {
            ...tx,
            entity_type: tx.wallet_type.includes('AGENT') ? 'agent' : 'retailer',
            entity_name: tx.wallet_owner_name || tx.wallet_owner_code || fallbackName,
          }
        })
      } catch (error) {
        console.error('Failed to fetch recent credits:', error)
        return []
      }
    },
  })

  // Use entities directly since they're already filtered by the backend
  const filteredEntities = React.useMemo(() => {
    const entities = entityType === 'agent' ? agents : retailers
    return entities || []
  }, [entityType, agents, retailers])

  // Calculate today's stats - only for CREDIT transactions
  const todayStats = React.useMemo(() => {
    if (!recentCredits || recentCredits.length === 0) {
      return {
        baseAmount: 0,
        numberOfTransfers: 0,
        totalWithBonus: 0,
      }
    }

    const today = new Date()
    const startOfToday = startOfDay(today)
    const endOfToday = endOfDay(today)

    // Filter transactions for today - already filtered to CREDIT type in query
    const todayTransactions = recentCredits.filter(credit => {
      const creditDate = new Date(credit.created_at)
      return creditDate >= startOfToday && creditDate <= endOfToday && credit.type === 'CREDIT'
    })

    // Calculate totals - amounts are already in GHS (converted from pesewas)
    // The amount from backend already includes the 30% gross-up
    const totalWithBonus = todayTransactions.reduce((sum, credit) => sum + credit.amount, 0)
    const numberOfTransfers = todayTransactions.length

    // Reverse calculate base amount: total = base / 0.7, so base = total * 0.7
    // Base amount is 70% of the total grossed-up amount
    const baseAmount = totalWithBonus * 0.7

    return {
      baseAmount,
      numberOfTransfers,
      totalWithBonus,
    }
  }, [recentCredits])

  // Helper function to check if transaction can be reversed
  const canReverseTransaction = (transaction: any) => {
    if (!transaction) return false

    // Only CREDIT transactions can be reversed
    if (transaction.type !== 'CREDIT') return false

    // Only COMPLETED transactions can be reversed
    if (transaction.status !== 'COMPLETED') return false

    // Transaction must not be already reversed
    if (transaction.status === 'REVERSED') return false

    // Check 24-hour time limit
    const transactionDate = new Date(transaction.created_at)
    const hoursSinceTransaction = differenceInHours(new Date(), transactionDate)

    return hoursSinceTransaction < 24
  }

  // Reversal mutation
  const reversalMutation = useMutation({
    mutationFn: async ({ transactionId, reason }: { transactionId: string; reason: string }) => {
      return await walletService.reverseTransaction(transactionId, {
        reason,
        confirmed: true,
      })
    },
    onSuccess: response => {
      toast({
        title: 'Transaction Reversed',
        description: `Successfully reversed ${formatCurrency(response.data.reversed_amount)}. ${
          response.data.balance_is_negative ? '⚠️ Wallet balance is now negative.' : ''
        }`,
        variant: 'default',
      })

      // Close dialog and reset state
      setReversalDialog({ open: false, transaction: null })
      setReversalReason('')
      setReversalConfirmed(false)

      // Refresh the transactions list
      queryClient.invalidateQueries({ queryKey: ['wallet-credits'] })
    },
    onError: (error: any) => {
      toast({
        title: 'Reversal Failed',
        description:
          error.response?.data?.message || error.message || 'Failed to reverse transaction',
        variant: 'destructive',
      })
    },
  })

  // Handle reversal submission
  const handleReversal = () => {
    if (!reversalDialog.transaction) return

    if (!reversalReason || reversalReason.length < 20) {
      toast({
        title: 'Invalid Reason',
        description: 'Reversal reason must be at least 20 characters',
        variant: 'destructive',
      })
      return
    }

    if (!reversalConfirmed) {
      toast({
        title: 'Confirmation Required',
        description: 'Please confirm that you want to reverse this transaction',
        variant: 'destructive',
      })
      return
    }

    reversalMutation.mutate({
      transactionId: reversalDialog.transaction.id,
      reason: reversalReason,
    })
  }

  const onSubmit = async (data: CreditFormData) => {
    setIsSubmitting(true)
    try {
      // Send only the base amount - backend will apply 30% gross-up
      const baseAmount = data.amount

      let response
      if (data.entityType === 'agent') {
        // Credit agent wallet - backend will calculate and apply 30% gross-up
        response = await walletService.creditAgentWallet(data.entityId, {
          amount: baseAmount, // Send only base amount, backend applies 30% gross-up
          credit_type: 'payment',
          payment_method: 'manual',
          notes: data.notes || '', // Default to empty string if not provided
        })
      } else {
        // Credit retailer wallet - backend will calculate and apply 30% gross-up
        response = await walletService.creditRetailerWallet(data.entityId, {
          wallet_type: 'stake', // Default to stake wallet
          amount: baseAmount, // Send only base amount, backend applies 30% gross-up
          credit_type: 'payment',
          payment_method: 'manual',
          notes: data.notes || '', // Default to empty string if not provided
        })
      }

      // Extract amounts from backend response
      const responseData = response.data || {}
      const creditedBaseAmount = responseData.base_amount || baseAmount
      const creditedGrossUpAmount = responseData.commission_amount || 0
      const creditedTotalAmount = responseData.gross_amount || baseAmount

      // Show success message with breakdown from backend response
      const successMessage = `GH₵${creditedTotalAmount.toFixed(2)} has been credited to the ${data.entityType}'s wallet (Base: GH₵${creditedBaseAmount.toFixed(2)} + Gross-up: GH₵${creditedGrossUpAmount.toFixed(2)}).`

      toast({
        title: 'Wallet Credited Successfully',
        description: successMessage,
      })

      // Reset form
      reset()
      setSelectedEntity(null)
      setSearchTerm('')

      // Refresh credit history
      refetchCredits()
    } catch (error) {
      toast({
        title: 'Credit Failed',
        description:
          (error as { response?: { data?: { error?: string } } })?.response?.data?.error ||
          'Failed to credit wallet. Please try again.',
        variant: 'destructive',
      })
    } finally {
      setIsSubmitting(false)
    }
  }

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('en-GH', {
      style: 'currency',
      currency: 'GHS',
      minimumFractionDigits: 2,
    }).format(amount) // Amount is already in GHS from API gateway
  }

  return (
    <div className="p-3 sm:p-4 md:p-6 space-y-3 sm:space-y-4 md:space-y-6">
      <div className="space-y-1">
        <h1 className="text-xl sm:text-2xl md:text-3xl font-bold tracking-tight">Wallet Credits</h1>
        <p className="text-xs sm:text-sm text-muted-foreground">
          Manually credit stake wallets for agents and retailers
        </p>
      </div>

      {/* Today's Stats Cards */}
      <div className="grid gap-3 sm:gap-4 grid-cols-1 sm:grid-cols-2 md:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-xs sm:text-sm font-medium">Credits Sold (Today)</CardTitle>
            <DollarSign className="h-3 sm:h-4 w-3 sm:w-4 text-muted-foreground shrink-0" />
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">
              GH₵ {todayStats.baseAmount.toFixed(2)}
            </div>
            <p className="text-xs text-muted-foreground mt-1">Base amount sold (70% of total)</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-xs sm:text-sm font-medium">
              No. of Transfers (Today)
            </CardTitle>
            <ArrowUpRight className="h-3 sm:h-4 w-3 sm:w-4 text-muted-foreground shrink-0" />
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">{todayStats.numberOfTransfers}</div>
            <p className="text-xs text-muted-foreground mt-1">Credit transactions today</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-xs sm:text-sm font-medium">Total with Bonus</CardTitle>
            <Gift className="h-3 sm:h-4 w-3 sm:w-4 text-muted-foreground shrink-0" />
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold text-green-600">
              GH₵ {todayStats.totalWithBonus.toFixed(2)}
            </div>
            <p className="text-xs text-muted-foreground mt-1">Including 30% gross-up bonus</p>
          </CardContent>
        </Card>
      </div>

      {/* Credit Form Card - Full Width */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base sm:text-lg">
            <Wallet className="h-4 sm:h-5 w-4 sm:w-5" />
            Credit Stake Wallet
          </CardTitle>
          <CardDescription className="text-xs sm:text-sm">
            Add funds to agent or retailer stake wallets
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="max-w-2xl mx-auto">
            <form onSubmit={handleSubmit(onSubmit)} className="space-y-3 sm:space-y-4">
              {/* Entity Type Selection */}
              <div className="space-y-2">
                <Label className="text-xs sm:text-sm">Select Entity Type</Label>
                <RadioGroup
                  value={entityType}
                  onValueChange={value => {
                    setValue('entityType', value as 'agent' | 'retailer')
                    setSelectedEntity(null)
                    setValue('entityId', '')
                  }}
                >
                  <div className="flex items-center space-x-2">
                    <RadioGroupItem value="agent" id="agent" />
                    <Label
                      htmlFor="agent"
                      className="flex items-center gap-2 cursor-pointer text-xs sm:text-sm"
                    >
                      <Users className="h-3 sm:h-4 w-3 sm:w-4" />
                      Agent
                    </Label>
                  </div>
                  <div className="flex items-center space-x-2">
                    <RadioGroupItem value="retailer" id="retailer" />
                    <Label
                      htmlFor="retailer"
                      className="flex items-center gap-2 cursor-pointer text-xs sm:text-sm"
                    >
                      <Store className="h-3 sm:h-4 w-3 sm:w-4" />
                      Retailer
                    </Label>
                  </div>
                </RadioGroup>
              </div>

              {/* Entity Search and Selection */}
              <div className="space-y-2">
                <Label className="text-xs sm:text-sm">
                  Select {entityType === 'agent' ? 'Agent' : 'Retailer'}
                </Label>
                {!selectedEntity ? (
                  <>
                    <div className="relative">
                      <Search className="absolute left-2 top-2.5 h-3 sm:h-4 w-3 sm:w-4 text-muted-foreground" />
                      <Input
                        placeholder={`Search ${entityType}s by name, code, or phone...`}
                        className="pl-8 text-xs sm:text-sm"
                        value={searchTerm}
                        onChange={e => setSearchTerm(e.target.value)}
                      />
                    </div>
                    {filteredEntities && filteredEntities.length > 0 && searchTerm && (
                      <div className="border rounded-md max-h-48 overflow-y-auto">
                        {filteredEntities.slice(0, 10).map(entity => (
                          <div
                            key={entity.id}
                            className="p-2 hover:bg-accent cursor-pointer"
                            onClick={() => {
                              setSelectedEntity(entity)
                              setValue('entityId', entity.id)
                              setSearchTerm('')
                            }}
                          >
                            <div className="font-medium text-xs sm:text-sm">{entity.name}</div>
                            <div className="text-xs text-muted-foreground">
                              {(entity as any).code ||
                                (entity as any).agent_code ||
                                (entity as any).retailer_code}{' '}
                              • {(entity as any).phone_number || 'No phone'}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </>
                ) : (
                  <div className="p-3 bg-muted rounded-md flex justify-between items-start">
                    <div className="min-w-0 flex-1">
                      <div className="font-medium text-xs sm:text-sm break-words">
                        {selectedEntity.name}
                      </div>
                      <div className="text-xs text-muted-foreground break-words">
                        {(selectedEntity as any).code ||
                          (selectedEntity as any).agent_code ||
                          (selectedEntity as any).retailer_code}{' '}
                        • {(selectedEntity as any).phone_number || 'No phone'}
                      </div>
                    </div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => {
                        setSelectedEntity(null)
                        setValue('entityId', '')
                        setSearchTerm('')
                      }}
                      className="h-8 px-2 shrink-0"
                    >
                      <X className="h-3 sm:h-4 w-3 sm:w-4" />
                    </Button>
                  </div>
                )}
                {errors.entityId && (
                  <p className="text-xs sm:text-sm text-destructive">{errors.entityId.message}</p>
                )}
              </div>

              {/* Amount */}
              <div className="space-y-2">
                <Label htmlFor="amount" className="text-xs sm:text-sm">
                  Amount (GHS)
                </Label>
                <div className="relative">
                  <DollarSign className="absolute left-2 top-2.5 h-3 sm:h-4 w-3 sm:w-4 text-muted-foreground" />
                  <Input
                    id="amount"
                    type="number"
                    step="0.01"
                    placeholder="0.00"
                    className="pl-8 text-xs sm:text-sm"
                    {...register('amount', { valueAsNumber: true })}
                  />
                </div>
                {errors.amount && (
                  <p className="text-xs sm:text-sm text-destructive">{errors.amount.message}</p>
                )}
              </div>

              {/* Notes */}
              <div className="space-y-2">
                <Label htmlFor="notes" className="text-xs sm:text-sm">
                  Notes <span className="text-muted-foreground">(optional)</span>
                </Label>
                <Textarea
                  id="notes"
                  placeholder="e.g., Payment received for stake wallet top-up"
                  rows={3}
                  className="text-xs sm:text-sm"
                  {...register('notes')}
                />
                {errors.notes && (
                  <p className="text-xs sm:text-sm text-destructive">{errors.notes.message}</p>
                )}
              </div>

              {/* Grossed-Up Amount Breakdown - Shows for all entity types and credit types */}
              {amount > 0 && (
                <div className="p-3 sm:p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-md space-y-2">
                  <div className="text-xs sm:text-sm font-semibold text-blue-900 dark:text-blue-100 mb-2">
                    Credit Amount Breakdown
                  </div>
                  <div className="flex justify-between items-center text-xs sm:text-sm">
                    <span className="text-muted-foreground">Base Amount:</span>
                    <span className="font-medium">GH₵ {calculateGrossedUp.baseAmount}</span>
                  </div>
                  <div className="flex justify-between items-center text-xs sm:text-sm">
                    <span className="text-muted-foreground">
                      Gross-up ({calculateGrossedUp.grossUpPercentage}%):
                    </span>
                    <span className="font-medium text-blue-600 dark:text-blue-400">
                      + GH₵ {calculateGrossedUp.grossUpAmount}
                    </span>
                  </div>
                  <div className="border-t border-blue-200 dark:border-blue-700 pt-2 flex justify-between items-center">
                    <span className="font-semibold text-xs sm:text-sm">
                      Total Amount to Credit:
                    </span>
                    <span className="text-base sm:text-lg font-bold text-green-600 dark:text-green-400">
                      GH₵ {calculateGrossedUp.totalAmount}
                    </span>
                  </div>
                  <div className="mt-2 pt-2 border-t border-blue-200 dark:border-blue-700">
                    <p className="text-xs text-blue-700 dark:text-blue-300">
                      The base amount represents 70% of the final grossed-up amount. The{' '}
                      {entityType} will receive the total amount shown above (base amount grossed up
                      by 30%).
                    </p>
                  </div>
                </div>
              )}

              {/* Submit Button */}
              <Button
                type="submit"
                className="w-full text-xs sm:text-sm"
                disabled={isSubmitting || !selectedEntity}
              >
                {isSubmitting ? (
                  'Processing...'
                ) : (
                  <>
                    <CreditCard className="mr-2 h-3 sm:h-4 w-3 sm:w-4" />
                    {amount > 0
                      ? `Credit GH₵ ${calculateGrossedUp.totalAmount} (incl. gross-up)`
                      : 'Credit Wallet'}
                  </>
                )}
              </Button>
            </form>
          </div>
        </CardContent>
      </Card>

      {/* Credit History Table */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base sm:text-lg">Credit History</CardTitle>
          <CardDescription className="text-xs sm:text-sm">
            Complete history of manual wallet credits
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-xs sm:text-sm">Date</TableHead>
                  <TableHead className="text-xs sm:text-sm">Entity</TableHead>
                  <TableHead className="text-xs sm:text-sm">Type</TableHead>
                  <TableHead className="text-xs sm:text-sm">Amount</TableHead>
                  <TableHead className="text-xs sm:text-sm">Credit Type</TableHead>
                  <TableHead className="text-xs sm:text-sm">Reference</TableHead>
                  <TableHead className="text-xs sm:text-sm">Credited By</TableHead>
                  <TableHead className="text-xs sm:text-sm">Status</TableHead>
                  <TableHead className="text-xs sm:text-sm text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {recentCredits &&
                  recentCredits.map(credit => (
                    <TableRow key={credit.id}>
                      <TableCell className="text-xs sm:text-sm">
                        {credit.created_at
                          ? formatInGhanaTime(credit.created_at, 'MMM dd, yyyy HH:mm')
                          : 'Unknown'}
                      </TableCell>
                      <TableCell className="text-xs sm:text-sm">
                        <div>
                          <p className="font-medium">{credit.entity_name || 'Unknown'}</p>
                          <p className="text-xs text-muted-foreground">
                            {credit.entity_type === 'agent' ? 'Agent' : 'Retailer'}
                          </p>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className="text-xs">
                          {credit.entity_type || 'agent'}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-medium text-green-600 text-xs sm:text-sm">
                        +{formatCurrency(credit.amount)}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={credit.type === 'CREDIT' ? 'default' : 'secondary'}
                          className="text-xs"
                        >
                          {credit.type || 'CREDIT'}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs sm:text-sm">
                        {credit.reference || '-'}
                      </TableCell>
                      <TableCell className="text-xs sm:text-sm">
                        {credit.description || 'Manual Credit'}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={credit.status === 'COMPLETED' ? 'default' : 'secondary'}
                          className="text-xs"
                        >
                          {credit.status || 'COMPLETED'}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        {canReverseTransaction(credit) ? (
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              setReversalDialog({ open: true, transaction: credit })
                              setReversalReason('')
                              setReversalConfirmed(false)
                            }}
                            className="text-xs"
                          >
                            <Undo2 className="h-3 w-3 mr-1" />
                            Reverse
                          </Button>
                        ) : credit.status === 'REVERSED' ? (
                          <Badge variant="secondary" className="text-xs">
                            Reversed
                          </Badge>
                        ) : (
                          <span className="text-xs text-muted-foreground">-</span>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* Reversal Confirmation Dialog */}
      <Dialog
        open={reversalDialog.open}
        onOpenChange={open => {
          if (!open) {
            setReversalDialog({ open: false, transaction: null })
            setReversalReason('')
            setReversalConfirmed(false)
          }
        }}
      >
        <DialogContent className="sm:max-w-[550px]">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-yellow-600" />
              Reverse Transaction
            </DialogTitle>
            <DialogDescription>
              This action will reverse the credit transaction and debit the wallet. This operation
              cannot be undone.
            </DialogDescription>
          </DialogHeader>

          {reversalDialog.transaction && (
            <div className="space-y-4">
              {/* Transaction Details */}
              <div className="rounded-lg border p-4 bg-muted/50">
                <h4 className="font-semibold mb-2 text-sm">Transaction Details</h4>
                <div className="space-y-1 text-sm">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Entity:</span>
                    <span className="font-medium">
                      {reversalDialog.transaction.entity_name || 'Unknown'}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Amount:</span>
                    <span className="font-semibold text-green-600">
                      +{formatCurrency(reversalDialog.transaction.amount)}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Date:</span>
                    <span>
                      {formatInGhanaTime(
                        reversalDialog.transaction.created_at,
                        'MMM dd, yyyy HH:mm'
                      )}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Reference:</span>
                    <span className="font-mono text-xs">
                      {reversalDialog.transaction.reference || '-'}
                    </span>
                  </div>
                </div>
              </div>

              {/* Warning Message */}
              <div className="rounded-lg border border-yellow-200 bg-yellow-50 p-4">
                <div className="flex gap-2">
                  <AlertTriangle className="h-5 w-5 text-yellow-600 flex-shrink-0 mt-0.5" />
                  <div className="space-y-1">
                    <p className="font-semibold text-sm text-yellow-900">Important Notice</p>
                    <ul className="text-xs text-yellow-800 space-y-1 list-disc list-inside">
                      <li>
                        The wallet will be debited by{' '}
                        {formatCurrency(reversalDialog.transaction.amount)}
                      </li>
                      <li>This may result in a negative balance if funds have been spent</li>
                      <li>A new DEBIT transaction will be created</li>
                      <li>The original transaction will be marked as REVERSED</li>
                      <li>This action cannot be undone</li>
                    </ul>
                  </div>
                </div>
              </div>

              {/* Reversal Reason */}
              <div className="space-y-2">
                <Label htmlFor="reversal-reason">
                  Reason for Reversal <span className="text-red-500">*</span>
                </Label>
                <Textarea
                  id="reversal-reason"
                  placeholder="Explain why this transaction needs to be reversed (minimum 20 characters)..."
                  value={reversalReason}
                  onChange={e => setReversalReason(e.target.value)}
                  rows={4}
                  className="resize-none"
                />
                <p className="text-xs text-muted-foreground">
                  {reversalReason.length}/20 characters minimum
                </p>
              </div>

              {/* Confirmation Checkbox */}
              <div className="flex items-start space-x-2">
                <Checkbox
                  id="confirm-reversal"
                  checked={reversalConfirmed}
                  onCheckedChange={(checked: boolean) => setReversalConfirmed(checked)}
                />
                <Label
                  htmlFor="confirm-reversal"
                  className="text-sm font-normal leading-tight cursor-pointer"
                >
                  I understand the consequences and confirm that I want to reverse this transaction
                </Label>
              </div>
            </div>
          )}

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setReversalDialog({ open: false, transaction: null })
                setReversalReason('')
                setReversalConfirmed(false)
              }}
              disabled={reversalMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleReversal}
              disabled={
                !reversalReason ||
                reversalReason.length < 20 ||
                !reversalConfirmed ||
                reversalMutation.isPending
              }
            >
              {reversalMutation.isPending ? (
                <>
                  <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2" />
                  Reversing...
                </>
              ) : (
                <>
                  <Undo2 className="h-4 w-4 mr-2" />
                  Reverse Transaction
                </>
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
