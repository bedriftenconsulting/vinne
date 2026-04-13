import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { Player } from '@/services/players'
import { User, Phone, Mail, CreditCard, Calendar, Clock } from 'lucide-react'
import { formatInGhanaTime } from '@/lib/date-utils'

interface PlayerInfoCardProps {
  player: Player
}

export function PlayerInfoCard({ player }: PlayerInfoCardProps) {
  const getStatusBadgeVariant = (status: string) => {
    switch (status) {
      case 'ACTIVE':
        return 'default'
      case 'SUSPENDED':
        return 'secondary'
      case 'BANNED':
        return 'destructive'
      default:
        return 'secondary'
    }
  }

  const getStatusLabel = (status: string) => {
    switch (status) {
      case 'ACTIVE':
        return 'Active'
      case 'SUSPENDED':
        return 'Suspended'
      case 'BANNED':
        return 'Banned'
      default:
        return status
    }
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <User className="h-5 w-5" />
            Player Information
          </CardTitle>
          <Badge variant={getStatusBadgeVariant(player.status)}>
            {getStatusLabel(player.status)}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-1">
            <p className="text-sm text-muted-foreground">Full Name</p>
            <p className="font-medium">
              {player.first_name && player.last_name
                ? `${player.first_name} ${player.last_name}`
                : player.first_name || player.last_name || 'N/A'}
            </p>
          </div>

          <div className="space-y-1">
            <p className="text-sm text-muted-foreground flex items-center gap-1">
              <Phone className="h-3 w-3" />
              Phone Number
            </p>
            <p className="font-medium">{player.phone_number}</p>
          </div>

          <div className="space-y-1">
            <p className="text-sm text-muted-foreground flex items-center gap-1">
              <Mail className="h-3 w-3" />
              Email
            </p>
            <p className="font-medium">{player.email || 'N/A'}</p>
          </div>

          <div className="space-y-1">
            <p className="text-sm text-muted-foreground flex items-center gap-1">
              <CreditCard className="h-3 w-3" />
              National ID
            </p>
            <p className="font-medium">{player.national_id || 'N/A'}</p>
          </div>

          <div className="space-y-1">
            <p className="text-sm text-muted-foreground flex items-center gap-1">
              <Phone className="h-3 w-3" />
              Mobile Money Phone
            </p>
            <p className="font-medium">{player.mobile_money_phone || 'N/A'}</p>
          </div>

          <div className="space-y-1">
            <p className="text-sm text-muted-foreground flex items-center gap-1">
              <Calendar className="h-3 w-3" />
              Date of Birth
            </p>
            <p className="font-medium">
              {player.date_of_birth ? formatInGhanaTime(player.date_of_birth, 'PP') : 'N/A'}
            </p>
          </div>

          <div className="space-y-1">
            <p className="text-sm text-muted-foreground flex items-center gap-1">
              <Calendar className="h-3 w-3" />
              Registered
            </p>
            <p className="font-medium">{formatInGhanaTime(player.created_at, 'PPp')}</p>
          </div>

          <div className="space-y-1">
            <p className="text-sm text-muted-foreground flex items-center gap-1">
              <Clock className="h-3 w-3" />
              Last Login
            </p>
            <p className="font-medium">
              {player.last_login ? formatInGhanaTime(player.last_login, 'PPp') : 'Never'}
            </p>
          </div>
        </div>

        {player.wallet_id && (
          <div className="pt-4 border-t">
            <p className="text-sm text-muted-foreground">Wallet ID</p>
            <p className="font-mono text-sm">{player.wallet_id}</p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
