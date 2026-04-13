import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Trophy, CreditCard } from 'lucide-react'

interface RetailerWalletCardProps {
  retailerId: string
  retailerName: string
  walletType: 'stake' | 'winning'
  balance?: number
  pendingBalance?: number
  onViewTransactions: () => void
}

export function RetailerWalletCard({
  retailerId,
  retailerName,
  walletType,
  balance = 0,
  pendingBalance = 0,
  onViewTransactions,
}: RetailerWalletCardProps) {
  const formatCurrency = (amount: number) => {
    // Handle invalid values
    if (typeof amount !== 'number' || isNaN(amount)) {
      return 'GH₵0.00'
    }
    return new Intl.NumberFormat('en-GH', {
      style: 'currency',
      currency: 'GHS',
      minimumFractionDigits: 2,
    }).format(amount / 100) // Convert from pesewas to GHS
  }

  const walletIcon =
    walletType === 'stake' ? <CreditCard className="h-5 w-5" /> : <Trophy className="h-5 w-5" />
  const walletTitle = walletType === 'stake' ? 'Stake Wallet' : 'Winnings Wallet'

  return (
    <Card className="w-full">
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <CardTitle className="flex items-center gap-2">
              {walletIcon}
              {walletTitle}
            </CardTitle>
            <CardDescription>{retailerName}</CardDescription>
          </div>
          <Badge variant="outline" className="font-mono">
            {retailerId.slice(0, 8).toUpperCase()}
          </Badge>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Balance</span>
              <span
                className={`text-2xl font-bold ${walletType === 'winning' ? 'text-green-600' : ''}`}
              >
                {formatCurrency(balance)}
              </span>
            </div>
            {pendingBalance > 0 && (
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Pending</span>
                <span className="text-lg text-yellow-600">{formatCurrency(pendingBalance)}</span>
              </div>
            )}
          </div>

          <div className="flex gap-2 pt-2">
            <Button onClick={onViewTransactions} variant="outline" className="w-full">
              View Transaction History
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
