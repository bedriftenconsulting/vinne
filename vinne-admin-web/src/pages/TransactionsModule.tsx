import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  CreditCard, Search, Eye, RefreshCw,
  CheckCircle, TrendingUp, Users, Hash,
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
import { formatCurrency } from '@/lib/utils'
import { formatInGhanaTime } from '@/lib/date-utils'
import PageHeader from '@/components/ui/page-header'
import api from '@/lib/api'

// ── Types ──────────────────────────────────────────────────────────────────────
interface UssdTicket {
  serial_number: string
  game_type: 'ACCESS_PASS' | 'DRAW_ENTRY'
  game_name: string
  customer_phone: string
  unit_price: number
  total_amount: number
  payment_status: string
  payment_ref: string
  payment_reference: string   // Hubtel MoMo TX ID
  payment_method: string
  created_at: string
  paid_at: string | null
}

interface Transaction {
  payment_ref: string
  momo_tx_id: string          // Hubtel payment_reference
  type: string                // "1-Day Pass" | "2-Day Pass" | "Extra WinBig (N)"
  phone: string               // MSISDN
  amount: number              // pesewas — SUM(unit_price)
  gateway: string             // "MTN MoMo" | "Telecel Cash" | "AirtelTigo Money"
  date: string
  tickets: UssdTicket[]
}

// ── Helpers ────────────────────────────────────────────────────────────────────
function detectGateway(phone: string): string {
  const p = phone.replace(/^\+233/, '0').replace(/^233/, '0')
  const prefix = p.slice(0, 3)
  if (['024', '054', '055', '059', '025'].includes(prefix)) return 'MTN MoMo'
  if (['020', '050'].includes(prefix))                           return 'Telecel Cash'
  if (['026', '056', '027', '057', '028', '058'].includes(prefix)) return 'AirtelTigo Money'
  return 'Hubtel MoMo'
}

function deriveType(tickets: UssdTicket[]): string {
  const accessCount = tickets.filter(t => t.game_type === 'ACCESS_PASS').length
  const entryCount  = tickets.filter(t => t.game_type === 'DRAW_ENTRY').length
  if (accessCount === 1) return '1-Day Pass'
  if (accessCount === 2) return '2-Day Pass'
  if (entryCount > 0)   return `Extra WinBig (${entryCount})`
  return 'Ticket Purchase'
}

function buildPageNumbers(current: number, total: number): (number | 'ellipsis')[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  if (current <= 4) return [1, 2, 3, 4, 5, 'ellipsis', total]
  if (current >= total - 3) return [1, 'ellipsis', total - 4, total - 3, total - 2, total - 1, total]
  return [1, 'ellipsis', current - 1, current, current + 1, 'ellipsis', total]
}

// ── Data fetching ──────────────────────────────────────────────────────────────
async function fetchCompletedTransactions(): Promise<Transaction[]> {
  try {
    let page = 1
    let all: UssdTicket[] = []
    while (true) {
      let batch: UssdTicket[] = []
      try {
        const res = await api.get('/admin/tickets', { params: { page: String(page) } })
        batch = res.data?.data?.tickets || res.data?.tickets || []
      } catch { break }
      // Filter USSD completed tickets only
      const ussdBatch = batch.filter(
        t => (t.issuer_type === 'USSD' ||
          t.serial_number?.startsWith('WB-ACC-') ||
          t.serial_number?.startsWith('WB-ENT-') ||
          t.serial_number?.startsWith('CP-ACC-') ||
          t.serial_number?.startsWith('CP-ENT-')) &&
          t.payment_status === 'completed'
      )
      all = [...all, ...ussdBatch]
      if (batch.length < 10) break
      page++
      if (page > 30) break
    }

    // Group by payment_ref
    const byRef = new Map<string, UssdTicket[]>()
    for (const t of all) {
      const ref = t.payment_ref
      if (!byRef.has(ref)) byRef.set(ref, [])
      byRef.get(ref)!.push(t)
    }

    // Build one Transaction per payment_ref
    const txns: Transaction[] = []
    for (const [ref, tickets] of byRef) {
      const rep = tickets[0]
      txns.push({
        payment_ref:  ref,
        momo_tx_id:   rep.payment_reference || '',
        type:         deriveType(tickets),
        phone:        rep.customer_phone,
        amount:       tickets.reduce((s, t) => s + Number(t.unit_price || 0), 0),
        gateway:      detectGateway(rep.customer_phone),
        date:         rep.paid_at || rep.created_at,
        tickets,
      })
    }

    // Sort newest first
    return txns.sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime())
  } catch {
    return []
  }
}

