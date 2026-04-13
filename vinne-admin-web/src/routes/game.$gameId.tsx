import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ArrowLeft, Calendar, Clock, DollarSign, Trophy, Users } from 'lucide-react'
import { gameService } from '@/services/games'
import { formatCurrency } from '@/lib/utils'

export const Route = createFileRoute('/game/$gameId')({
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  beforeLoad: ({ context }: any) => {
    // Check if user is authenticated
    if (!context.auth?.isAuthenticated) {
      throw redirect({
        to: '/login',
        search: {
          redirect: `/game/$gameId`,
        },
      })
    }
  },
  component: GameDetails,
})

function GameDetails() {
  const { gameId } = Route.useParams()
  const navigate = useNavigate()

  const {
    data: game,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['game', gameId],
    queryFn: () => gameService.getGame(gameId),
  })

  const formatDrawFrequency = (frequency: string) => {
    const frequencyMap: Record<string, string> = {
      daily: 'Daily',
      weekly: 'Weekly',
      'bi-weekly': 'Bi-Weekly',
      monthly: 'Monthly',
    }
    return frequencyMap[frequency.toLowerCase()] || frequency
  }

  const getStatusBadge = (status: string | undefined) => {
    if (!status) {
      return <Badge className="bg-gray-100 text-gray-800">Unknown</Badge>
    }
    const statusMap: Record<string, { label: string; className: string }> = {
      active: { label: 'Active', className: 'bg-green-100 text-green-800' },
      draft: { label: 'Draft', className: 'bg-gray-100 text-gray-800' },
      suspended: { label: 'Suspended', className: 'bg-red-100 text-red-800' },
      pending: { label: 'Pending Approval', className: 'bg-yellow-100 text-yellow-800' },
    }
    const statusInfo = statusMap[status.toLowerCase()] || { label: status, className: '' }
    return <Badge className={statusInfo.className}>{statusInfo.label}</Badge>
  }

  if (isLoading) {
    return (
      <AdminLayout>
        <div className="p-6 space-y-6">
          <div className="flex justify-center items-center h-64">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
          </div>
        </div>
      </AdminLayout>
    )
  }

  if (error || !game) {
    return (
      <AdminLayout>
        <div className="p-6">
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Trophy className="h-12 w-12 text-gray-400 mb-4" />
              <h3 className="text-lg font-semibold text-gray-900">Game Not Found</h3>
              <p className="text-sm text-gray-500 mb-4">
                The game you're looking for doesn't exist or you don't have access to it.
              </p>
              <Button onClick={() => navigate({ to: '/games' })}>
                <ArrowLeft className="h-4 w-4 mr-2" />
                Back to Games
              </Button>
            </CardContent>
          </Card>
        </div>
      </AdminLayout>
    )
  }

  return (
    <AdminLayout>
      <div className="p-6 space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            {game.logo_url && (
              <img
                src={game.logo_url}
                alt={`${game.name} logo`}
                className="h-20 w-20 object-contain rounded-lg border-2 border-gray-200"
              />
            )}
            <div>
              <div className="flex items-center gap-2 mb-2">
                <Button variant="ghost" size="sm" onClick={() => navigate({ to: '/games' })}>
                  <ArrowLeft className="h-4 w-4 mr-2" />
                  Back
                </Button>
              </div>
              <h1 className="text-3xl font-bold tracking-tight">{game.name}</h1>
              <div className="flex items-center gap-2">
                <p className="text-muted-foreground">
                  {game.code} • {game.game_category}
                </p>
                {game.brand_color && (
                  <div
                    className="h-4 w-4 rounded border border-gray-300"
                    style={{ backgroundColor: game.brand_color }}
                    title={`Brand color: ${game.brand_color}`}
                  />
                )}
              </div>
            </div>
          </div>
          <div>{getStatusBadge(game.status)}</div>
        </div>

        {/* Game Info Cards */}
        <div className="grid gap-6 md:grid-cols-2">
          {/* Basic Information */}
          <Card>
            <CardHeader>
              <CardTitle>Basic Information</CardTitle>
              <CardDescription>Core game configuration details</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Game Code</p>
                  <p className="text-lg font-semibold">{game.code}</p>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Category</p>
                  <p className="text-lg font-semibold capitalize">{game.game_category}</p>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Format</p>
                  <p className="text-lg font-semibold">
                    {game.game_format?.replace(/_/g, '/').toUpperCase()}
                  </p>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Status</p>
                  <p className="text-lg font-semibold capitalize">{game.status}</p>
                </div>
              </div>
              {game.description && (
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Description</p>
                  <p className="text-sm mt-1">{game.description}</p>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Draw Configuration */}
          <Card>
            <CardHeader>
              <CardTitle>Draw Configuration</CardTitle>
              <CardDescription>Draw schedule and timing settings</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-start gap-3">
                <Calendar className="h-5 w-5 text-muted-foreground mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm font-medium">Draw Frequency</p>
                  <p className="text-lg font-semibold">
                    {formatDrawFrequency(game.draw_frequency || '')}
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <Clock className="h-5 w-5 text-muted-foreground mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm font-medium">Draw Time</p>
                  <p className="text-lg font-semibold">{game.draw_time || 'Not set'}</p>
                </div>
              </div>
              {game.draw_days && game.draw_days.length > 0 && (
                <div className="flex items-start gap-3">
                  <Users className="h-5 w-5 text-muted-foreground mt-0.5" />
                  <div className="flex-1">
                    <p className="text-sm font-medium">Draw Days</p>
                    <div className="flex gap-2 mt-1 flex-wrap">
                      {game.draw_days.map(day => (
                        <Badge key={day} variant="secondary">
                          {day}
                        </Badge>
                      ))}
                    </div>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Pricing Configuration */}
          <Card>
            <CardHeader>
              <CardTitle>Pricing</CardTitle>
              <CardDescription>Ticket pricing and bet configuration</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {(game.min_stake || game.base_price) && (
                <div className="flex items-start gap-3">
                  <DollarSign className="h-5 w-5 text-muted-foreground mt-0.5" />
                  <div className="flex-1">
                    <p className="text-sm font-medium">Base Price / Min Stake</p>
                    <p className="text-lg font-semibold">
                      {formatCurrency(game.base_price || game.min_stake || 0)}
                    </p>
                  </div>
                </div>
              )}
              {game.max_stake && (
                <div className="flex items-start gap-3">
                  <DollarSign className="h-5 w-5 text-muted-foreground mt-0.5" />
                  <div className="flex-1">
                    <p className="text-sm font-medium">Maximum Stake</p>
                    <p className="text-lg font-semibold">{formatCurrency(game.max_stake)}</p>
                  </div>
                </div>
              )}
              {game.max_tickets_per_player && (
                <div className="flex items-start gap-3">
                  <Trophy className="h-5 w-5 text-muted-foreground mt-0.5" />
                  <div className="flex-1">
                    <p className="text-sm font-medium">Max Tickets Per Player</p>
                    <p className="text-lg font-semibold">{game.max_tickets_per_player}</p>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Additional Settings */}
          <Card>
            <CardHeader>
              <CardTitle>Additional Settings</CardTitle>
              <CardDescription>Other game configuration options</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Number Range</p>
                  <p className="text-lg font-semibold">
                    {game.number_range_min && game.number_range_max
                      ? `${game.number_range_min}-${game.number_range_max}`
                      : 'Not set'}
                  </p>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Numbers to Select</p>
                  <p className="text-lg font-semibold">{game.selection_count || 'Not set'}</p>
                </div>
                {game.multi_draw_enabled !== undefined && (
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">Multi-Draw</p>
                    <p className="text-lg font-semibold">
                      {game.multi_draw_enabled ? 'Enabled' : 'Disabled'}
                    </p>
                  </div>
                )}
                {game.max_draws_advance && (
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">Max Draws Advance</p>
                    <p className="text-lg font-semibold">{game.max_draws_advance}</p>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </AdminLayout>
  )
}
