import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
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
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination'
import { DatePicker } from '@/components/ui/date-picker'
import { Trophy, Search, Clock, AlertCircle, Filter } from 'lucide-react'
import { drawService, type Draw } from '@/services/draws'
import { gameService } from '@/services/games'
import { formatCurrency } from '@/lib/utils'
import { protoTimestampToDate, formatInGhanaTime } from '@/lib/date-utils'

// Helper to convert proto status enum to string
const protoStatusToString = (status: number | string): string => {
  if (typeof status === 'string') return status
  const statusMap: Record<number, string> = {
    0: 'unspecified',
    1: 'scheduled',
    2: 'in_progress',
    3: 'completed',
    4: 'failed',
    5: 'cancelled',
  }
  return statusMap[status] || 'unspecified'
}

export default function Draws() {
  const navigate = useNavigate({ from: '/draws' })
  const searchParams = useSearch({ from: '/draws' })

  const [searchTerm, setSearchTerm] = useState('')
  const [showFilters, setShowFilters] = useState(false)

  // Get filter values from URL or defaults
  const currentPage = searchParams.page || 1
  const statusFilter = searchParams.status || 'all'
  const gameFilter = searchParams.game || 'all'
  const startDate = searchParams.startDate ? new Date(searchParams.startDate) : undefined
  const endDate = searchParams.endDate ? new Date(searchParams.endDate) : undefined
  const pageSize = 20

  // Helper to update URL search params
  const updateFilters = (updates: Partial<typeof searchParams>) => {
    navigate({
      search: prev => ({
        ...prev,
        ...updates,
        // Remove undefined values
        ...(updates.page === 1 ? { page: undefined } : {}),
        ...(updates.status === 'all' ? { status: undefined } : {}),
        ...(updates.game === 'all' ? { game: undefined } : {}),
        ...(updates.startDate === undefined ? { startDate: undefined } : {}),
        ...(updates.endDate === undefined ? { endDate: undefined } : {}),
      }),
    })
  }

  const setCurrentPage = (page: number) => updateFilters({ page })
  const setStatusFilter = (status: string) => updateFilters({ status, page: 1 })
  const setGameFilter = (game: string) => updateFilters({ game, page: 1 })
  const setStartDate = (date: Date | undefined) =>
    updateFilters({ startDate: date?.toISOString(), page: 1 })
  const setEndDate = (date: Date | undefined) =>
    updateFilters({ endDate: date?.toISOString(), page: 1 })

  // Fetch games for filter dropdown
  const { data: gamesData } = useQuery({
    queryKey: ['games'],
    queryFn: async () => {
      const response = await gameService.getGames(1, 100)
      return response.data || []
    },
  })

  // Fetch draws with pagination and filters
  const { data: drawsResponse } = useQuery({
    queryKey: ['draws', statusFilter, gameFilter, startDate, endDate, currentPage, pageSize],
    queryFn: async () => {
      const params: Record<string, string | number> = {
        page: currentPage,
        per_page: pageSize,
      }

      if (statusFilter !== 'all') {
        params.status = statusFilter
      }
      if (gameFilter !== 'all') {
        params.game_id = gameFilter
      }
      if (startDate) {
        params.start_date = startDate.toISOString()
      }
      if (endDate) {
        params.end_date = endDate.toISOString()
      }

      const response = await drawService.getDraws(params)
      return response
    },
    placeholderData: {
      draws: [],
      total_count: 0,
    },
    retry: 0,
    refetchOnWindowFocus: false,
  })

  const draws = Array.isArray(drawsResponse?.draws) ? drawsResponse.draws : []
  const totalCount = drawsResponse?.total_count || 0
  const totalPages = Math.ceil(totalCount / pageSize)

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'scheduled':
        return <Badge className="bg-blue-100 text-blue-800">Scheduled</Badge>
      case 'in_progress':
        return <Badge className="bg-orange-100 text-orange-800">In Progress</Badge>
      case 'active':
        return <Badge className="bg-green-100 text-green-800">Active</Badge>
      case 'closed':
        return <Badge className="bg-yellow-100 text-yellow-800">Closed</Badge>
      case 'completed':
        return <Badge className="bg-purple-100 text-purple-800">Completed</Badge>
      case 'cancelled':
        return <Badge className="bg-red-100 text-red-800">Cancelled</Badge>
      default:
        return <Badge variant="outline">{status}</Badge>
    }
  }

  // Calculate stats
  const stats = {
    totalDraws: totalCount,
    activeDraws: draws.filter((d: Draw) => protoStatusToString(d.status) === 'in_progress').length,
    closedDraws: draws.filter((d: Draw) => protoStatusToString(d.status) === 'completed').length,
    totalStakes: draws.reduce((sum: number, d: Draw) => sum + (d.total_stakes || 0), 0),
  }

  // Filter draws by search term locally
  const filteredDraws = searchTerm
    ? draws.filter((draw: Draw) => {
        const searchLower = searchTerm.toLowerCase()
        return (
          draw.draw_name?.toLowerCase().includes(searchLower) ||
          draw.game_name?.toLowerCase().includes(searchLower) ||
          draw.draw_number?.toString().includes(searchLower) ||
          draw.id.toLowerCase().includes(searchLower)
        )
      })
    : draws

  return (
    <div className="p-3 sm:p-4 md:p-6 space-y-3 sm:space-y-4 md:space-y-6">
      <div>
        <h1 className="text-2xl sm:text-3xl font-bold tracking-tight">Draw Management</h1>
        <p className="text-sm sm:text-base text-muted-foreground">
          Monitor and manage lottery draw executions
        </p>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-3 sm:gap-4 grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-xs sm:text-sm font-medium">Total Draws</CardTitle>
            <Trophy className="h-4 w-4 shrink-0 text-muted-foreground" />
          </CardHeader>
          <CardContent className="p-3 sm:p-6 pt-0">
            <div className="text-lg sm:text-2xl font-bold">{stats.totalDraws}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-xs sm:text-sm font-medium">Active</CardTitle>
            <Clock className="h-4 w-4 shrink-0 text-muted-foreground" />
          </CardHeader>
          <CardContent className="p-3 sm:p-6 pt-0">
            <div className="text-lg sm:text-2xl font-bold">{stats.activeDraws}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-xs sm:text-sm font-medium">Closed (Pending)</CardTitle>
            <AlertCircle className="h-4 w-4 shrink-0 text-muted-foreground" />
          </CardHeader>
          <CardContent className="p-3 sm:p-6 pt-0">
            <div className="text-lg sm:text-2xl font-bold">{stats.closedDraws}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-xs sm:text-sm font-medium">Total Stakes</CardTitle>
            <Trophy className="h-4 w-4 shrink-0 text-muted-foreground" />
          </CardHeader>
          <CardContent className="p-3 sm:p-6 pt-0">
            <div className="text-lg sm:text-2xl font-bold truncate">
              {formatCurrency(stats.totalStakes)}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Draws Management */}
      <Card>
        <CardHeader>
          <div className="flex flex-col sm:flex-row justify-between sm:items-center gap-3">
            <div>
              <CardTitle className="text-lg sm:text-xl">Draws</CardTitle>
              <CardDescription className="text-xs sm:text-sm">
                Monitor and manage lottery draw executions
              </CardDescription>
            </div>
            <div className="flex flex-col sm:flex-row gap-2 items-stretch sm:items-center">
              <div className="relative flex-1 sm:flex-initial">
                <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search draws..."
                  className="pl-8 w-full sm:w-64"
                  value={searchTerm}
                  onChange={e => setSearchTerm(e.target.value)}
                />
              </div>
              <Button variant="outline" size="sm" onClick={() => setShowFilters(!showFilters)}>
                <Filter className="h-4 w-4 mr-1" />
                Filters
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {showFilters && (
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4 p-4 border rounded-lg">
              <div className="space-y-2">
                <Label htmlFor="game">Game</Label>
                <Select value={gameFilter} onValueChange={setGameFilter}>
                  <SelectTrigger id="game">
                    <SelectValue placeholder="All games" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All games</SelectItem>
                    {gamesData?.map(game => (
                      <SelectItem key={game.id} value={game.id}>
                        {game.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="status">Status</Label>
                <Select value={statusFilter} onValueChange={setStatusFilter}>
                  <SelectTrigger id="status">
                    <SelectValue placeholder="All statuses" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All statuses</SelectItem>
                    <SelectItem value="scheduled">Scheduled</SelectItem>
                    <SelectItem value="in_progress">In Progress</SelectItem>
                    <SelectItem value="completed">Completed</SelectItem>
                    <SelectItem value="cancelled">Cancelled</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="startDate">Start Date</Label>
                <DatePicker
                  value={startDate}
                  onChange={setStartDate}
                  placeholder="Select start date"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="endDate">End Date</Label>
                <DatePicker value={endDate} onChange={setEndDate} placeholder="Select end date" />
              </div>
            </div>
          )}

          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-xs sm:text-sm">Draw Number</TableHead>
                  <TableHead className="text-xs sm:text-sm hidden md:table-cell">Game</TableHead>
                  <TableHead className="text-xs sm:text-sm">Draw Date</TableHead>
                  <TableHead className="text-xs sm:text-sm">Status</TableHead>
                  <TableHead className="text-xs sm:text-sm hidden lg:table-cell">
                    Tickets Sold
                  </TableHead>
                  <TableHead className="text-xs sm:text-sm hidden lg:table-cell">
                    Total Stakes
                  </TableHead>
                  <TableHead className="text-xs sm:text-sm hidden xl:table-cell">
                    Winning Numbers
                  </TableHead>
                  <TableHead className="text-xs sm:text-sm hidden xl:table-cell">Draw ID</TableHead>
                  <TableHead className="text-xs sm:text-sm hidden xl:table-cell">Schedule ID</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredDraws.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9} className="text-center py-12">
                      <Trophy className="mx-auto h-12 w-12 text-gray-400" />
                      <h3 className="mt-2 text-sm font-medium text-gray-900">No draws found</h3>
                      <p className="mt-1 text-sm text-gray-500">
                        {searchTerm || statusFilter !== 'all' || gameFilter !== 'all'
                          ? 'Try adjusting your filters'
                          : 'Get started by creating a new draw'}
                      </p>
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredDraws.map((draw: Draw) => {
                    const drawDate = draw.draw_date
                      ? new Date(draw.draw_date)
                      : draw.scheduled_time
                        ? protoTimestampToDate(draw.scheduled_time)
                        : new Date()
                    const statusString = protoStatusToString(draw.status)
                    return (
                      <TableRow key={draw.id}>
                        <TableCell className="font-medium text-xs sm:text-sm">
                          <Link
                            to="/draw/$drawId"
                            params={{ drawId: draw.id }}
                            className="text-blue-600 hover:text-blue-800 hover:underline font-medium"
                          >
                            {draw.draw_number
                              ? `Draw #${draw.draw_number}`
                              : draw.draw_name || draw.id.slice(0, 8)}
                          </Link>
                        </TableCell>
                        <TableCell className="text-xs sm:text-sm hidden md:table-cell">
                          <div>
                            <div className="font-medium">
                              {draw.game_name || draw.game?.name || 'N/A'}
                            </div>
                            <div className="text-sm text-muted-foreground">
                              {draw.game?.code || draw.draw_location || '-'}
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="text-xs sm:text-sm">
                          {formatInGhanaTime(drawDate, 'PPP p')}
                        </TableCell>
                        <TableCell className="text-xs sm:text-sm">
                          {getStatusBadge(statusString)}
                        </TableCell>
                        <TableCell className="text-xs sm:text-sm hidden lg:table-cell">
                          {(draw.total_tickets_sold || 0).toLocaleString()}
                        </TableCell>
                        <TableCell className="text-xs sm:text-sm hidden lg:table-cell">
                          {formatCurrency(draw.total_stakes || 0)}
                        </TableCell>
                        <TableCell className="text-xs sm:text-sm hidden xl:table-cell">
                          {draw.winning_numbers && draw.winning_numbers.length > 0 ? (
                            <div className="flex gap-1">
                              {draw.winning_numbers.map((num, idx) => (
                                <Badge key={idx} variant="secondary">
                                  {num}
                                </Badge>
                              ))}
                            </div>
                          ) : (
                            <span className="text-muted-foreground">-</span>
                          )}
                        </TableCell>
                        <TableCell className="text-xs sm:text-sm hidden xl:table-cell font-mono text-muted-foreground">
                          {draw.id}
                        </TableCell>
                        <TableCell className="text-xs sm:text-sm hidden xl:table-cell font-mono text-muted-foreground">
                          {draw.game_schedule_id || '-'}
                        </TableCell>
                      </TableRow>
                    )
                  })
                )}
              </TableBody>
            </Table>
          </div>

          {totalPages > 1 && (
            <Pagination>
              <PaginationContent>
                <PaginationItem>
                  <PaginationPrevious
                    onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}
                    className={
                      currentPage === 1 ? 'pointer-events-none opacity-50' : 'cursor-pointer'
                    }
                  />
                </PaginationItem>

                {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                  const pageNum = i + 1
                  // Show first page, last page, current page, and pages around current
                  if (
                    pageNum === 1 ||
                    pageNum === totalPages ||
                    (pageNum >= currentPage - 1 && pageNum <= currentPage + 1)
                  ) {
                    return (
                      <PaginationItem key={pageNum}>
                        <PaginationLink
                          onClick={() => setCurrentPage(pageNum)}
                          isActive={pageNum === currentPage}
                          className="cursor-pointer"
                        >
                          {pageNum}
                        </PaginationLink>
                      </PaginationItem>
                    )
                  }
                  if (pageNum === currentPage - 2 || pageNum === currentPage + 2) {
                    return (
                      <PaginationItem key={pageNum}>
                        <PaginationEllipsis />
                      </PaginationItem>
                    )
                  }
                  return null
                })}

                <PaginationItem>
                  <PaginationNext
                    onClick={() => setCurrentPage(Math.min(totalPages, currentPage + 1))}
                    className={
                      currentPage === totalPages
                        ? 'pointer-events-none opacity-50'
                        : 'cursor-pointer'
                    }
                  />
                </PaginationItem>
              </PaginationContent>
            </Pagination>
          )}
        </CardContent>
      </Card>

    </div>
  )
}
