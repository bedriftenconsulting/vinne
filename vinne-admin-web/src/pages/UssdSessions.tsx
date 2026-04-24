import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Phone, Search, RefreshCw, Eye, CheckCircle, Clock, Ticket, Hash,
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger,
} from '@/components/ui/dialog'
import {
  Pagination, PaginationContent, PaginationEllipsis,
  PaginationItem, PaginationLink, PaginationNext, PaginationPrevious,
} from '@/components/ui/pagination'
import PageHeader from '@/components/ui/page-header'
import { formatInGhanaTime } from '@/lib/date-utils'
import { formatCurrency } from '@/lib/utils'
import api from '@/lib/api'

const PAGE_SIZE = 10

interface UssdTicket {
  id: string
  serial_number: string
  game_code: string
  game_name: string
  game_type: 'ACCESS_PASS' | 'DRAW_ENTRY'
  customer_phone: string
  issuer_type?: string
  issuer_id: string
  payment_method: string
  payment_ref: string
  payment_status: string
  total_amount: number
  unit_price: number
  status: string
  draw_date: string
  created_at: string
  updated_at: string
}

async function fetchUssdTickets(): Promise<UssdTicket[]> {
  try {
    let page = 1
    let all: UssdTicket[] = []
    while (true) {
      const res = await api.get('/ussd/tickets', { params: { page, limit: 200 } })
      const batch: UssdTicket[] = res.data?.data || []
      all = [...all, ...batch]
      if (batch.length < 200 || page >= (res.data?.pages ?? 1)) break
      page++
    }
    return all
  } catch {
    return []
  }
}

const statusBadge = (status: string) => {
  if (status === 'completed')
    return <Badge className="bg-green-100 text-green-800 border-green-200"><CheckCircle className="h-3 w-3 mr-1" />Completed</Badge>
  if (status === 'pending')
    return <Badge variant="secondary"><Clock className="h-3 w-3 mr-1" />Pending</Badge>
  return <Badge variant="outline">{status}</Badge>
}

const typeBadge = (t: UssdTicket) => {
  const isAccess = t.game_type === 'ACCESS_PASS' || t.serial_number?.startsWith('CP-ACC-')
  if (isAccess)
    return <Badge variant="default"><Ticket className="h-3 w-3 mr-1" />Access Pass</Badge>
  return <Badge variant="secondary"><Hash className="h-3 w-3 mr-1" />Draw Entry</Badge>
}

function buildPageNumbers(current: number, total: number): (number | 'ellipsis')[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  if (current <= 4) return [1, 2, 3, 4, 5, 'ellipsis', total]
  if (current >= total - 3) return [1, 'ellipsis', total - 4, total - 3, total - 2, total - 1, total]
  return [1, 'ellipsis', current - 1, current, current + 1, 'ellipsis', total]
}