// ── Sub-components ─────────────────────────────────────────────────────────────
function GatewayBadge({ gateway }: { gateway: string }) {
  const colours: Record<string, string> = {
    'MTN MoMo':         'bg-yellow-100 text-yellow-800 border-yellow-200',
    'Telecel Cash':     'bg-red-100 text-red-800 border-red-200',
    'AirtelTigo Money': 'bg-blue-100 text-blue-800 border-blue-200',
    'Hubtel MoMo':      'bg-purple-100 text-purple-800 border-purple-200',
  }
  return (
    <Badge className={colours[gateway] ?? 'bg-gray-100 text-gray-800'}>
      {gateway}
    </Badge>
  )
}

function TypeBadge({ type }: { type: string }) {
  if (type.startsWith('1-Day'))   return <Badge variant="default">{type}</Badge>
  if (type.startsWith('2-Day'))   return <Badge className="bg-indigo-100 text-indigo-800 border-indigo-200">{type}</Badge>
  if (type.startsWith('Extra'))   return <Badge variant="secondary">{type}</Badge>
  return <Badge variant="outline">{type}</Badge>
}

const PAGE_SIZE = 10

// ── Main component ─────────────────────────────────────────────────────────────
export default function TransactionsModule() {
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [page, setPage] = useState(1)

  const { data: allTxns = [], isLoading, refetch, isFetching } = useQuery({
    queryKey: ['ussd-transactions'],
    queryFn: fetchCompletedTransactions,
    refetchInterval: 30_000,
  })

  const filtered = useMemo(() => {
    let r = allTxns
    if (typeFilter === '1-day')  r = r.filter(t => t.type.startsWith('1-Day'))
    if (typeFilter === '2-day')  r = r.filter(t => t.type.startsWith('2-Day'))
    if (typeFilter === 'extra')  r = r.filter(t => t.type.startsWith('Extra'))
    if (search) {
      const s = search.toLowerCase()
      r = r.filter(t =>
        t.payment_ref.toLowerCase().includes(s) ||
        t.momo_tx_id.toLowerCase().includes(s) ||
        t.phone.includes(s)
      )
    }
    return r
  }, [allTxns, typeFilter, search])

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const safePage   = Math.min(page, totalPages)
  const paginated  = filtered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE)
  const pageNums   = buildPageNumbers(safePage, totalPages)

  // Stats — always from full unfiltered set
  const totalRevenue   = allTxns.reduce((s, t) => s + t.amount, 0)
  const uniquePlayers  = new Set(allTxns.map(t => t.phone)).size
  const passCount      = allTxns.filter(t => t.type.includes('Day')).length
  const extraCount     = allTxns.filter(t => t.type.startsWith('Extra')).length

  return (
    <div className="space-y-6">
      <PageHeader
        title="Transactions"
        description="Completed USSD ticket purchases collected via Hubtel"
        badge="Live"
      >
        <Button variant="outline" size="sm" onClick={() => refetch()} disabled={isFetching}>
          <RefreshCw className={`h-4 w-4 mr-2 ${isFetching ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </PageHeader>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card className="border-green-200 bg-green-50 dark:bg-green-950/20 dark:border-green-900">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-green-700 dark:text-green-400">Total Revenue</CardTitle>
            <span className="text-green-600 font-bold text-sm">₵</span>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-700 dark:text-green-400">{formatCurrency(totalRevenue)}</div>
            <p className="text-xs text-green-600">{allTxns.length} completed transactions</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Unique Players</CardTitle>
            <Users className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{uniquePlayers}</div>
            <p className="text-xs text-muted-foreground">distinct phone numbers</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Pass Sales</CardTitle>
            <CreditCard className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{passCount}</div>
            <p className="text-xs text-muted-foreground">1-Day &amp; 2-Day passes</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Extra WinBig</CardTitle>
            <Hash className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{extraCount}</div>
            <p className="text-xs text-muted-foreground">extra entry purchases</p>
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
                  placeholder="Phone, payment ref, MoMo TX ID..."
                  value={search}
                  onChange={e => { setSearch(e.target.value); setPage(1) }}
                  className="pl-8"
                />
              </div>
            </div>
            <div>
              <Label>Type</Label>
              <div className="flex gap-2 mt-1">
                {([
                  { val: '',      label: 'All' },
                  { val: '1-day', label: '1-Day Pass' },
                  { val: '2-day', label: '2-Day Pass' },
                  { val: 'extra', label: 'Extra WinBig' },
                ] as const).map(({ val, label }) => (
                  <Button
                    key={val}
                    size="sm"
                    variant={typeFilter === val ? 'default' : 'outline'}
                    onClick={() => { setTypeFilter(val); setPage(1) }}
                  >
                    {label}
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
          <CardTitle className="flex items-center gap-2">
            <CheckCircle className="h-5 w-5 text-green-600" />
            Completed Transactions ({filtered.length})
          </CardTitle>
          <span className="text-sm text-muted-foreground">
            Page {safePage} of {totalPages}
          </span>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="flex items-center justify-center h-40">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
            </div>
          ) : filtered.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
              <TrendingUp className="h-10 w-10" />
              <p className="text-lg font-medium">No completed transactions yet</p>
            </div>
          ) : (
            <>
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Transaction ID</TableHead>
                      <TableHead>Payment Ref (Hubtel)</TableHead>
                      <TableHead>Type</TableHead>
                      <TableHead>Player</TableHead>
                      <TableHead className="text-right">Amount</TableHead>
                      <TableHead>Gateway</TableHead>
                      <TableHead>Date</TableHead>
                      <TableHead></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {paginated.map(tx => (
                      <TableRow key={tx.payment_ref}>
                        <TableCell className="font-mono text-xs text-muted-foreground max-w-[120px] truncate" title={tx.payment_ref}>
                          {tx.payment_ref.slice(0, 12)}…
                        </TableCell>
                        <TableCell className="font-mono text-xs text-green-700 max-w-[130px] truncate" title={tx.momo_tx_id}>
                          {tx.momo_tx_id ? tx.momo_tx_id.slice(0, 14) + '…' : '—'}
                        </TableCell>
                        <TableCell><TypeBadge type={tx.type} /></TableCell>
                        <TableCell className="font-mono text-sm">{tx.phone}</TableCell>
                        <TableCell className="text-right font-semibold">{formatCurrency(tx.amount)}</TableCell>
                        <TableCell><GatewayBadge gateway={tx.gateway} /></TableCell>
                        <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                          {formatInGhanaTime(tx.date, 'PP p')}
                        </TableCell>
                        <TableCell>
                          <Dialog>
                            <DialogTrigger asChild>
                              <Button variant="ghost" size="sm"><Eye className="h-4 w-4" /></Button>
                            </DialogTrigger>
                            <DialogContent className="max-w-lg">
                              <DialogHeader>
                                <DialogTitle>Transaction Detail</DialogTitle>
                              </DialogHeader>
                              <div className="space-y-4 text-sm">
                                <div className="grid grid-cols-2 gap-3">
                                  <div>
                                    <p className="text-xs text-muted-foreground">Transaction ID</p>
                                    <p className="font-mono text-xs break-all">{tx.payment_ref}</p>
                                  </div>
                                  <div>
                                    <p className="text-xs text-muted-foreground">Hubtel MoMo Ref</p>
                                    <p className="font-mono text-xs break-all text-green-700">{tx.momo_tx_id || '—'}</p>
                                  </div>
                                  <div>
                                    <p className="text-xs text-muted-foreground">Type</p>
                                    <TypeBadge type={tx.type} />
                                  </div>
                                  <div>
                                    <p className="text-xs text-muted-foreground">Player (MSISDN)</p>
                                    <p className="font-mono">{tx.phone}</p>
                                  </div>
                                  <div>
                                    <p className="text-xs text-muted-foreground">Total Amount</p>
                                    <p className="font-bold text-base">{formatCurrency(tx.amount)}</p>
                                  </div>
                                  <div>
                                    <p className="text-xs text-muted-foreground">Gateway</p>
                                    <GatewayBadge gateway={tx.gateway} />
                                  </div>
                                  <div>
                                    <p className="text-xs text-muted-foreground">Date</p>
                                    <p>{formatInGhanaTime(tx.date, 'PPp')}</p>
                                  </div>
                                  <div>
                                    <p className="text-xs text-muted-foreground">Status</p>
                                    <Badge className="bg-green-100 text-green-800">
                                      <CheckCircle className="h-3 w-3 mr-1" />Completed
                                    </Badge>
                                  </div>
                                </div>
                                <div>
                                  <p className="text-xs text-muted-foreground mb-2">Tickets in this transaction</p>
                                  <div className="rounded border divide-y text-xs">
                                    {tx.tickets.map(t => (
                                      <div key={t.serial_number} className="flex items-center justify-between px-3 py-2">
                                        <span className="font-mono">{t.serial_number}</span>
                                        <span className="text-muted-foreground">
                                          {t.game_type === 'ACCESS_PASS' ? 'Access Pass' : 'Draw Entry'}
                                        </span>
                                        <span className="font-medium">{formatCurrency(t.unit_price)}</span>
                                      </div>
                                    ))}
                                  </div>
                                </div>
                              </div>
                            </DialogContent>
                          </Dialog>
                        </TableCell>
                      </TableRow>
                    ))}
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
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
