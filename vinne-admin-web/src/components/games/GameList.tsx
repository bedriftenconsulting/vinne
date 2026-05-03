import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { GameBrandingDialog } from '@/components/games/GameBrandingDialog'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Plus,
  Edit,
  Play,
  Pause,
  Search,
  Calendar,
  Palette,
  Eye,
  CheckCircle,
  XCircle,
  RefreshCw,
  Star,
  Trash2,
  PlayCircle,
} from 'lucide-react'
import { gameService, type Game } from '@/services/games'
import { formatInGhanaTime } from '@/lib/date-utils'
import { useToast } from '@/hooks/use-toast'

interface GameListProps {
  onCreateGame: () => void
  onEditGame: (game: Game) => void
  onViewDetails: (game: Game) => void
  onManageSchedule: (game: Game) => void
  onManagePrizes: (game: Game) => void
  onViewApprovals?: () => void
}

export function GameList({
  onCreateGame,
  onEditGame,
  onViewDetails,
  onManageSchedule,
  onManagePrizes,
}: GameListProps) {
  const [searchTerm, setSearchTerm] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [typeFilter, setTypeFilter] = useState<string>('all')
  const [brandingGame, setBrandingGame] = useState<Game | null>(null)
  const [defaultGameId, setDefaultGameId] = useState<string>(
    () => localStorage.getItem('winbig_default_game_id') || ''
  )
  const [confirmDialog, setConfirmDialog] = useState<{
    open: boolean
    title: string
    description: string
    action: () => void
    destructive?: boolean
  }>({ open: false, title: '', description: '', action: () => {} })

  const queryClient = useQueryClient()

  const { data: gamesData, isLoading, refetch } = useQuery({
    queryKey: ['games-list'],
    queryFn: () => gameService.getGames(1, 500),
    staleTime: 0,
    refetchOnMount: true,
    refetchOnWindowFocus: true,
  })

  const games = (gamesData?.data as Game[]) || []

  // Filter games based on search and filters
  const filteredGames = games.filter((game: Game) => {
    const matchesSearch =
      game.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      game.code.toLowerCase().includes(searchTerm.toLowerCase())

    const matchesStatus =
      statusFilter === 'all' || game.status.toLowerCase() === statusFilter.toLowerCase()

    const gameType = game.game_format || game.game_category || 'unknown'
    const matchesType = typeFilter === 'all' || gameType === typeFilter

    return matchesSearch && matchesStatus && matchesType
  })

  const getStatusBadge = (status: string) => {
    const statusMap: {
      [key: string]: { variant: 'default' | 'secondary' | 'outline' | 'destructive'; label: string }
    } = {
      active: { variant: 'default', label: 'Active' },
      draft: { variant: 'secondary', label: 'Draft' },
      submitted: { variant: 'outline', label: 'Submitted' },
      first_approved: { variant: 'outline', label: 'First Approved' },
      approved: { variant: 'default', label: 'Approved' },
      pending_approval: { variant: 'outline', label: 'Pending Approval' },
      suspended: { variant: 'destructive', label: 'Suspended' },
      archived: { variant: 'secondary', label: 'Archived' },
    }

    const normalizedStatus = status
      .toLowerCase()
      .replace(/([A-Z])/g, '_$1')
      .toLowerCase()
    const config = statusMap[normalizedStatus] || { variant: 'secondary', label: status }

    return <Badge variant={config.variant}>{config.label}</Badge>
  }

  const getGameTypeLabel = (type: string) => {
    const typeMap: { [key: string]: string } = {
      national: 'National',
      private: 'Competition',
      special: 'Special',
      competition: 'Competition',
      raffle: 'Raffle',
      '5_by_90': '5/90',
      direct: 'Direct',
      perm: 'Perm',
      banker: 'Banker',
    }
    return typeMap[type?.toLowerCase()] || 'Competition'
  }

  // Mutations for approval workflow
  const activateGameMutation = useMutation({
    mutationFn: (gameId: string) => gameService.activateGame(gameId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      queryClient.invalidateQueries({ queryKey: ['games-list'] })
      toast({ title: 'Success', description: 'Game activated successfully' })
    },
    onError: () => {
      toast({ title: 'Error', description: 'Failed to activate game', variant: 'destructive' })
    },
  })

  const suspendGameMutation = useMutation({
    mutationFn: ({ gameId, reason }: { gameId: string; reason: string }) =>
      gameService.suspendGame(gameId, reason),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      queryClient.invalidateQueries({ queryKey: ['games-list'] })
      toast({ title: 'Success', description: 'Game suspended' })
    },
    onError: () => {
      toast({ title: 'Error', description: 'Failed to suspend game', variant: 'destructive' })
    },
  })

  const approveGameMutation = useMutation({
    mutationFn: ({ gameId, notes }: { gameId: string; notes?: string }) =>
      gameService.approveGame(gameId, notes),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      queryClient.invalidateQueries({ queryKey: ['games-list'] })
      toast({
        title: 'Success',
        description: 'Game approved successfully',
      })
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to approve game',
        variant: 'destructive',
      })
    },
  })

  const rejectGameMutation = useMutation({
    mutationFn: ({ gameId, reason }: { gameId: string; reason: string }) =>
      gameService.rejectGame(gameId, reason),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      queryClient.invalidateQueries({ queryKey: ['games-list'] })
      toast({
        title: 'Success',
        description: 'Game rejected',
      })
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to reject game',
        variant: 'destructive',
      })
    },
  })

  const deleteGameMutation = useMutation({
    mutationFn: (gameId: string) => gameService.deleteGame(gameId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      queryClient.invalidateQueries({ queryKey: ['games-list'] })
      toast({ title: 'Game deleted' })
    },
    onError: () => {
      toast({ title: 'Error', description: 'Failed to delete game', variant: 'destructive' })
    },
  })

  const { toast } = useToast()

  const handleApproveGame = (game: Game) => {
    setConfirmDialog({
      open: true,
      title: 'Approve Game',
      description: `Are you sure you want to approve "${game.name}"?`,
      action: () => {
        approveGameMutation.mutate({ gameId: game.id })
        setConfirmDialog({ ...confirmDialog, open: false })
      },
    })
  }

  const handleRejectGame = (game: Game) => {
    const reason = prompt('Please provide a reason for rejection:')
    if (reason) {
      rejectGameMutation.mutate({ gameId: game.id, reason })
    }
  }

  const handleActivateGame = (game: Game) => {
    setConfirmDialog({
      open: true,
      title: 'Activate Game',
      description: `Are you sure you want to activate "${game.name}"? It will be live and available to players.`,
      action: () => {
        activateGameMutation.mutate(game.id)
        setConfirmDialog(prev => ({ ...prev, open: false }))
      },
    })
  }

  const handleSuspendGame = (game: Game) => {
    const reason = prompt('Please provide a reason for suspending this game:')
    if (reason) {
      suspendGameMutation.mutate({ gameId: game.id, reason })
    }
  }

  const handleResumeGame = (game: Game) => {
    setConfirmDialog({
      open: true,
      title: 'Resume Game',
      description: `Resume "${game.name}"? It will go back to Active and be visible to players.`,
      action: () => {
        activateGameMutation.mutate(game.id)
        setConfirmDialog(prev => ({ ...prev, open: false }))
      },
    })
  }

  const handleDeleteGame = (game: Game) => {
    setConfirmDialog({
      open: true,
      title: `Delete "${game.name}"?`,
      description: `This is permanent and cannot be undone. All schedules and data for this game will be removed.`,
      destructive: true,
      action: () => {
        deleteGameMutation.mutate(game.id)
        setConfirmDialog(prev => ({ ...prev, open: false }))
      },
    })
  }

  const handleSetDefault = async (game: Game) => {
    try {
      await fetch(`${import.meta.env.VITE_API_URL?.replace('/api/v1', '')}/api/v1/config`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer winbig-admin-config-2026',
        },
        body: JSON.stringify({ featured_game_id: game.id, featured_game_code: game.code }),
      })
    } catch { /* non-fatal, still update local state */ }
    localStorage.setItem('winbig_default_game_id', game.id)
    setDefaultGameId(game.id)
    toast({ title: `"${game.name}" set as default hero game ⭐` })
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Game Management</CardTitle>
          <div className="flex gap-2">
            <Button variant="outline" size="icon" onClick={() => refetch()} title="Refresh">
              <RefreshCw className="h-4 w-4" />
            </Button>
            <Button onClick={onCreateGame} className="gap-2">
              <Plus className="h-4 w-4" />
              Create New Game
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="mb-6 flex gap-4">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="Search games by name or code..."
              value={searchTerm}
              onChange={e => setSearchTerm(e.target.value)}
              className="pl-10"
            />
          </div>
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="All Statuses" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Statuses</SelectItem>
              <SelectItem value="active">Active</SelectItem>
              <SelectItem value="draft">Draft</SelectItem>
              <SelectItem value="pending_approval">Pending Approval</SelectItem>
              <SelectItem value="suspended">Suspended</SelectItem>
              <SelectItem value="archived">Archived</SelectItem>
            </SelectContent>
          </Select>
          <Select value={typeFilter} onValueChange={setTypeFilter}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="All Types" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Types</SelectItem>
              <SelectItem value="competition">Competition</SelectItem>
              <SelectItem value="private">Private</SelectItem>
              <SelectItem value="national">National</SelectItem>
              <SelectItem value="special">Special</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-8">
            <div className="text-muted-foreground">Loading games...</div>
          </div>
        ) : filteredGames.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-8">
            <div className="text-muted-foreground">No games found</div>
            {searchTerm || statusFilter !== 'all' || typeFilter !== 'all' ? (
              <Button
                variant="link"
                onClick={() => {
                  setSearchTerm('')
                  setStatusFilter('all')
                  setTypeFilter('all')
                }}
                className="mt-2"
              >
                Clear filters
              </Button>
            ) : null}
          </div>
        ) : (
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Game Code</TableHead>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Ticket Price</TableHead>
                  <TableHead>Draw Frequency</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredGames.map((game: Game) => (
                  <TableRow key={game.id}>
                    <TableCell className="font-medium">{game.code}</TableCell>
                    <TableCell>{game.name}{defaultGameId === game.id && <span className="ml-2 text-xs bg-yellow-100 text-yellow-700 border border-yellow-300 px-1.5 py-0.5 rounded-full font-semibold">Default</span>}</TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {getGameTypeLabel(game.game_format || game.game_category || 'competition')}
                      </Badge>
                    </TableCell>
                    <TableCell>₵{(game.base_price || game.ticket_price || 0).toFixed(2)}</TableCell>
                    <TableCell className="capitalize">
                      {game.draw_frequency.replace('_', ' ')}
                    </TableCell>
                    <TableCell>{getStatusBadge(game.status)}</TableCell>
                    <TableCell>
                      {formatInGhanaTime(game.created_at, 'MMM dd, yyyy')}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-2">
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => handleSetDefault(game)}
                          title="Set as default hero game"
                          className={defaultGameId === game.id ? 'text-yellow-500' : 'text-muted-foreground hover:text-yellow-500'}
                        >
                          <Star className={`h-4 w-4 ${defaultGameId === game.id ? 'fill-yellow-400' : ''}`} />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => onViewDetails(game)}
                          title="View Details"
                        >
                          <Eye className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => onEditGame(game)}
                          title="Edit Game"
                        >
                          <Edit className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => setBrandingGame(game)}
                          title="Game Branding"
                          className="text-purple-600 hover:text-purple-700"
                        >
                          <Palette className="h-4 w-4" />
                        </Button>

                        {/* Approval workflow actions based on status */}
                        {(game.status.toUpperCase() === 'SUBMITTED' ||
                          game.status.toUpperCase() === 'FIRST_APPROVED') && (
                          <>
                            <Button
                              variant="ghost"
                              size="icon"
                              onClick={() => handleApproveGame(game)}
                              title="Approve Game"
                              className="text-green-600 hover:text-green-700"
                            >
                              <CheckCircle className="h-4 w-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              onClick={() => handleRejectGame(game)}
                              title="Reject Game"
                              className="text-red-600 hover:text-red-700"
                            >
                              <XCircle className="h-4 w-4" />
                            </Button>
                          </>
                        )}
                        {(game.status.toUpperCase() === 'DRAFT' || game.status.toUpperCase() === 'APPROVED') && (
                          <Button
                            variant="ghost"
                            size="icon"
                            className="text-green-600 hover:text-green-700"
                            title="Activate Game"
                            onClick={() => handleActivateGame(game)}
                          >
                            <Play className="h-4 w-4" />
                          </Button>
                        )}
                        {game.status.toUpperCase() === 'ACTIVE' && (
                          <Button
                            variant="ghost"
                            size="icon"
                            className="text-orange-600 hover:text-orange-700"
                            title="Suspend Game"
                            onClick={() => handleSuspendGame(game)}
                          >
                            <Pause className="h-4 w-4" />
                          </Button>
                        )}
                        {game.status.toUpperCase() === 'SUSPENDED' && (
                          <Button
                            variant="ghost"
                            size="icon"
                            className="text-green-600 hover:text-green-700"
                            title="Resume Game"
                            onClick={() => handleResumeGame(game)}
                          >
                            <PlayCircle className="h-4 w-4" />
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="icon"
                          className="text-red-500 hover:text-red-600"
                          title="Delete Game"
                          onClick={() => handleDeleteGame(game)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      {/* Confirmation Dialog */}
      <AlertDialog
        open={confirmDialog.open}
        onOpenChange={(open: boolean) => setConfirmDialog({ ...confirmDialog, open })}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirmDialog.title}</AlertDialogTitle>
            <AlertDialogDescription>{confirmDialog.description}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmDialog.action}
              className={confirmDialog.destructive ? 'bg-red-600 hover:bg-red-700 text-white' : ''}
            >
              {confirmDialog.destructive ? 'Delete' : 'Confirm'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <GameBrandingDialog
        isOpen={brandingGame !== null}
        onClose={() => setBrandingGame(null)}
        game={brandingGame}
      />
    </Card>
  )
}
