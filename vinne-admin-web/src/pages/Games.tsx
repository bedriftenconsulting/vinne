import { useState, useEffect } from 'react'
import { getGMTWeekStart } from '@/lib/date-utils'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { CreateGameWizard } from '@/components/games/CreateGameWizard'
import { EditGameWizard } from '@/components/games/EditGameWizard'
import { GameBrandingDialog } from '@/components/games/GameBrandingDialog'
import { ScheduledGamesView } from '@/components/games/ScheduledGamesView'
import { EditScheduleDialog } from '@/components/games/EditScheduleDialog'
import { GenerateScheduleDialog } from '@/components/games/GenerateScheduleDialog'
import { useToast } from '@/hooks/use-toast'
import {
  Plus,
  Edit,
  Play,
  Pause,
  Trash2,
  CheckCircle,
  XCircle,
  AlertCircle,
  Loader2,
  Search,
  Gamepad2,
  Calendar,
  List,
  Palette,
} from 'lucide-react'
import { gameService, type Game, type CreateGameRequest, type GameSchedule } from '@/services/games'

export default function Games() {
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const [searchTerm, setSearchTerm] = useState(
    () => localStorage.getItem('games-search-term') || ''
  )
  const [selectedGame, setSelectedGame] = useState<Game | null>(null)
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [activeTab, setActiveTab] = useState(
    () => localStorage.getItem('games-filter-tab') || 'all'
  )
  const [mainTab, setMainTab] = useState(
    () => localStorage.getItem('games-main-tab') || 'schedules'
  )
  const [isActivateDialogOpen, setIsActivateDialogOpen] = useState(false)
  const [gameToActivate, setGameToActivate] = useState<Game | null>(null)
  const [isEditScheduleDialogOpen, setIsEditScheduleDialogOpen] = useState(false)
  const [selectedSchedule, setSelectedSchedule] = useState<GameSchedule | null>(null)
  const [isGenerateScheduleDialogOpen, setIsGenerateScheduleDialogOpen] = useState(false)
  const [weekForGeneration, setWeekForGeneration] = useState<Date>(new Date())
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [gameToDelete, setGameToDelete] = useState<Game | null>(null)
  const [isBrandingDialogOpen, setIsBrandingDialogOpen] = useState(false)
  const [gameForBranding, setGameForBranding] = useState<Game | null>(null)

  // Persist state to localStorage
  useEffect(() => {
    localStorage.setItem('games-search-term', searchTerm)
  }, [searchTerm])

  useEffect(() => {
    localStorage.setItem('games-filter-tab', activeTab)
  }, [activeTab])

  useEffect(() => {
    localStorage.setItem('games-main-tab', mainTab)
  }, [mainTab])

  // Form state for create/edit
  const [formData, setFormData] = useState<CreateGameRequest>({
    code: '',
    name: '',
    organizer: 'rand_lottery',
    game_category: 'private',
    format: '5_by_90',
    game_format: '5_by_90',
    game_type: '5_by_90', // Keep for backward compatibility
    bet_types: [],
    number_range_min: 1,
    number_range_max: 90,
    selection_count: 5,
    draw_frequency: 'daily',
    draw_days: [],
    draw_time: '',
    sales_cutoff_minutes: 30,
    min_stake: 0.5,
    max_stake: 200,
    base_price: 1,
    max_tickets_per_player: 10,
    max_tickets_per_transaction: 10,
    multi_draw_enabled: false,
    max_draws_advance: 5,
    weekly_schedule: false,
  })

  // Fetch games
  const {
    data: gamesData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['games'],
    queryFn: () => gameService.getGames(1, 100),
  })

  // Fetch scheduled games for current week in GMT
  const { data: scheduledGamesData } = useQuery({
    queryKey: ['scheduledGames'],
    queryFn: async () => {
      // Get Monday of current week in GMT timezone
      const weekStartStr = getGMTWeekStart()
      try {
        return await gameService.getWeeklySchedule(weekStartStr)
      } catch {
        return []
      }
    },
  })

  // Create and update mutations are handled by the respective wizard components

  // Activate game mutation
  const activateGameMutation = useMutation({
    mutationFn: gameService.activateGame,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      toast({
        title: 'Success',
        description: 'Game activated successfully',
      })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error',
        description:
          (error as Error & { response?: { data?: { error?: { message?: string } } } })?.response
            ?.data?.error?.message ||
          (error as Error)?.message ||
          'Failed to activate game',
        variant: 'destructive',
      })
    },
  })

  // Suspend game mutation
  const suspendGameMutation = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      gameService.suspendGame(id, reason),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      toast({
        title: 'Success',
        description: 'Game suspended successfully',
      })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error',
        description:
          (error as Error & { response?: { data?: { error?: { message?: string } } } })?.response
            ?.data?.error?.message ||
          (error as Error)?.message ||
          'Failed to suspend game',
        variant: 'destructive',
      })
    },
  })

  // Delete game mutation
  const deleteGameMutation = useMutation({
    mutationFn: (id: string) => gameService.deleteGame(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      setIsDeleteDialogOpen(false)
      setGameToDelete(null)
      toast({
        title: 'Success',
        description: 'Game deleted successfully',
      })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error',
        description:
          (error as Error & { response?: { data?: { error?: { message?: string } } } })?.response
            ?.data?.error?.message ||
          (error as Error)?.message ||
          'Failed to delete game',
        variant: 'destructive',
      })
    },
  })

  // Helper function to format draw frequency for display
  const formatDrawFrequency = (frequency: string) => {
    const frequencyMap: Record<string, string> = {
      daily: 'Daily',
      weekly: 'Weekly',
      bi_weekly: 'Bi-Weekly',
      monthly: 'Monthly',
      special: 'Special',
    }
    return frequencyMap[frequency] || frequency
  }

  const resetForm = () => {
    setFormData({
      code: '',
      name: '',
      organizer: 'rand_lottery',
      game_category: 'private',
      format: '5_by_90',
      bet_types: [],
      game_format: '5_by_90',
      game_type: '5_by_90', // Keep for backward compatibility
      number_range_min: 1,
      number_range_max: 90,
      selection_count: 5,
      bonus_number_enabled: false,
      draw_frequency: 'daily',
      draw_days: [],
      draw_time: '',
      start_date: '',
      start_time: '',
      end_date: '',
      end_time: '',
      min_stake: 0.5,
      max_stake: 200,
      base_price: 1,
      max_tickets_per_player: 10,
      max_tickets_per_transaction: 10,
      sales_cutoff_minutes: 30,
      multi_draw_enabled: false,
      weekly_schedule: false,
    })
  }

  const openEditDialog = (game: Game) => {
    console.log('Opening edit dialog for game:', game)
    console.log('Game ID:', game.id)
    setSelectedGame(game)
    setFormData({
      ...formData,
      code: game.code,
      name: game.name,
      game_type: game.game_type || game.type,
      number_range_min: game.number_range_min,
      number_range_max: game.number_range_max,
      selection_count: game.selection_count,
      bonus_number_enabled: false,
      bonus_range_min: 1,
      bonus_range_max: 10,
      bonus_count: 1,
      draw_frequency: game.draw_frequency,
      draw_days: game.draw_days,
      draw_time: game.draw_time,
      start_time: game.draw_time,
      end_time: '',
      sales_cutoff_minutes: game.sales_cutoff_minutes,
      base_price: game.base_price || game.ticket_price || 1,
      max_tickets_per_player: game.max_tickets_per_player,
    })
    setIsEditDialogOpen(true)
  }

  const handleActivateGame = (game: Game) => {
    setGameToActivate(game)
    setIsActivateDialogOpen(true)
  }

  const confirmActivateGame = () => {
    if (!gameToActivate) return
    activateGameMutation.mutate(gameToActivate.id, {
      onSuccess: () => {
        setIsActivateDialogOpen(false)
        setGameToActivate(null)
      },
    })
  }

  const getStatusBadge = (status: Game['status']) => {
    // Normalize status to handle both backend (lowercase) and frontend (capitalized) formats
    const normalizedStatus = typeof status === 'string' ? status.toLowerCase() : status

    const statusConfig = {
      draft: { color: 'bg-gray-100 text-gray-800', icon: Edit, label: 'Draft' },
      active: { color: 'bg-green-100 text-green-800', icon: CheckCircle, label: 'Active' },
      suspended: { color: 'bg-red-100 text-red-800', icon: XCircle, label: 'Suspended' },
      terminated: { color: 'bg-gray-100 text-gray-600', icon: Trash2, label: 'Terminated' },
    }

    const config = statusConfig[normalizedStatus as keyof typeof statusConfig] || {
      color: 'bg-gray-100 text-gray-800',
      icon: Edit,
      label: status,
    }
    const Icon = config.icon

    return (
      <Badge className={`${config.color} inline-flex items-center gap-1 w-fit`}>
        <Icon className="h-3 w-3" />
        {config.label}
      </Badge>
    )
  }

  const normalizeStatus = (status?: string) =>
    (status || '')
      .toLowerCase()
      .replace(/[\s-]+/g, '_')
      .trim()

  const isDraftLikeStatus = (status?: string) => {
    const normalized = normalizeStatus(status)
    return normalized === 'draft' || normalized === 'pending_approval' || normalized === 'pendingapproval'
  }

  const filteredGames =
    gamesData?.data?.filter(game => {
      const matchesSearch =
        game.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        game.code.toLowerCase().includes(searchTerm.toLowerCase())

      if (activeTab === 'all') return matchesSearch
      if (activeTab === 'active') return matchesSearch && normalizeStatus(game.status) === 'active'
      if (activeTab === 'draft') return matchesSearch && isDraftLikeStatus(game.status)
      return matchesSearch
    }) || []

  return (
    <AdminLayout>
      <div className="p-3 sm:p-4 md:p-6 space-y-3 sm:space-y-4 md:space-y-6">
        {/* Header */}
        <div className="flex flex-col sm:flex-row justify-between sm:items-center gap-3">
          <div>
            <h1 className="text-xl sm:text-2xl font-bold text-gray-900">Game Management</h1>
            <p className="text-xs sm:text-sm text-gray-500 mt-1">
              Manage lottery games and configurations
            </p>
          </div>
          <div className="flex flex-col sm:flex-row gap-2 sm:gap-3">
            <Button
              onClick={() => {
                resetForm()
                setIsCreateDialogOpen(true)
              }}
              className="justify-start sm:justify-center"
            >
              <Plus className="h-4 w-4 mr-2" />
              Create Game
            </Button>
          </div>
        </div>

        {/* Stats Cards */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Total Games</CardTitle>
              <Gamepad2 className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{gamesData?.data?.length || 0}</div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Active Games</CardTitle>
              <Play className="h-4 w-4 text-green-600" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {gamesData?.data?.filter(g => g.status?.toLowerCase() === 'active').length || 0}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Draft Games</CardTitle>
              <Edit className="h-4 w-4 text-gray-600" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {gamesData?.data?.filter(g => isDraftLikeStatus(g.status)).length || 0}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Scheduled Games</CardTitle>
              <Calendar className="h-4 w-4 text-blue-600" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{scheduledGamesData?.length || 0}</div>
              <p className="text-xs text-muted-foreground mt-1">This week</p>
            </CardContent>
          </Card>
        </div>

        {/* Main Tabs */}
        <Tabs value={mainTab} onValueChange={setMainTab} className="space-y-4">
          <TabsList>
            <TabsTrigger value="schedules" className="flex items-center gap-2">
              <Calendar className="h-4 w-4" />
              Scheduled Games
            </TabsTrigger>
            <TabsTrigger value="games" className="flex items-center gap-2">
              <List className="h-4 w-4" />
              Games List
            </TabsTrigger>
          </TabsList>

          <TabsContent value="games">
            {/* Games Table */}
            <Card>
              <CardHeader>
                <div className="flex justify-between items-center">
                  <CardTitle>Games</CardTitle>
                  <div className="flex items-center gap-4">
                    <div className="relative">
                      <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 h-4 w-4" />
                      <Input
                        placeholder="Search games..."
                        value={searchTerm}
                        onChange={e => setSearchTerm(e.target.value)}
                        className="pl-10 w-64"
                      />
                    </div>
                    <Tabs value={activeTab} onValueChange={setActiveTab}>
                      <TabsList>
                        <TabsTrigger value="active">Active</TabsTrigger>
                        <TabsTrigger value="all">All</TabsTrigger>
                        <TabsTrigger value="draft">Draft</TabsTrigger>
                      </TabsList>
                    </Tabs>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                {isLoading ? (
                  <div className="flex justify-center items-center py-8">
                    <Loader2 className="h-6 w-6 animate-spin" />
                  </div>
                ) : error ? (
                  <div className="text-center py-8 text-red-600">
                    <AlertCircle className="h-8 w-8 mx-auto mb-2" />
                    <p>Failed to load games</p>
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-12">Logo</TableHead>
                        <TableHead>Code</TableHead>
                        <TableHead>Name</TableHead>
                        <TableHead>Category</TableHead>
                        <TableHead>Format</TableHead>
                        <TableHead>Frequency</TableHead>
                        <TableHead>Draw Time</TableHead>
                        <TableHead>Status</TableHead>
                        <TableHead>Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {filteredGames.map(game => (
                        <TableRow key={game.id}>
                          <TableCell>
                            <div className="flex items-center justify-center">
                              {game.logo_url ? (
                                <img
                                  src={game.logo_url}
                                  alt={`${game.name} logo`}
                                  className="h-8 w-8 object-contain rounded"
                                />
                              ) : game.brand_color ? (
                                <div
                                  className="h-8 w-8 rounded border border-gray-200"
                                  style={{ backgroundColor: game.brand_color }}
                                  title={game.brand_color}
                                />
                              ) : (
                                <Gamepad2 className="h-6 w-6 text-gray-300" />
                              )}
                            </div>
                          </TableCell>
                          <TableCell className="font-medium">{game.code}</TableCell>
                          <TableCell>{game.name}</TableCell>
                          <TableCell>
                            <Badge variant="outline">
                              {game.game_category
                                ? game.game_category.charAt(0).toUpperCase() +
                                  game.game_category.slice(1)
                                : '-'}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <Badge variant="secondary">
                              {game.game_format?.replace(/_/g, '/').toUpperCase() || '-'}
                            </Badge>
                          </TableCell>
                          <TableCell>{formatDrawFrequency(game.draw_frequency || '')}</TableCell>
                          <TableCell>{game.draw_time || '-'}</TableCell>
                          <TableCell>{getStatusBadge(game.status)}</TableCell>
                          <TableCell>
                            <div className="flex items-center gap-2">
                              {/* Only show edit button for non-terminated games */}
                              {game.status?.toLowerCase() !== 'terminated' && (
                                <>
                                  <Button
                                    variant="ghost"
                                    size="sm"
                                    onClick={() => openEditDialog(game)}
                                    title="Edit Game"
                                  >
                                    <Edit className="h-4 w-4" />
                                  </Button>
                                  <Button
                                    variant="ghost"
                                    size="sm"
                                    onClick={() => {
                                      setGameForBranding(game)
                                      setIsBrandingDialogOpen(true)
                                    }}
                                    title="Manage Branding"
                                  >
                                    <Palette className="h-4 w-4 text-purple-600" />
                                  </Button>
                                </>
                              )}
                              {/* Activate draft games directly */}
                              {game.status?.toLowerCase() === 'draft' && (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleActivateGame(game)}
                                  title="Activate Game"
                                  disabled={activateGameMutation.isPending}
                                >
                                  {activateGameMutation.isPending ? (
                                    <Loader2 className="h-4 w-4 animate-spin text-green-600" />
                                  ) : (
                                    <Play className="h-4 w-4 text-green-600" />
                                  )}
                                </Button>
                              )}
                              {game.status?.toLowerCase() === 'active' && (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() =>
                                    suspendGameMutation.mutate({
                                      id: game.id,
                                      reason: 'Manual suspension',
                                    })
                                  }
                                  title="Suspend Game"
                                >
                                  <Pause className="h-4 w-4 text-red-600" />
                                </Button>
                              )}
                              {/* Reactivate suspended games */}
                              {game.status?.toLowerCase() === 'suspended' && (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleActivateGame(game)}
                                  title="Reactivate Game"
                                  disabled={activateGameMutation.isPending}
                                >
                                  {activateGameMutation.isPending ? (
                                    <Loader2 className="h-4 w-4 animate-spin text-green-600" />
                                  ) : (
                                    <Play className="h-4 w-4 text-green-600" />
                                  )}
                                </Button>
                              )}
                              {/* Only allow deleting draft games or suspended games */}
                              {(game.status?.toLowerCase() === 'draft' ||
                                game.status?.toLowerCase() === 'suspended') && (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => {
                                    setGameToDelete(game)
                                    setIsDeleteDialogOpen(true)
                                  }}
                                  title="Delete Game"
                                >
                                  <Trash2 className="h-4 w-4 text-red-600" />
                                </Button>
                              )}
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="schedules">
            <Card>
              <CardContent className="pt-6">
                <ScheduledGamesView
                  onGenerateSchedule={selectedWeek => {
                    setWeekForGeneration(selectedWeek)
                    setIsGenerateScheduleDialogOpen(true)
                  }}
                  onEditSchedule={schedule => {
                    setSelectedSchedule(schedule)
                    setIsEditScheduleDialogOpen(true)
                  }}
                />
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>

        {/* Edit Schedule Dialog */}
        <EditScheduleDialog
          isOpen={isEditScheduleDialogOpen}
          onClose={() => {
            setIsEditScheduleDialogOpen(false)
            setSelectedSchedule(null)
          }}
          schedule={selectedSchedule}
        />

        {/* Generate Schedule Dialog */}
        <GenerateScheduleDialog
          isOpen={isGenerateScheduleDialogOpen}
          onClose={() => setIsGenerateScheduleDialogOpen(false)}
          selectedWeek={weekForGeneration}
        />

        {/* Create Game Dialog */}
        <CreateGameWizard
          isOpen={isCreateDialogOpen}
          onClose={() => setIsCreateDialogOpen(false)}
        />

        {/* Edit Game Dialog */}
        <EditGameWizard
          isOpen={isEditDialogOpen}
          onClose={() => setIsEditDialogOpen(false)}
          game={selectedGame}
        />

        {/* Game Branding Dialog */}
        <GameBrandingDialog
          isOpen={isBrandingDialogOpen}
          onClose={() => {
            setIsBrandingDialogOpen(false)
            setGameForBranding(null)
          }}
          game={gameForBranding}
        />

        {/* Activate Game Confirmation Dialog */}
        <Dialog open={isActivateDialogOpen} onOpenChange={setIsActivateDialogOpen}>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>Activate Game</DialogTitle>
              <DialogDescription>
                Are you sure you want to activate "{gameToActivate?.name}"? This will make the game
                available for players to purchase tickets.
              </DialogDescription>
            </DialogHeader>

            {gameToActivate && (
              <div className="grid gap-4 py-4">
                <div className="space-y-2">
                  <div className="font-semibold">
                    {gameToActivate.name} ({gameToActivate.code})
                  </div>
                  <div className="text-sm text-gray-600">
                    Format: {gameToActivate.selection_count}/{gameToActivate.number_range_max}
                  </div>
                  <div className="text-sm text-gray-600">
                    Base Price: GHS {gameToActivate.base_price || gameToActivate.ticket_price}
                  </div>
                  <div className="text-sm text-gray-600">
                    Draw Frequency: {gameToActivate.draw_frequency}
                  </div>
                </div>
                <div className="p-3 bg-yellow-50 border border-yellow-200 rounded-md">
                  <div className="text-sm text-yellow-800">
                    <strong>Warning:</strong> Once activated, players will be able to purchase
                    tickets for this game. Make sure all game configuration is correct.
                  </div>
                </div>
              </div>
            )}

            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => {
                  setIsActivateDialogOpen(false)
                  setGameToActivate(null)
                }}
              >
                Cancel
              </Button>
              <Button
                onClick={confirmActivateGame}
                disabled={activateGameMutation.isPending}
                className="bg-green-600 hover:bg-green-700"
              >
                {activateGameMutation.isPending && (
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                )}
                Activate Game
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Delete Confirmation Dialog */}
        <AlertDialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Are you sure?</AlertDialogTitle>
              <AlertDialogDescription>
                This will permanently delete the game &quot;{gameToDelete?.name}&quot;. This action
                cannot be undone.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => gameToDelete && deleteGameMutation.mutate(gameToDelete.id)}
                className="bg-red-600 hover:bg-red-700"
              >
                Delete
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </AdminLayout>
  )
}
