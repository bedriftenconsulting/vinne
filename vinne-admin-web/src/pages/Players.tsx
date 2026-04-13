import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
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
import { playerService } from '@/services/players'
import { Search, Eye } from 'lucide-react'

export default function Players() {
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')

  const { data: playersData } = useQuery({
    queryKey: ['players', page, search, statusFilter],
    queryFn: () =>
      playerService.searchPlayers({
        query: search,
        page,
        per_page: 20,
        status:
          statusFilter === 'all'
            ? undefined
            : (statusFilter.toUpperCase() as 'ACTIVE' | 'SUSPENDED' | 'BANNED'),
      }),
    placeholderData: {
      players: [],
      total: 0,
      page: 1,
      per_page: 20,
    },
    retry: 0,
    refetchOnWindowFocus: false,
  })

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
    <div className="p-3 sm:p-4 md:p-6 space-y-3 sm:space-y-4 md:space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <h1 className="text-2xl sm:text-3xl font-bold">Players</h1>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-3 sm:gap-4 grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 p-3 sm:p-6 sm:pb-2">
            <h3 className="text-xs sm:text-sm font-medium">Total Players</h3>
          </CardHeader>
          <CardContent className="p-3 sm:p-6 pt-0">
            <div className="text-lg sm:text-2xl font-bold">{playersData?.total || 0}</div>
            <p className="text-xs text-muted-foreground">Registered</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 p-3 sm:p-6 sm:pb-2">
            <h3 className="text-xs sm:text-sm font-medium">Active Players</h3>
          </CardHeader>
          <CardContent className="p-3 sm:p-6 pt-0">
            <div className="text-lg sm:text-2xl font-bold">
              {Array.isArray(playersData?.players)
                ? playersData.players.filter(p => p.status === 'ACTIVE').length
                : 0}
            </div>
            <p className="text-xs text-muted-foreground">Active</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 p-3 sm:p-6 sm:pb-2">
            <h3 className="text-xs sm:text-sm font-medium">Suspended</h3>
          </CardHeader>
          <CardContent className="p-3 sm:p-6 pt-0">
            <div className="text-lg sm:text-2xl font-bold">
              {Array.isArray(playersData?.players)
                ? playersData.players.filter(p => p.status === 'SUSPENDED').length
                : 0}
            </div>
            <p className="text-xs text-muted-foreground">Suspended</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 p-3 sm:p-6 sm:pb-2">
            <h3 className="text-xs sm:text-sm font-medium">Banned</h3>
          </CardHeader>
          <CardContent className="p-3 sm:p-6 pt-0">
            <div className="text-lg sm:text-2xl font-bold">
              {Array.isArray(playersData?.players)
                ? playersData.players.filter(p => p.status === 'BANNED').length
                : 0}
            </div>
            <p className="text-xs text-muted-foreground">Banned</p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="p-3 sm:p-6">
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2">
            <div className="flex items-center space-x-2 flex-1">
              <Search className="h-4 w-4 shrink-0" />
              <Input
                placeholder="Search by phone, email, or name..."
                value={search}
                onChange={e => setSearch(e.target.value)}
                className="flex-1 sm:max-w-md"
              />
            </div>
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="w-full sm:w-48">
                <SelectValue placeholder="Filter by status" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All statuses</SelectItem>
                <SelectItem value="active">Active</SelectItem>
                <SelectItem value="suspended">Suspended</SelectItem>
                <SelectItem value="banned">Banned</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent className="p-0 sm:p-6 sm:pt-0">
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-xs sm:text-sm">Phone Number</TableHead>
                  <TableHead className="text-xs sm:text-sm">Name</TableHead>
                  <TableHead className="text-xs sm:text-sm hidden md:table-cell">Email</TableHead>
                  <TableHead className="text-xs sm:text-sm">Status</TableHead>
                  <TableHead className="text-xs sm:text-sm hidden sm:table-cell">
                    Registered
                  </TableHead>
                  <TableHead className="text-xs sm:text-sm hidden lg:table-cell">
                    Last Login
                  </TableHead>
                  <TableHead className="text-xs sm:text-sm">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {Array.isArray(playersData?.players) && playersData.players.length > 0 ? (
                  playersData.players.map(player => (
                    <TableRow key={player.id}>
                      <TableCell className="font-medium text-xs sm:text-sm py-2 sm:py-4">
                        {player.phone_number}
                      </TableCell>
                      <TableCell className="py-2 sm:py-4">
                        <Link
                          to="/admin/player/$playerId"
                          params={{ playerId: player.id }}
                          className="text-blue-600 hover:text-blue-800 hover:underline font-medium text-xs sm:text-sm truncate max-w-32 sm:max-w-none inline-block"
                        >
                          {player.first_name && player.last_name
                            ? `${player.first_name} ${player.last_name}`
                            : player.first_name || player.last_name || 'N/A'}
                        </Link>
                      </TableCell>
                      <TableCell className="text-xs sm:text-sm hidden md:table-cell py-2 sm:py-4">
                        {player.email || 'N/A'}
                      </TableCell>
                      <TableCell className="py-2 sm:py-4">
                        <Badge variant={getStatusBadgeVariant(player.status)} className="text-xs">
                          {getStatusLabel(player.status)}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs sm:text-sm hidden sm:table-cell py-2 sm:py-4">
                        {new Date(player.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell className="text-xs sm:text-sm hidden lg:table-cell py-2 sm:py-4">
                        {player.last_login
                          ? new Date(player.last_login).toLocaleDateString()
                          : 'Never'}
                      </TableCell>
                      <TableCell className="py-2 sm:py-4">
                        <Link to="/admin/player/$playerId" params={{ playerId: player.id }}>
                          <Button variant="outline" size="sm" className="h-7 w-7 sm:h-8 sm:w-8 p-0">
                            <Eye className="h-3 w-3 sm:h-4 sm:w-4" />
                          </Button>
                        </Link>
                      </TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                      {search || statusFilter !== 'all'
                        ? 'No players found matching your search criteria.'
                        : 'No players registered yet.'}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>

          {playersData && playersData.total > 0 && (
            <div className="flex flex-col sm:flex-row items-center justify-between gap-3 sm:gap-2 py-3 sm:py-4 px-3 sm:px-0">
              <div className="text-xs sm:text-sm text-muted-foreground text-center sm:text-left">
                <span className="hidden sm:inline">
                  Showing {(page - 1) * (playersData.per_page || 20) + 1} to{' '}
                </span>
                <span className="hidden sm:inline">
                  {Math.min(page * (playersData.per_page || 20), playersData.total)} of{' '}
                </span>
                <span className="sm:hidden">
                  Page {page} of {Math.ceil(playersData.total / (playersData.per_page || 20))} (
                  {playersData.total} total)
                </span>
                <span className="hidden sm:inline">{playersData.total} players</span>
              </div>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage(p => Math.max(1, p - 1))}
                  disabled={page <= 1}
                  className="text-xs sm:text-sm h-8 sm:h-9"
                >
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage(p => p + 1)}
                  disabled={page >= Math.ceil(playersData.total / (playersData.per_page || 20))}
                  className="text-xs sm:text-sm h-8 sm:h-9"
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