export default function UssdSessions() {
  const [search, setSearch] = useState('')
  const [gameTypeFilter, setGameTypeFilter] = useState('')
  const [paymentStatusFilter, setPaymentStatusFilter] = useState('completed')
  const [page, setPage] = useState(1)

  const { data: allTickets = [], isLoading, refetch, isFetching } = useQuery({
    queryKey: ['ussd-tickets'],
    queryFn: fetchUssdTickets,
    refetchInterval: 30000,
  })

  // Apply filters
  const filtered = useMemo(() => {
    let result = allTickets
    if (gameTypeFilter === 'ACCESS_PASS') result = result.filter(t => t.game_type === 'ACCESS_PASS' || t.serial_number?.startsWith('CP-ACC-'))
    if (gameTypeFilter === 'DRAW_ENTRY') result = result.filter(t => t.game_type === 'DRAW_ENTRY' || t.serial_number?.startsWith('WB-ENT-'))
    if (paymentStatusFilter) result = result.filter(t => t.payment_status === paymentStatusFilter)
    if (search) {
      const s = search.toLowerCase()
      result = result.filter(
        t =>
          t.serial_number?.toLowerCase().includes(s) ||
          t.customer_phone?.includes(s) ||
          t.payment_ref?.toLowerCase().includes(s),
      )
    }
    return result
  }, [allTickets, gameTypeFilter, paymentStatusFilter, search])

  // Reset to page 1 when filters change
  const handleFilter = (setter: (v: string) => void) => (v: string) => {
    setter(v)
    setPage(1)
  }

  // Pagination
  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const safePage = Math.min(page, totalPages)
  const paginated = filtered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE)
  const pageNumbers = buildPageNumbers(safePage, totalPages)

  // Stats (always from full unfiltered set)
  // game_type may not come from API — use serial prefix as fallback
  const isAccessPass = (t: UssdTicket) =>
    t.game_type === 'ACCESS_PASS' || t.serial_number?.startsWith('CP-ACC-')
  const isDrawEntry = (t: UssdTicket) =>
    t.game_type === 'DRAW_ENTRY' || t.serial_number?.startsWith('WB-ENT-')

  const uniquePhones = new Set(allTickets.map(t => t.customer_phone)).size
  const accessPasses = allTickets.filter(isAccessPass)
  const drawEntries = allTickets.filter(isDrawEntry)
  const completed = allTickets.filter(t => t.payment_status === 'completed')
  // total_amount is in pesewas — divide by 100 for GHS
  const totalRevenue = allTickets.filter(t => t.payment_status === 'completed').reduce((sum, t) => sum + Number(t.unit_price || 0), 0)

  return (
    <div className="space-y-6">
      <PageHeader
        title="USSD Sessions"
        description="Monitor all ticket purchases made via *899*92 USSD channel"
        badge="Live"
      >
        <Button variant="outline" size="sm" onClick={() => refetch()} disabled={isFetching}>
          <RefreshCw className={`h-4 w-4 mr-2 ${isFetching ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </PageHeader>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-5">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Unique Users</CardTitle>
            <Phone className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{uniquePhones}</div>
            <p className="text-xs text-muted-foreground">distinct phone numbers</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Access Passes</CardTitle>
            <Ticket className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{accessPasses.length}</div>
            <p className="text-xs text-muted-foreground">tickets sold</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Draw Entries</CardTitle>
            <Hash className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{drawEntries.length}</div>
            <p className="text-xs text-muted-foreground">total draw entries issued</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Completed</CardTitle>
            <CheckCircle className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">{completed.length}</div>
            <p className="text-xs text-muted-foreground">of {allTickets.length} total tickets</p>
          </CardContent>
        </Card>
        <Card className="border-green-200 bg-green-50 dark:bg-green-950/20 dark:border-green-900">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-green-700 dark:text-green-400">USSD Revenue</CardTitle>
            <span className="text-green-600 dark:text-green-400 font-bold text-sm">₵</span>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-700 dark:text-green-400">
              {formatCurrency(totalRevenue)}
            </div>
            <p className="text-xs text-green-600 dark:text-green-500">from {accessPasses.length} access passes</p>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="pt-4">
          <div className="flex flex-wrap gap-4 items-end">
            <div className="flex-1 min-w-[200px]">
              <Label>Search</Label>
              <div className="relative mt-1">
                <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Phone, serial, reference..."
                  value={search}
                  onChange={e => { setSearch(e.target.value); setPage(1) }}
                  className="pl-8"
                />
              </div>
            </div>
            <div>
              <Label>Type</Label>
              <div className="flex gap-2 mt-1">
                {(['', 'ACCESS_PASS', 'DRAW_ENTRY'] as const).map(v => (
                  <Button key={v} size="sm" variant={gameTypeFilter === v ? 'default' : 'outline'}
                    onClick={() => handleFilter(setGameTypeFilter)(v)}>
                    {v === '' ? 'All' : v === 'ACCESS_PASS' ? 'Access Pass' : 'Draw Entry'}
                  </Button>
                ))}
              </div>
            </div>
            <div>
              <Label>Payment</Label>
              <div className="flex gap-2 mt-1">
                {(['', 'completed', 'pending'] as const).map(v => (
                  <Button key={v} size="sm" variant={paymentStatusFilter === v ? 'default' : 'outline'}
                    onClick={() => handleFilter(setPaymentStatusFilter)(v)}>
                    {v === '' ? 'All' : v.charAt(0).toUpperCase() + v.slice(1)}
                  </Button>
                ))}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Table */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>USSD Tickets ({filtered.length})</CardTitle>
          <span className="text-sm text-muted-foreground">
            Page {safePage} of {totalPages} · showing {paginated.length} of {filtered.length}
          </span>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="flex items-center justify-center h-40">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
            </div>
          ) : filtered.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
              <Phone className="h-10 w-10" />
              <p className="text-lg font-medium">No USSD tickets yet</p>
              <p className="text-sm">Tickets will appear here when users dial *899*92</p>
            </div>
          ) : (
            <>
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Serial</TableHead>
                      <TableHead>Phone</TableHead>
                      <TableHead>Type</TableHead>
                      <TableHead>Game</TableHead>
                      <TableHead>Amount</TableHead>
                      <TableHead>Payment</TableHead>
                      <TableHead>Reference</TableHead>
                      <TableHead>Date</TableHead>
                      <TableHead></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {paginated.map(ticket => (
                      <TableRow key={ticket.id || ticket.serial_number}>
                        <TableCell className="font-mono text-xs font-medium">{ticket.serial_number}</TableCell>
                        <TableCell className="font-mono text-sm">{ticket.customer_phone}</TableCell>
                        <TableCell>{typeBadge(ticket)}</TableCell>
                        <TableCell className="text-sm max-w-[160px] truncate">{ticket.game_name}</TableCell>
                        <TableCell className="font-semibold">
                          {ticket.unit_price > 0 ? formatCurrency(Number(ticket.unit_price)) : '—'}
                        </TableCell>
                        <TableCell>{statusBadge(ticket.payment_status)}</TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground max-w-[140px] truncate">
                          {ticket.payment_ref}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {formatInGhanaTime(ticket.created_at, 'PP p')}
                        </TableCell>
                        <TableCell>
                          <Dialog>
                            <DialogTrigger asChild>
                              <Button variant="ghost" size="sm"><Eye className="h-4 w-4" /></Button>
                            </DialogTrigger>
                            <DialogContent>
                              <DialogHeader>
                                <DialogTitle>Ticket — {ticket.serial_number}</DialogTitle>
                              </DialogHeader>
                              <div className="grid grid-cols-2 gap-4 text-sm">
                                <div><Label>Serial</Label><p className="font-mono">{ticket.serial_number}</p></div>
                                <div><Label>Phone</Label><p className="font-mono">{ticket.customer_phone}</p></div>
                                <div><Label>Type</Label><div className="mt-1">{typeBadge(ticket)}</div></div>
                                <div><Label>Payment Status</Label><div className="mt-1">{statusBadge(ticket.payment_status)}</div></div>
                                <div><Label>Game</Label><p>{ticket.game_name}</p></div>
                                <div><Label>Game Code</Label><p className="font-mono">{ticket.game_code}</p></div>
                                <div><Label>Amount</Label><p className="font-bold text-base">{formatCurrency(Number(ticket.unit_price))}</p></div>
                                <div><Label>Payment Method</Label><p className="capitalize">{ticket.payment_method?.replace('_', ' ')}</p></div>
                                <div className="col-span-2"><Label>Payment Reference</Label><p className="font-mono text-xs break-all">{ticket.payment_ref}</p></div>
                                <div><Label>Draw Date</Label><p>{ticket.draw_date}</p></div>
                                <div><Label>Ticket Status</Label><Badge variant="outline">{ticket.status}</Badge></div>
                                <div><Label>Created</Label><p>{formatInGhanaTime(ticket.created_at, 'PPp')}</p></div>
                                <div><Label>Updated</Label><p>{formatInGhanaTime(ticket.updated_at, 'PPp')}</p></div>
                              </div>
                            </DialogContent>
                          </Dialog>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              {/* Pagination */}
              {totalPages > 1 && (
                <div className="mt-4">
                  <Pagination>
                    <PaginationContent>
                      <PaginationItem>
                        <PaginationPrevious
                          href="#"
                          onClick={e => { e.preventDefault(); setPage(p => Math.max(1, p - 1)) }}
                          aria-disabled={safePage === 1}
                          className={safePage === 1 ? 'pointer-events-none opacity-50' : ''}
                        />
                      </PaginationItem>

                      {pageNumbers.map((n, i) =>
                        n === 'ellipsis' ? (
                          <PaginationItem key={`ellipsis-${i}`}>
                            <PaginationEllipsis />
                          </PaginationItem>
                        ) : (
                          <PaginationItem key={n}>
                            <PaginationLink
                              href="#"
                              isActive={n === safePage}
                              onClick={e => { e.preventDefault(); setPage(n) }}
                            >
                              {n}
                            </PaginationLink>
                          </PaginationItem>
                        )
                      )}

                      <PaginationItem>
                        <PaginationNext
                          href="#"
                          onClick={e => { e.preventDefault(); setPage(p => Math.min(totalPages, p + 1)) }}
                          aria-disabled={safePage === totalPages}
                          className={safePage === totalPages ? 'pointer-events-none opacity-50' : ''}
                        />
                      </PaginationItem>
                    </PaginationContent>
                  </Pagination>
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
