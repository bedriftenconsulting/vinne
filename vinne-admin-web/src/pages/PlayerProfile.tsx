import { useQuery } from '@tanstack/react-query'
import { useParams, Link } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { playerService } from '@/services/players'
import { PlayerInfoCard } from '@/components/players/PlayerInfoCard'
import { PlayerStatusCard } from '@/components/players/PlayerStatusCard'
import { PlayerWalletCard } from '@/components/players/PlayerWalletCard'
import { PlayerTicketsCard } from '@/components/players/PlayerTicketsCard'
import { ArrowLeft } from 'lucide-react'

export default function PlayerProfile() {
  const { playerId } = useParams({ from: '/admin/player/$playerId' })

  const {
    data: player,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['player', playerId],
    queryFn: () => playerService.getPlayer(playerId),
  })

  if (isLoading) {
    return (
      <div className="p-3 sm:p-4 md:p-6">
        <div className="animate-pulse space-y-6">
          <div className="h-8 bg-muted rounded w-1/4" />
          <div className="grid gap-6 md:grid-cols-2">
            <div className="h-96 bg-muted rounded" />
            <div className="space-y-6">
              <div className="h-48 bg-muted rounded" />
              <div className="h-48 bg-muted rounded" />
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (error || !player) {
    return (
      <div className="p-3 sm:p-4 md:p-6">
        <div className="flex flex-col items-center justify-center py-12">
          <p className="text-lg text-muted-foreground mb-4">Failed to load player details.</p>
          <Link to="/admin/players">
            <Button variant="outline">
              <ArrowLeft className="h-4 w-4 mr-2" />
              Back to Players
            </Button>
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className="p-3 sm:p-4 md:p-6 space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <Link to="/admin/players">
              <Button variant="ghost" size="sm">
                <ArrowLeft className="h-4 w-4" />
              </Button>
            </Link>
            <h1 className="text-2xl sm:text-3xl font-bold">Player Profile</h1>
          </div>
          <p className="text-sm text-muted-foreground">View and manage player information</p>
        </div>
      </div>

      {/* Content Grid */}
      <div className="grid gap-6 md:grid-cols-2">
        {/* Left Column - Player Info */}
        <div>
          <PlayerInfoCard player={player} />
        </div>

        {/* Right Column - Status and Wallet */}
        <div className="space-y-6">
          <PlayerStatusCard player={player} />
          <PlayerWalletCard playerId={player.id} walletId={player.wallet_id} />
        </div>
      </div>

      {/* Tickets Section - Full Width */}
      <div className="mt-6">
        <PlayerTicketsCard playerId={player.id} />
      </div>
    </div>
  )
}
