import React, { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { 
  Users, 
  Search, 
  Filter, 
  Eye, 
  Edit, 
  Ban, 
  CheckCircle, 
  XCircle,
  AlertCircle,
  Phone,
  Mail,
  Calendar,
  Ticket,
  Download,
  UserPlus,
} from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Pagination, PaginationContent, PaginationEllipsis,
  PaginationItem, PaginationLink, PaginationNext, PaginationPrevious,
} from '@/components/ui/pagination'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
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
  DialogTrigger,
} from '@/components/ui/dialog'

import { toast } from '@/hooks/use-toast'
import { formatInGhanaTime } from '@/lib/date-utils'
import PageHeader from '@/components/ui/page-header'

export interface Player {
  player_id: string
  name: string
  phone_number: string
  email?: string
  date_registered: string
  account_status: 'active' | 'suspended' | 'banned' | 'pending_verification'
  total_tickets_purchased: number
  total_amount_spent: number
  total_winnings: number
  last_activity: string
  verification_status: 'verified' | 'pending' | 'rejected'
  kyc_level: 'basic' | 'intermediate' | 'advanced'
  wallet_balance: number
  created_at: string
  updated_at: string
  phone_verified?: boolean
  email_verified?: boolean
}

export interface PlayerStatistics {
  total_players: number
  active_players: number
  suspended_players: number
  banned_players: number
  pending_verification: number
  total_tickets_sold: number
  total_revenue: number
  total_winnings_paid: number
}

import api from '@/lib/api'

// Real API service using the admin API client (handles auth automatically)
const playersService = {
  async getPlayers(params?: {
    search?: string
    status?: string
    verification_status?: string
    page?: number
    limit?: number
  }): Promise<{ players: Player[]; total_count: number; page: number; limit: number }> {
    try {
      const q: Record<string, string> = {}
      if (params?.search) q.search = params.search
      if (params?.status) q.status = params.status
      if (params?.page) q.page = String(params.page)
      q.limit = String(params?.limit || 50)

      const res = await api.get('/admin/players/search', { params: q })
      const raw: any[] = res.data?.data?.players || res.data?.players || []

      // Fetch ticket counts — page through all since backend caps at 10
      let ticketCountMap: Record<string, number> = {}
      let amountMap: Record<string, number> = {}
      try {
        let pg = 1
        while (true) {
          let tickets: any[] = []
          try {
            const ticketRes = await api.get('/admin/tickets', { params: { page: pg } })
            tickets = ticketRes.data?.data?.tickets || ticketRes.data?.tickets || []
          } catch { break }
          tickets.forEach((t: any) => {
            const id = t.issuer_id || t.issuerId
            if (id) {
              ticketCountMap[id] = (ticketCountMap[id] || 0) + 1
              amountMap[id] = (amountMap[id] || 0) + Number(t.total_amount || 0)
            }
          })
          if (tickets.length < 10) break
          pg++
          if (pg > 30) break
        }
      } catch { /* non-fatal */ }

      const players: Player[] = raw.map((p: any) => {
        const pid = p.id || p.player_id
        return {
          player_id: pid,
          name: `${p.first_name || ''} ${p.last_name || ''}`.trim() || p.phone_number,
          phone_number: p.phone_number,
          email: p.email,
          date_registered: p.created_at,
          account_status: (p.status || 'ACTIVE').toLowerCase() as Player['account_status'],
          total_tickets_purchased: ticketCountMap[pid] || 0,
          total_amount_spent: amountMap[pid] || 0,
          total_winnings: p.total_winnings || 0,
          last_activity: p.last_login_at || p.updated_at || p.created_at,
          verification_status: p.phone_verified ? 'verified' : 'pending',
          kyc_level: 'basic' as const,
          wallet_balance: p.wallet_balance || 0,
          created_at: p.created_at,
          updated_at: p.updated_at || p.created_at,
          phone_verified: !!p.phone_verified,
          email_verified: !!p.email_verified,
        }
      })
      return { players, total_count: res.data?.data?.total || players.length, page: 1, limit: 20 }
    } catch {
      return { players: [], total_count: 0, page: 1, limit: 20 }
    }
  },

  async getPlayerStatistics(): Promise<PlayerStatistics> {
    const { players, total_count } = await this.getPlayers({ limit: 200 })
    return {
      total_players: total_count,
      active_players: players.filter(p => p.account_status === 'active').length,
      suspended_players: players.filter(p => p.account_status === 'suspended').length,
      banned_players: players.filter(p => p.account_status === 'banned').length,
      pending_verification: players.filter(p => p.verification_status === 'pending').length,
      total_tickets_sold: players.reduce((s, p) => s + p.total_tickets_purchased, 0),
      total_revenue: players.reduce((s, p) => s + p.total_amount_spent, 0),
      total_winnings_paid: players.reduce((s, p) => s + p.total_winnings, 0),
    }
  },

  async updatePlayerStatus(playerId: string, status: string): Promise<void> {
    const endpoint = status === 'suspended' ? 'suspend' : 'activate'
    await api.post(`/admin/players/${playerId}/${endpoint}`, { reason: 'Admin action' })
  },

  async getPlayerDetails(playerId: string): Promise<Player> {
    const res = await api.get(`/admin/players/${playerId}`)
    const p = res.data?.data?.player || res.data?.data || res.data
    return {
      player_id: p.id,
      name: `${p.first_name || ''} ${p.last_name || ''}`.trim() || p.phone_number,
      phone_number: p.phone_number,
      email: p.email,
      date_registered: p.created_at,
      account_status: (p.status || 'active').toLowerCase() as Player['account_status'],
      total_tickets_purchased: 0,
      total_amount_spent: 0,
      total_winnings: 0,
      last_activity: p.last_login_at || p.updated_at,
      verification_status: p.phone_verified ? 'verified' : 'pending',
      kyc_level: 'basic',
      wallet_balance: 0,
      created_at: p.created_at,
      updated_at: p.updated_at,
    }
  },
}

