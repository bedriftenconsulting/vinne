import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Wallet, TrendingUp, Clock, CreditCard } from 'lucide-react'

interface AgentWalletCardProps {
  agentId: string
  agentName: string
  balance: number
  pendingBalance?: number
  lastTransaction?: {
    amount: number
    type: 'credit' | 'debit' | 'transfer'
    date: string
  }
  onCreditClick?: () => void
  onViewTransactions: () => void
}

export function AgentWalletCard({
  agentId,
  agentName,
  balance,
  pendingBalance = 0,
  lastTransaction,
  onCreditClick,
  onViewTransactions,
}: AgentWalletCardProps) {
  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('en-GH', {
      style: 'currency',
      currency: 'GHS',
      minimumFractionDigits: 2,
    }).format(amount / 100) // All amounts are in pesewas
  }

  const getTransactionIcon = (type: string) => {
    switch (type) {
      case 'credit':
        return <TrendingUp className="h-4 w-4 text-green-500" />
      case 'debit':
        return <TrendingUp className="h-4 w-4 text-red-500 rotate-180" />
      default:
        return <CreditCard className="h-4 w-4 text-blue-500" />
    }
  }

  return (
    <Card className="w-full">
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <CardTitle className="flex items-center gap-2">
              <Wallet className="h-5 w-5" />
              Agent Stake Wallet
            </CardTitle>
            <CardDescription>{agentName}</CardDescription>
          </div>
          <Badge variant="outline" className="font-mono">
            {agentId.slice(0, 8).toUpperCase()}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">Available Balance</span>
            <span className="text-2xl font-bold">{formatCurrency(balance)}</span>
          </div>
          {pendingBalance > 0 && (
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground flex items-center gap-1">
                <Clock className="h-3 w-3" />
                Pending
              </span>
              <span className="text-sm text-muted-foreground">
                {formatCurrency(pendingBalance)}
              </span>
            </div>
          )}
        </div>

        {lastTransaction && (
          <div className="pt-3 border-t">
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">Last Transaction</span>
              <div className="flex items-center gap-2">
                {getTransactionIcon(lastTransaction.type)}
                <span
                  className={
                    lastTransaction.type === 'credit'
                      ? 'text-green-600'
                      : lastTransaction.type === 'debit'
                        ? 'text-red-600'
                        : 'text-blue-600'
                  }
                >
                  {lastTransaction.type === 'debit' ? '-' : '+'}
                  {formatCurrency(lastTransaction.amount)}
                </span>
              </div>
            </div>
            <div className="text-xs text-muted-foreground mt-1">
              {lastTransaction.date ? new Date(lastTransaction.date).toLocaleString() : 'Unknown'}
            </div>
          </div>
        )}

        <div className="flex gap-2 pt-2">
          {onCreditClick && (
            <Button onClick={onCreditClick} className="flex-1">
              Credit Wallet
            </Button>
          )}
          <Button
            onClick={onViewTransactions}
            variant="outline"
            className={onCreditClick ? 'flex-1' : 'w-full'}
          >
            View History
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
