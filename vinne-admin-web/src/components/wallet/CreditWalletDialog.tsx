import { useState, useEffect } from 'react'
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
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Calculator, AlertCircle, TrendingUp } from 'lucide-react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import * as z from 'zod'

const creditSchema = z.object({
  amount: z
    .string()
    .regex(/^\d+(\.\d{0,2})?$/, 'Invalid amount format')
    .refine(val => parseFloat(val) > 0, 'Amount must be greater than 0')
    .refine(val => parseFloat(val) <= 1000000, 'Amount cannot exceed 1,000,000 GHS'),
  description: z.string().min(1, 'Description is required').max(500, 'Description too long'),
})

type CreditFormData = z.infer<typeof creditSchema>

interface CreditWalletDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  walletType: 'agent' | 'retailer'
  ownerName: string
  ownerId: string
  currentBalance: number
  commissionRate?: number // In basis points (e.g., 1000 = 10%)
  onConfirm: (amount: number, description: string) => Promise<void>
}

export function CreditWalletDialog({
  open,
  onOpenChange,
  walletType,
  ownerName,
  // ownerId: _ownerId, // Not used in this component
  currentBalance,
  commissionRate = 0,
  onConfirm,
}: CreditWalletDialogProps) {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    watch,
    reset,
    formState: { errors },
  } = useForm<CreditFormData>({
    resolver: zodResolver(creditSchema),
    defaultValues: {
      amount: '',
      description: '',
    },
  })

  const watchAmount = watch('amount')

  // Calculate commission and total
  const calculateAmounts = (inputAmount: string) => {
    const amount = parseFloat(inputAmount) || 0
    const amountInPesewas = Math.round(amount * 100)

    if (walletType === 'agent' && commissionRate > 0) {
      // For agents, we gross up the amount
      const commissionAmount = Math.round((amountInPesewas * commissionRate) / 10000)
      const totalAmount = amountInPesewas + commissionAmount
      return {
        baseAmount: amountInPesewas,
        commission: commissionAmount,
        total: totalAmount,
      }
    }

    return {
      baseAmount: amountInPesewas,
      commission: 0,
      total: amountInPesewas,
    }
  }

  const amounts = calculateAmounts(watchAmount)

  const formatCurrency = (amountInPesewas: number) => {
    return new Intl.NumberFormat('en-GH', {
      style: 'currency',
      currency: 'GHS',
      minimumFractionDigits: 2,
    }).format(amountInPesewas / 100)
  }

  const formatPercent = (basisPoints: number) => {
    return `${(basisPoints / 100).toFixed(2)}%`
  }

  const onSubmit = async (data: CreditFormData) => {
    try {
      setIsLoading(true)
      setError(null)

      // Convert to pesewas for the API
      const amountInPesewas = Math.round(parseFloat(data.amount) * 100)
      await onConfirm(amountInPesewas, data.description)

      reset()
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to credit wallet')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    if (!open) {
      reset()
      setError(null)
    }
  }, [open, reset])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>Credit {walletType === 'agent' ? 'Agent' : 'Retailer'} Wallet</DialogTitle>
          <DialogDescription>
            Add funds to {ownerName}'s {walletType} wallet
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="amount">Amount (GHS)</Label>
              <div className="relative">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">
                  ₵
                </span>
                <Input
                  id="amount"
                  type="text"
                  placeholder="0.00"
                  className="pl-8"
                  {...register('amount')}
                  disabled={isLoading}
                />
              </div>
              {errors.amount && <p className="text-sm text-destructive">{errors.amount.message}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="description">Description</Label>
              <Textarea
                id="description"
                placeholder="Enter transaction description..."
                rows={3}
                {...register('description')}
                disabled={isLoading}
              />
              {errors.description && (
                <p className="text-sm text-destructive">{errors.description.message}</p>
              )}
            </div>

            {watchAmount && parseFloat(watchAmount) > 0 && (
              <Card className="bg-muted/50">
                <CardContent className="pt-4 space-y-3">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <Calculator className="h-4 w-4" />
                    Transaction Summary
                  </div>

                  <div className="space-y-2 text-sm">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Amount</span>
                      <span>{formatCurrency(amounts.baseAmount)}</span>
                    </div>

                    {walletType === 'agent' && commissionRate > 0 && (
                      <>
                        <div className="flex justify-between">
                          <span className="text-muted-foreground">
                            Commission ({formatPercent(commissionRate)})
                          </span>
                          <span className="text-green-600">
                            +{formatCurrency(amounts.commission)}
                          </span>
                        </div>
                        <Separator />
                        <div className="flex justify-between font-medium">
                          <span>Total Credit</span>
                          <span className="text-lg">{formatCurrency(amounts.total)}</span>
                        </div>
                      </>
                    )}

                    <Separator />

                    <div className="flex justify-between text-xs">
                      <span className="text-muted-foreground">Current Balance</span>
                      <span>{formatCurrency(currentBalance)}</span>
                    </div>

                    <div className="flex justify-between text-xs">
                      <span className="text-muted-foreground">New Balance</span>
                      <span className="font-medium text-green-600">
                        {formatCurrency(currentBalance + amounts.total)}
                      </span>
                    </div>
                  </div>
                </CardContent>
              </Card>
            )}

            {error && (
              <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            <Alert>
              <TrendingUp className="h-4 w-4" />
              <AlertDescription>
                {walletType === 'agent' && commissionRate > 0
                  ? 'Commission will be automatically calculated and added to the credit amount.'
                  : 'This transaction will be recorded in the transaction history.'}
              </AlertDescription>
            </Alert>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isLoading}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isLoading || !watchAmount}>
              {isLoading ? 'Processing...' : 'Credit Wallet'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