const formatCurrency = (amount: number) => {
  return new Intl.NumberFormat('en-GH', {
    style: 'currency',
    currency: 'GHS',
    minimumFractionDigits: 2
  }).format(amount / 100) // Convert from pesewas to cedis
}

const PlayersModule: React.FC = () => {
  const queryClient = useQueryClient()
  
  // Filter states
  const [searchFilter, setSearchFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [verificationFilter, setVerificationFilter] = useState('')
  const [selectedPlayer, setSelectedPlayer] = useState<Player | null>(null)
  const [page, setPage] = useState(1)

  // Fetch players
  const { data: playersData, isLoading: playersLoading } = useQuery({
    queryKey: ['players', searchFilter, statusFilter, verificationFilter],
    queryFn: () => playersService.getPlayers({
      search: searchFilter || undefined,
      status: statusFilter || undefined,
      verification_status: verificationFilter || undefined,
    }),
  })

  // Fetch statistics
  const { data: statistics } = useQuery({
    queryKey: ['player-statistics'],
    queryFn: () => playersService.getPlayerStatistics(),
  })

  // Update player status mutation
  const updateStatusMutation = useMutation({
    mutationFn: ({ playerId, status }: { playerId: string; status: string }) =>
      playersService.updatePlayerStatus(playerId, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['players'] })
      queryClient.invalidateQueries({ queryKey: ['player-statistics'] })
      toast({
        title: 'Player Status Updated',
        description: 'Player status has been updated successfully'
      })
    },
    onError: (error: Error) => {
      toast({
        title: 'Update Failed',
        description: error.message,
        variant: 'destructive'
      })
    }
  })

  const handleStatusUpdate = (playerId: string, newStatus: string) => {
    updateStatusMutation.mutate({ playerId, status: newStatus })
  }

  const getStatusBadge = (status: string) => {
    const statusConfig = {
      active: { variant: 'default' as const, label: 'Active' },
      suspended: { variant: 'secondary' as const, label: 'Suspended' },
      banned: { variant: 'destructive' as const, label: 'Banned' },
      pending_verification: { variant: 'outline' as const, label: 'Pending Verification' }
    }
    
    const config = statusConfig[status as keyof typeof statusConfig] || statusConfig.active
    return <Badge variant={config.variant}>{config.label}</Badge>
  }

  const getVerificationBadge = (status: string) => {
    const statusConfig = {
      verified: { variant: 'default' as const, label: 'Verified', icon: CheckCircle },
      pending: { variant: 'secondary' as const, label: 'Pending', icon: Calendar },
      rejected: { variant: 'destructive' as const, label: 'Rejected', icon: XCircle }
    }
    
    const config = statusConfig[status as keyof typeof statusConfig] || statusConfig.pending
    const Icon = config.icon
    
    return (
      <Badge variant={config.variant} className="flex items-center gap-1">
        <Icon className="h-3 w-3" />
        {config.label}
      </Badge>
    )
  }

  const filteredPlayers = playersData?.players?.filter(player => {
    if (searchFilter) {
      const search = searchFilter.toLowerCase()
      return (
        player.name.toLowerCase().includes(search) ||
        player.phone_number.includes(search) ||
        player.email?.toLowerCase().includes(search) ||
        player.player_id.toLowerCase().includes(search)
      )
    }
    return true
  }) || []

  const PAGE_SIZE = 10
  const totalPages = Math.max(1, Math.ceil(filteredPlayers.length / PAGE_SIZE))
  const safePage = Math.min(page, totalPages)
  const paginatedPlayers = useMemo(
    () => filteredPlayers.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE),
    [filteredPlayers, safePage]
  )
  function buildPageNumbers(current: number, total: number): (number | 'ellipsis')[] {
    if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
    if (current <= 4) return [1, 2, 3, 4, 5, 'ellipsis', total]
    if (current >= total - 3) return [1, 'ellipsis', total - 4, total - 3, total - 2, total - 1, total]
    return [1, 'ellipsis', current - 1, current, current + 1, 'ellipsis', total]
  }
  const pageNums = buildPageNumbers(safePage, totalPages)

  if (playersLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title="Players Module"
        description="Manage player accounts, verification status, and activity"
        badge="Live"
      >
        <Button variant="outline" size="sm">
          <Download className="h-4 w-4 mr-2" />
          Export Players
        </Button>
        <Button size="sm">
          <UserPlus className="h-4 w-4 mr-2" />
          Add Player
        </Button>
      </PageHeader>

      {/* Statistics Cards */}
      <div className="grid gap-4 md:grid-cols-5">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Players</CardTitle>
            <Users className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {statistics?.total_players?.toLocaleString() || '0'}
            </div>
            <p className="text-xs text-muted-foreground">
              Registered accounts
            </p>
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Players</CardTitle>
            <CheckCircle className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {statistics?.active_players?.toLocaleString() || '0'}
            </div>
            <p className="text-xs text-muted-foreground">
              Active accounts
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Pending Verification</CardTitle>
            <Calendar className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {statistics?.pending_verification?.toLocaleString() || '0'}
            </div>
            <p className="text-xs text-muted-foreground">
              Awaiting verification
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Tickets</CardTitle>
            <Ticket className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {statistics?.total_tickets_sold?.toLocaleString() || '0'}
            </div>
            <p className="text-xs text-muted-foreground">
              Tickets purchased
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Revenue</CardTitle>
            <Users className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatCurrency(statistics?.total_revenue || 0)}
            </div>
            <p className="text-xs text-muted-foreground">
              Player spending
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Filters</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            <div>
              <Label>Search Players</Label>
              <div className="relative">
                <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search by name, phone, email, or player ID..."
                  value={searchFilter}
                  onChange={(e) => { setSearchFilter(e.target.value); setPage(1) }}
                  className="pl-8"
                />
              </div>
            </div>
            
            <div>
              <Label>Account Status</Label>
              <Select value={statusFilter || "all"} onValueChange={(value) => { setStatusFilter(value === "all" ? "" : value); setPage(1) }}>
                <SelectTrigger>
                  <SelectValue placeholder="All Statuses" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Statuses</SelectItem>
                  <SelectItem value="active">Active</SelectItem>
                  <SelectItem value="suspended">Suspended</SelectItem>
                  <SelectItem value="banned">Banned</SelectItem>
                  <SelectItem value="pending_verification">Pending Verification</SelectItem>
                </SelectContent>
              </Select>
            </div>
            
            <div>
              <Label>Verification Status</Label>
              <Select value={verificationFilter || "all"} onValueChange={(value) => { setVerificationFilter(value === "all" ? "" : value); setPage(1) }}>
                <SelectTrigger>
                  <SelectValue placeholder="All Verification" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Verification</SelectItem>
                  <SelectItem value="verified">Verified</SelectItem>
                  <SelectItem value="pending">Pending</SelectItem>
                  <SelectItem value="rejected">Rejected</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-end">
              <Button 
                variant="outline" 
                onClick={() => {
                  setSearchFilter('')
                  setStatusFilter('')
                  setVerificationFilter('')
                  setPage(1)
                }}
                className="w-full"
              >
                <Filter className="h-4 w-4 mr-2" />
                Clear Filters
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Players Table */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>Players ({filteredPlayers.length})</CardTitle>
            <CardDescription>
              Manage player accounts and verification status
            </CardDescription>
          </div>
          <span className="text-sm text-muted-foreground">Page {safePage} of {totalPages}</span>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Player ID</TableHead>
                  <TableHead>Name</TableHead>
                  <TableHead>Phone Number</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Date Registered</TableHead>
                  <TableHead>Account Status</TableHead>
                  <TableHead>Verification</TableHead>
                  <TableHead>Total Tickets</TableHead>
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredPlayers.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9} className="text-center py-8 text-muted-foreground">
                      No players found
                    </TableCell>
                  </TableRow>
                ) : (
                  paginatedPlayers.map((player) => (
                    <TableRow key={player.player_id}>
                      <TableCell className="font-mono font-medium">
                        {player.player_id}
                      </TableCell>
                      <TableCell>
                        <div>
                          <p className="font-medium">{player.name}</p>
                          <p className="text-xs text-muted-foreground">
                            Last active: {formatInGhanaTime(player.last_activity, 'PP')}
                          </p>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-1">
                          <Phone className="h-3 w-3 text-muted-foreground" />
                          <span className="font-mono text-sm">{player.phone_number}</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        {player.email ? (
                          <div className="flex items-center gap-1">
                            <Mail className="h-3 w-3 text-muted-foreground" />
                            <span className="text-sm">{player.email}</span>
                          </div>
                        ) : (
                          <span className="text-muted-foreground text-sm">No email</span>
                        )}
                      </TableCell>
                      <TableCell className="text-sm">
                        {formatInGhanaTime(player.date_registered, 'PP')}
                      </TableCell>
                      <TableCell>
                        {getStatusBadge(player.account_status)}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-col gap-1">
                          <div className="flex items-center gap-1">
                            <Phone className="h-3 w-3 text-muted-foreground" />
                            {player.phone_verified
                              ? <Badge variant="default" className="text-xs py-0 bg-green-600"><CheckCircle className="h-2.5 w-2.5 mr-0.5" />Phone</Badge>
                              : <Badge variant="secondary" className="text-xs py-0"><AlertCircle className="h-2.5 w-2.5 mr-0.5" />Phone</Badge>
                            }
                          </div>
                          <div className="flex items-center gap-1">
                            <Mail className="h-3 w-3 text-muted-foreground" />
                            {player.email_verified
                              ? <Badge variant="default" className="text-xs py-0 bg-green-600"><CheckCircle className="h-2.5 w-2.5 mr-0.5" />Email</Badge>
                              : <Badge variant="secondary" className="text-xs py-0"><AlertCircle className="h-2.5 w-2.5 mr-0.5" />Email</Badge>
                            }
                          </div>
                        </div>
                      </TableCell>
                      <TableCell className="text-center font-semibold">
                        {player.total_tickets_purchased}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <Dialog>
                            <DialogTrigger asChild>
                              <Button 
                                variant="outline" 
                                size="sm"
                                onClick={() => setSelectedPlayer(player)}
                              >
                                <Eye className="h-4 w-4" />
                              </Button>
                            </DialogTrigger>
                            <DialogContent className="max-w-2xl">
                              <DialogHeader>
                                <DialogTitle>Player Details</DialogTitle>
                                <DialogDescription>
                                  Complete information for {player.name}
                                </DialogDescription>
                              </DialogHeader>
                              
                              {selectedPlayer && (
                                <div className="space-y-6">
                                  {/* Basic Information */}
                                  <div className="grid grid-cols-2 gap-4">
                                    <div>
                                      <Label>Player ID</Label>
                                      <p className="font-mono text-sm">{selectedPlayer.player_id}</p>
                                    </div>
                                    <div>
                                      <Label>Name</Label>
                                      <p className="font-medium">{selectedPlayer.name}</p>
                                    </div>
                                    <div>
                                      <Label>Phone Number</Label>
                                      <p className="font-mono">{selectedPlayer.phone_number}</p>
                                    </div>
                                    <div>
                                      <Label>Email</Label>
                                      <p>{selectedPlayer.email || 'Not provided'}</p>
                                    </div>
                                    <div>
                                      <Label>Date Registered</Label>
                                      <p>{formatInGhanaTime(selectedPlayer.date_registered, 'PPp')}</p>
                                    </div>
                                    <div>
                                      <Label>Last Activity</Label>
                                      <p>{formatInGhanaTime(selectedPlayer.last_activity, 'PPp')}</p>
                                    </div>
                                  </div>

                                  {/* Status Information */}
                                  <div className="grid grid-cols-3 gap-4">
                                    <div>
                                      <Label>Account Status</Label>
                                      <div className="mt-1">
                                        {getStatusBadge(selectedPlayer.account_status)}
                                      </div>
                                    </div>
                                    <div>
                                      <Label>Verification Status</Label>
                                      <div className="mt-1">
                                        {getVerificationBadge(selectedPlayer.verification_status)}
                                      </div>
                                    </div>
                                    <div>
                                      <Label>KYC Level</Label>
                                      <Badge variant="outline" className="mt-1 capitalize">
                                        {selectedPlayer.kyc_level}
                                      </Badge>
                                    </div>
                                  </div>

                                  {/* Financial Information */}
                                  <div className="grid grid-cols-2 gap-4">
                                    <div>
                                      <Label>Total Tickets Purchased</Label>
                                      <p className="text-lg font-bold">{selectedPlayer.total_tickets_purchased}</p>
                                    </div>
                                    <div>
                                      <Label>Total Amount Spent</Label>
                                      <p className="text-lg font-bold">{formatCurrency(selectedPlayer.total_amount_spent)}</p>
                                    </div>
                                    <div>
                                      <Label>Total Winnings</Label>
                                      <p className="text-lg font-bold text-green-600">{formatCurrency(selectedPlayer.total_winnings)}</p>
                                    </div>
                                    <div>
                                      <Label>Wallet Balance</Label>
                                      <p className="text-lg font-bold">{formatCurrency(selectedPlayer.wallet_balance)}</p>
                                    </div>
                                  </div>
                                </div>
                              )}
                              
                              <DialogFooter>
                                <Button variant="outline">
                                  Close
                                </Button>
                                <Button>
                                  <Edit className="h-4 w-4 mr-2" />
                                  Edit Player
                                </Button>
                              </DialogFooter>
                            </DialogContent>
                          </Dialog>

                          {player.account_status === 'active' && (
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => handleStatusUpdate(player.player_id, 'suspended')}
                              disabled={updateStatusMutation.isPending}
                            >
                              <Ban className="h-4 w-4" />
                            </Button>
                          )}
                          
                          {player.account_status === 'suspended' && (
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => handleStatusUpdate(player.player_id, 'active')}
                              disabled={updateStatusMutation.isPending}
                            >
                              <CheckCircle className="h-4 w-4" />
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>

          {totalPages > 1 && (
            <div className="mt-4">
              <Pagination>
                <PaginationContent>
                  <PaginationItem>
                    <PaginationPrevious href="#"
                      onClick={e => { e.preventDefault(); setPage(p => Math.max(1, p - 1)) }}
                      aria-disabled={safePage === 1}
                      className={safePage === 1 ? 'pointer-events-none opacity-50' : ''} />
                  </PaginationItem>
                  {pageNums.map((n, i) =>
                    n === 'ellipsis' ? (
                      <PaginationItem key={`e${i}`}><PaginationEllipsis /></PaginationItem>
                    ) : (
                      <PaginationItem key={n}>
                        <PaginationLink href="#" isActive={n === safePage}
                          onClick={e => { e.preventDefault(); setPage(n) }}>{n}</PaginationLink>
                      </PaginationItem>
                    )
                  )}
                  <PaginationItem>
                    <PaginationNext href="#"
                      onClick={e => { e.preventDefault(); setPage(p => Math.min(totalPages, p + 1)) }}
                      aria-disabled={safePage === totalPages}
                      className={safePage === totalPages ? 'pointer-events-none opacity-50' : ''} />
                  </PaginationItem>
                </PaginationContent>
              </Pagination>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

export default PlayersModule