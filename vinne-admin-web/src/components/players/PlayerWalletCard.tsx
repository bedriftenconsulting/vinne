import { useQuery } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { playerService } from '@/services/players'
import { Wallet, TrendingUp, Clock } from 'lucide-react'

interface PlayerWalletCardProps {
  playerId: string
  walletId?: string
}

export function PlayerWalletCard({ playerId, walletId }: PlayerWalletCardProps) {
  const {
    data: wallet,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['player-wallet', playerId],
    queryFn: () => playerService.getPlayerWallet(playerId),
    enabled: true, // Always fetch - wallet lookup is by player_id
    retry: 1, // Only retry once if it fails
  })

  const formatCurrency = (amount: number, currency: string = 'GHS') => {
    return new Intl.NumberFormat('en-GH', {
      style: 'currency',
      currency: currency,
      minimumFractionDigits: 2,
    }).format(amount)
  }

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Wallet className="h-5 w-5" />
            Wallet Information
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8">
            <div className="animate-pulse space-y-3">
              <div className="h-4 bg-muted rounded w-1/2 mx-auto" />
              <div className="h-8 bg-muted rounded w-3/4 mx-auto" />
              <div className="h-4 bg-muted rounded w-2/3 mx-auto" />
            </div>
          </div>
        </CardContent>
      </Card>
    )
  }

  if (error || !wallet) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Wallet className="h-5 w-5" />
            Wallet Information
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-muted-foreground">
            <Wallet className="h-12 w-12 mx-auto mb-4 text-muted-foreground/50" />
            <p>No wallet found for this player.</p>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Wallet className="h-5 w-5" />
          Wallet Information
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Main Balance */}
        <div className="space-y-2">
          <p className="text-sm text-muted-foreground">Available Balance</p>
          <p className="text-3xl font-bold text-green-600">
            {formatCurrency(wallet.balance, wallet.currency)}
          </p>
        </div>

        {/* Pending Balance */}
        {wallet.pending_balance > 0 && (
          <div className="space-y-2 pt-4 border-t">
            <div className="flex items-center gap-2">
              <Clock className="h-4 w-4 text-yellow-600" />
              <p className="text-sm text-muted-foreground">Pending Balance</p>
            </div>
            <p className="text-2xl font-bold text-yellow-600">
              {formatCurrency(wallet.pending_balance, wallet.currency)}
            </p>
            <p className="text-xs text-muted-foreground">
              Funds awaiting confirmation or processing
            </p>
          </div>
        )}

        {/* Total Wallet Value */}
        <div className="space-y-2 pt-4 border-t">
          <div className="flex items-center gap-2">
            <TrendingUp className="h-4 w-4 text-blue-600" />
            <p className="text-sm text-muted-foreground">Total Wallet Value</p>
          </div>
          <p className="text-xl font-bold text-blue-600">
            {formatCurrency(wallet.balance + wallet.pending_balance, wallet.currency)}
          </p>
        </div>

        {/* Wallet ID */}
        <div className="pt-4 border-t">
          <p className="text-sm text-muted-foreground mb-1">Wallet ID</p>
          <p className="font-mono text-xs text-muted-foreground break-all">{walletId}</p>
        </div>
      </CardContent>
    </Card>
  )
}
