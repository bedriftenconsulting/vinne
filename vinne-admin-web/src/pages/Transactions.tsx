import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
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
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { HoverCard, HoverCardContent, HoverCardTrigger } from '@/components/ui/hover-card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
import { formatInGhanaTime } from '@/lib/date-utils'
import { format } from 'date-fns'
import { DayPicker } from 'react-day-picker'
import 'react-day-picker/dist/style.css'
import {
  Search,
  Download,
  TrendingUp,
  DollarSign,
  CreditCard,
  RefreshCw,
  Calendar,
  Wallet,
  Users,
  AlertTriangle,
  CheckCircle,
  Clock,
  XCircle,
  Plus,
  Building2,
  Trophy,
  Eye,
  Loader2,
} from 'lucide-react'
import { walletService, type WalletTransaction } from '@/services/wallet'
import { ticketService, type Ticket } from '@/services/tickets'

export default function Transactions() {
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [dateFrom, setDateFrom] = useState<Date>()
  const [dateTo, setDateTo] = useState<Date>()
  const [showCustomPicker, setShowCustomPicker] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 20

  // Build API filters
  const apiFilters = useMemo(() => {
    const filters: Record<string, unknown> = {
      page: currentPage,
      page_size: pageSize,
      sort_by: 'created_at',
      sort_order: 'desc' as const,
    }

    // Add search term
    if (search) {
      filters.search = search
    }

    // Add type filter
    if (typeFilter && typeFilter !== 'all') {
      // Map frontend filter values to API enum values
      const typeMap: Record<string, string> = {
        CREDIT: 'CREDIT',
        DEBIT: 'DEBIT',
        TRANSFER: 'TRANSFER',
        COMMISSION: 'COMMISSION',
        PAYOUT: 'PAYOUT',
      }
      if (typeMap[typeFilter]) {
        filters.transaction_types = [typeMap[typeFilter]]
      }
    }

    // Add status filter
    if (statusFilter && statusFilter !== 'all') {
      filters.statuses = [statusFilter]
    }

    // Add date range filters
    if (dateFrom) {
      filters.start_date = dateFrom.toISOString()
    }
    if (dateTo) {
      filters.end_date = dateTo.toISOString()
    }

    return filters
  }, [search, typeFilter, statusFilter, dateFrom, dateTo, currentPage, pageSize])

  // Fetch transactions using TanStack Query
  const {
    data: transactionsData,
    isLoading,
    isError,
    error,
    refetch,
  } = useQuery({
    queryKey: ['transactions', apiFilters],
    queryFn: () => walletService.getAllTransactions(apiFilters),
    staleTime: 30000, // 30 seconds
    placeholderData: {
      transactions: [],
      pagination: {
        page: 1,
        page_size: 20,
        total: 0,
        total_pages: 0,
        has_more: false,
      },
    },
    retry: 0,
    refetchOnWindowFocus: false,
  })

  const transactions = useMemo(() => {
    return transactionsData?.transactions || []
  }, [transactionsData?.transactions])

  const pagination = transactionsData?.pagination || {
    page: 1,
    page_size: 20,
    total: 0,
    total_pages: 0,
    has_more: false,
  }

  // Use backend statistics if available, fallback to calculated stats from current page
  const stats = useMemo(() => {
    // If we have backend statistics, use them (they reflect ALL transactions matching filters)
    if (transactionsData?.statistics) {
      return {
        totalVolume: transactionsData.statistics.total_volume,
        totalCredits: transactionsData.statistics.total_credits,
        totalDebits: transactionsData.statistics.total_debits,
        pendingAmount: transactionsData.statistics.pending_amount,
        pendingCount: transactionsData.statistics.pending_count,
        completedCount: transactionsData.statistics.completed_count,
        failedCount: transactionsData.statistics.failed_count,
      }
    }

    // Fallback: Calculate statistics from current page transactions
    const totalVolume = transactions.reduce((sum, tx) => sum + Math.abs(tx.amount), 0)
    const totalCredits = transactions
      .filter(tx => tx.type === 'CREDIT')
      .reduce((sum, tx) => sum + tx.amount, 0)
    const totalDebits = transactions
      .filter(tx => tx.type === 'DEBIT')
      .reduce((sum, tx) => sum + Math.abs(tx.amount), 0)
    const pendingAmount = transactions
      .filter(tx => tx.status === 'PENDING')
      .reduce((sum, tx) => sum + Math.abs(tx.amount), 0)
    const pendingCount = transactions.filter(tx => tx.status === 'PENDING').length
    const completedCount = transactions.filter(tx => tx.status === 'COMPLETED').length
    const failedCount = transactions.filter(tx => tx.status === 'FAILED').length

    return {
      totalVolume,
      totalCredits,
      totalDebits,
      pendingAmount,
      pendingCount,
      completedCount,
      failedCount,
    }
  }, [transactions, transactionsData?.statistics])

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('en-GH', {
      style: 'currency',
      currency: 'GHS',
    }).format(amount)
  }

  const formatDate = (dateString: string) => {
    return formatInGhanaTime(dateString, 'dd MMM yyyy HH:mm')
  }

  const getStatusBadgeVariant = (status: string) => {
    switch (status) {
      case 'COMPLETED':
        return 'default'
      case 'PENDING':
        return 'secondary'
      case 'FAILED':
        return 'destructive'
      case 'REVERSED':
        return 'outline'
      default:
        return 'outline'
    }
  }

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'COMPLETED':
        return <CheckCircle className="h-4 w-4 text-green-500" />
      case 'PENDING':
        return <Clock className="h-4 w-4 text-yellow-500" />
      case 'FAILED':
        return <XCircle className="h-4 w-4 text-red-500" />
      case 'REVERSED':
        return <AlertTriangle className="h-4 w-4 text-orange-500" />
      default:
        return <Clock className="h-4 w-4 text-gray-500" />
    }
  }

  const getTypeBadgeVariant = (type: string) => {
    switch (type) {
      case 'CREDIT':
        return 'default'
      case 'DEBIT':
        return 'secondary'
      case 'TRANSFER':
        return 'outline'
      case 'COMMISSION':
        return 'default'
      case 'PAYOUT':
        return 'secondary'
      default:
        return 'outline'
    }
  }

  const getTypeLabel = (type: string) => {
    switch (type) {
      case 'CREDIT':
        return 'Credit'
      case 'DEBIT':
        return 'Debit'
      case 'TRANSFER':
        return 'Transfer'
      case 'COMMISSION':
        return 'Commission'
      case 'PAYOUT':
        return 'Payout'
      default:
        return type.replace(/_/g, ' ')
    }
  }

  const getTransactionDescription = (transaction: WalletTransaction) => {
    // Ticket Sale: DEBIT from RETAILER_STAKE or PLAYER_WALLET
    if (
      transaction.type === 'DEBIT' &&
      (transaction.wallet_type === 'RETAILER_STAKE' || transaction.wallet_type === 'PLAYER_WALLET')
    ) {
      return 'Ticket Sale'
    }

    // Winnings: PAYOUT or CREDIT to RETAILER_WINNING
    if (
      transaction.type === 'PAYOUT' ||
      (transaction.type === 'CREDIT' && transaction.wallet_type === 'RETAILER_WINNING')
    ) {
      return 'Winnings'
    }

    // Top-Up: CREDIT transaction (base amount)
    if (transaction.type === 'CREDIT') {
      return 'Top-Up'
    }

    // Commission: COMMISSION transaction type
    if (transaction.type === 'COMMISSION') {
      return 'Commission'
    }

    // Default to existing type labels for other cases
    return getTypeLabel(transaction.type)
  }

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'CREDIT':
        return <Plus className="h-4 w-4 text-green-500" />
      case 'DEBIT':
        return <Building2 className="h-4 w-4 text-blue-500" />
      case 'TRANSFER':
        return <Users className="h-4 w-4 text-indigo-500" />
      case 'COMMISSION':
        return <CreditCard className="h-4 w-4 text-teal-500" />
      case 'PAYOUT':
        return <Trophy className="h-4 w-4 text-purple-500" />
      default:
        return <DollarSign className="h-4 w-4" />
    }
  }

  const getWalletTypeLabel = (walletType: string) => {
    switch (walletType) {
      case 'AGENT_STAKE':
        return 'Agent Stake'
      case 'RETAILER_STAKE':
        return 'Retailer Stake'
      case 'RETAILER_WINNING':
        return 'Retailer Winning'
      default:
        return walletType
    }
  }

  const getOwnerType = (walletType: string): 'Agent' | 'Retailer' | null => {
    if (walletType === 'AGENT_STAKE') {
      return 'Agent'
    } else if (walletType === 'RETAILER_STAKE' || walletType === 'RETAILER_WINNING') {
      return 'Retailer'
    }
    return null
  }

  const toNumberArray = (value: unknown): number[] => {
    if (Array.isArray(value)) {
      return value
        .map(item => Number(item))
        .filter(item => Number.isFinite(item) && item >= 1 && item <= 90)
    }

    if (typeof value === 'string') {
      const matches = value.match(/\d+/g) || []
      return matches
        .map(part => Number(part))
        .filter(item => Number.isFinite(item) && item >= 1 && item <= 90)
    }

    return []
  }

  const toRecord = (value: unknown): Record<string, unknown> | null => {
    if (!value) {
      return null
    }

    if (typeof value === 'string') {
      try {
        const parsed = JSON.parse(value) as unknown
        if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
          return parsed as Record<string, unknown>
        }
      } catch {
        return null
      }
      return null
    }

    if (typeof value === 'object' && !Array.isArray(value)) {
      return value as Record<string, unknown>
    }

    return null
  }

  type StakeNumberItem = {
    value: number
    role: 'selected' | 'banker' | 'opposed'
  }

  const extractTicketSerial = (value?: string) => {
    if (!value) {
      return null
    }
    // Ticket serials are typically numeric (e.g. TKT-10767484), but some environments
    // may use UUID-like or mixed formats. We match a broad "TKT-..." token.
    const match = value.match(/TKT-[A-Z0-9-]+/i)
    return match?.[0]?.toUpperCase() || null
  }

  const getStakeNumberItemsFromMetadata = (
    metadata?: Record<string, unknown> | string
  ): StakeNumberItem[] => {
    const normalizedMetadata = toRecord(metadata)
    if (!normalizedMetadata) {
      return []
    }

    const candidateKeys = [
      'numbers',
      'selected_numbers',
      'stake_numbers',
      'ticket_numbers',
      'bet_numbers',
      'selections',
      'selection',
      'draw_numbers',
    ]

    for (const key of candidateKeys) {
      const parsed = toNumberArray(normalizedMetadata[key])
      if (parsed.length > 0) {
        return parsed.map(value => ({ value, role: 'selected' as const }))
      }
    }

    const ticketData = toRecord(normalizedMetadata.ticket)
    if (ticketData) {
      const bankerNumbers = toNumberArray(ticketData.banker_numbers)
      const opposedNumbers = toNumberArray(ticketData.opposed_numbers)
      const betLines = ticketData.bet_lines as Array<Record<string, unknown>> | undefined
      const firstBetType = String(betLines?.[0]?.bet_type || '').toUpperCase()
      const isBankerAgainst =
        firstBetType.includes('BANKER-AGAINST') || firstBetType.includes('BANKER_AGAINST')

      if (isBankerAgainst && (bankerNumbers.length > 0 || opposedNumbers.length > 0)) {
        return [
          ...bankerNumbers.map(value => ({ value, role: 'banker' as const })),
          ...opposedNumbers.map(value => ({ value, role: 'opposed' as const })),
        ]
      }

      for (const key of candidateKeys) {
        const parsed = toNumberArray(ticketData[key])
        if (parsed.length > 0) {
          return parsed.map(value => ({ value, role: 'selected' as const }))
        }
      }

      // Fall back to collecting numbers from individual bet lines
      if (Array.isArray(betLines) && betLines.length > 0) {
        const allSelected: StakeNumberItem[] = []
        for (const line of betLines) {
          const lineBetType = String(line.bet_type || '').toUpperCase()
          const isLineBankerAgainst =
            lineBetType.includes('BANKER-AGAINST') || lineBetType.includes('BANKER_AGAINST')
          if (isLineBankerAgainst) {
            const lb = toNumberArray(line.banker)
            const lo = toNumberArray(line.opposed)
            allSelected.push(
              ...lb.map(value => ({ value, role: 'banker' as const })),
              ...lo.map(value => ({ value, role: 'opposed' as const })),
            )
          } else {
            for (const key of candidateKeys) {
              const parsed = toNumberArray(line[key])
              if (parsed.length > 0) {
                allSelected.push(...parsed.map(value => ({ value, role: 'selected' as const })))
                break
              }
            }
            const lb = toNumberArray(line.banker)
            const lo = toNumberArray(line.opposed)
            allSelected.push(
              ...lb.map(value => ({ value, role: 'banker' as const })),
              ...lo.map(value => ({ value, role: 'opposed' as const })),
            )
          }
        }
        if (allSelected.length > 0) {
          return allSelected
        }
      }
    }

    return []
  }

  const formatBetType = (value: string) => {
    return value
      .replace(/[_-]+/g, ' ')
      .toLowerCase()
      .replace(/\b\w/g, char => char.toUpperCase())
      .trim()
  }

  const toNumber = (value: unknown): number | null => {
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value
    }
    if (typeof value === 'string') {
      const parsed = Number(value)
      if (Number.isFinite(parsed)) {
        return parsed
      }
      const firstNumeric = value.match(/-?\d+(\.\d+)?/)
      if (firstNumeric) {
        const extracted = Number(firstNumeric[0])
        if (Number.isFinite(extracted)) {
          return extracted
        }
      }
    }
    return null
  }

  type BetDetails = {
    betType: string | null
    lines: number | null
    perLineStake: number | null
    game: string | null
    drawTime: string | null
  }

  const getBetDetailsFromMetadata = (metadata?: Record<string, unknown> | string): BetDetails => {
    const emptyResult: BetDetails = {
      betType: null,
      lines: null,
      perLineStake: null,
      game: null,
      drawTime: null,
    }
    const normalizedMetadata = toRecord(metadata)
    if (!normalizedMetadata) {
      return emptyResult
    }

    const directBetType = String(normalizedMetadata.bet_type || '').trim()
    const directLines = toNumber(normalizedMetadata.number_of_lines)
    const directUnitPrice = toNumber(normalizedMetadata.unit_price)
    const directTotalAmount = toNumber(normalizedMetadata.total_amount)

    const betLinesRaw = normalizedMetadata.bet_lines
    const betLines = betLinesRaw as Array<Record<string, unknown>> | undefined
    const lineBetType = String(betLines?.[0]?.bet_type || '').trim()
    const derivedLines = Array.isArray(betLines) && betLines.length > 0 ? betLines.length : null
    const firstLineAmount = toNumber(betLines?.[0]?.amount)
    const firstLineTotalAmount = toNumber(betLines?.[0]?.total_amount)
    const betLinesString = typeof betLinesRaw === 'string' ? betLinesRaw : ''
    const stringBetTypeMatch = betLinesString.match(/bet_type:([A-Z0-9_-]+)/i)
    const stringBetType = stringBetTypeMatch?.[1] || ''
    const stringLineCountMatches = betLinesString.match(/line_number:/g)
    const stringLines = stringLineCountMatches?.length || null
    const stringLineAmountMatch = betLinesString.match(/total_amount:([0-9.]+)/i)
    const stringLineAmount = stringLineAmountMatch ? Number(stringLineAmountMatch[1]) : null

    const ticketData = toRecord(normalizedMetadata.ticket)
    const ticketDirectBetType = String(ticketData?.bet_type || '').trim()
    const ticketBetLinesRaw = ticketData?.bet_lines
    const ticketBetLines = ticketBetLinesRaw as Array<Record<string, unknown>> | undefined
    const ticketLineBetType = String(ticketBetLines?.[0]?.bet_type || '').trim()
    const ticketLines = Array.isArray(ticketBetLines) && ticketBetLines.length > 0 ? ticketBetLines.length : null
    const ticketUnitPrice = toNumber(ticketData?.unit_price)
    const ticketTotalAmount = toNumber(ticketData?.total_amount)
    const ticketFirstLineAmount = toNumber(ticketBetLines?.[0]?.amount)
    const ticketFirstLineTotalAmount = toNumber(ticketBetLines?.[0]?.total_amount)
    const ticketBetLinesString = typeof ticketBetLinesRaw === 'string' ? ticketBetLinesRaw : ''
    const ticketStringBetTypeMatch = ticketBetLinesString.match(/bet_type:([A-Z0-9_-]+)/i)
    const ticketStringBetType = ticketStringBetTypeMatch?.[1] || ''
    const ticketStringLineCountMatches = ticketBetLinesString.match(/line_number:/g)
    const ticketStringLines = ticketStringLineCountMatches?.length || null
    const ticketStringLineAmountMatch = ticketBetLinesString.match(/total_amount:([0-9.]+)/i)
    const ticketStringLineAmount = ticketStringLineAmountMatch
      ? Number(ticketStringLineAmountMatch[1])
      : null
    const gameRaw = String(
      normalizedMetadata.game_name ||
        normalizedMetadata.game_code ||
        ticketData?.game_name ||
        ticketData?.game_code ||
        '',
    ).trim()
    const drawTimeRaw = String(
      normalizedMetadata.draw_time ||
        normalizedMetadata.draw_datetime ||
        normalizedMetadata.scheduled_time ||
        normalizedMetadata.draw_date ||
        ticketData?.draw_time ||
        ticketData?.draw_datetime ||
        ticketData?.scheduled_time ||
        ticketData?.draw_date ||
        '',
    ).trim()

    const betTypeRaw =
      directBetType ||
      lineBetType ||
      stringBetType ||
      ticketDirectBetType ||
      ticketLineBetType ||
      ticketStringBetType
    const lines = directLines || derivedLines || stringLines || ticketLines || ticketStringLines

    let perLineStake =
      directUnitPrice ||
      firstLineAmount ||
      firstLineTotalAmount ||
      stringLineAmount ||
      ticketUnitPrice ||
      ticketFirstLineAmount ||
      ticketFirstLineTotalAmount ||
      ticketStringLineAmount

    if (!perLineStake) {
      const totalAmount = directTotalAmount || ticketTotalAmount
      if (totalAmount && lines && lines > 0) {
        perLineStake = totalAmount / lines
      }
    }

    return {
      betType: betTypeRaw ? formatBetType(betTypeRaw) : null,
      lines: lines && lines > 0 ? lines : null,
      perLineStake: perLineStake ?? null,
      game: gameRaw || null,
      drawTime: drawTimeRaw || null,
    }
  }

  const getStakeNumberItems = (transaction: WalletTransaction): StakeNumberItem[] => {
    const directStakeNumbers = getStakeNumberItemsFromMetadata(transaction.metadata)
    if (directStakeNumbers.length > 0) {
      return directStakeNumbers
    }

    // For audit/debit-attempt records, resolve stake numbers from sibling transaction
    // that references the same ticket serial and carries the final ticket metadata.
    const serial =
      extractTicketSerial(transaction.description) || extractTicketSerial(transaction.reference)
    if (!serial) {
      return []
    }

    const siblingWithStake = transactions.find(candidate => {
      if (candidate.id === transaction.id) {
        return false
      }
      const candidateSerial =
        extractTicketSerial(candidate.description) || extractTicketSerial(candidate.reference)
      if (candidateSerial !== serial) {
        return false
      }
      return getStakeNumberItemsFromMetadata(candidate.metadata).length > 0
    })

    if (!siblingWithStake) {
      return []
    }

    return getStakeNumberItemsFromMetadata(siblingWithStake.metadata)
  }

  const getTransactionBetDetails = (transaction: WalletTransaction): BetDetails => {
    const ownDetails = getBetDetailsFromMetadata(transaction.metadata)
    if (
      ownDetails.betType ||
      ownDetails.lines ||
      ownDetails.perLineStake ||
      ownDetails.game ||
      ownDetails.drawTime
    ) {
      return ownDetails
    }

    const serial =
      extractTicketSerial(transaction.description) || extractTicketSerial(transaction.reference)
    if (!serial) {
      return ownDetails
    }

    const sibling = transactions.find(candidate => {
      if (candidate.id === transaction.id) {
        return false
      }
      const candidateSerial =
        extractTicketSerial(candidate.description) || extractTicketSerial(candidate.reference)
      if (candidateSerial !== serial) {
        return false
      }
      const details = getBetDetailsFromMetadata(candidate.metadata)
      return Boolean(
        details.betType ||
          details.lines ||
          details.perLineStake ||
          details.game ||
          details.drawTime,
      )
    })

    if (!sibling) {
      return ownDetails
    }

    return getBetDetailsFromMetadata(sibling.metadata)
  }

  // Transaction preview component
  const TransactionPreview = ({ transaction }: { transaction: WalletTransaction }) => {
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <span className="font-medium text-sm">
            {transaction.transaction_id || transaction.reference}
          </span>
          <Badge variant={getStatusBadgeVariant(transaction.status)} className="text-xs">
            {transaction.status}
          </Badge>
        </div>
        <div className="text-sm space-y-1">
          <div>
            <strong>Amount:</strong> {formatCurrency(transaction.amount)}
          </div>
          <div>
            <strong>Type:</strong> {getTypeLabel(transaction.type)}
          </div>
          <div>
            <strong>Wallet:</strong> {getWalletTypeLabel(transaction.wallet_type)}
          </div>
          {transaction.wallet_owner_name && (
            <div>
              <strong>{getOwnerType(transaction.wallet_type)}:</strong>{' '}
              {transaction.wallet_owner_name}
              {transaction.wallet_owner_code && (
                <span className="text-muted-foreground ml-1">
                  ({transaction.wallet_owner_code})
                </span>
              )}
            </div>
          )}
          {transaction.description && (
            <div>
              <strong>Description:</strong> {transaction.description.substring(0, 60)}
              {transaction.description.length > 60 ? '...' : ''}
            </div>
          )}
          <div>
            <strong>Date:</strong> {formatDate(transaction.created_at)}
          </div>
        </div>
      </div>
    )
  }

  // Transaction details component
  const TransactionDetails = ({ transaction }: { transaction: WalletTransaction }) => {
    const isTicketSale =
      transaction.type === 'DEBIT' &&
      (transaction.description?.includes('Ticket purchase - TKT-') ||
        transaction.reference?.includes('TKT-'))

    const serial =
      extractTicketSerial(transaction.description) || extractTicketSerial(transaction.reference)

    const stakeNumbers = getStakeNumberItems(transaction)
    const betDetails = getTransactionBetDetails(transaction)

    const hasAnyBetDetail = Boolean(
      betDetails.betType ||
        betDetails.lines ||
        betDetails.perLineStake ||
        betDetails.game ||
        betDetails.drawTime,
    )

    // Some wallet debit records (notably player wallet debits) may not carry ticket metadata.
    // When that happens, fetch the ticket by serial and derive the staked numbers/details from it.
    const shouldFetchTicket =
      Boolean(isTicketSale && serial) && (stakeNumbers.length === 0 || !hasAnyBetDetail)

    const { data: fetchedTicket, isLoading: isTicketLoading } = useQuery({
      queryKey: ['admin-ticket-by-serial', serial],
      queryFn: async () => {
        if (!serial) {
          return null
        }
        return ticketService.getTicketBySerialNumber(serial)
      },
      enabled: shouldFetchTicket,
      staleTime: 30_000,
    })

    const ticketStakeNumbers = fetchedTicket
      ? getStakeNumberItemsFromMetadata({ ticket: fetchedTicket as unknown as Ticket })
      : []

    const resolvedStakeNumbers = stakeNumbers.length > 0 ? stakeNumbers : ticketStakeNumbers

    const ticketBetDetails = fetchedTicket
      ? getBetDetailsFromMetadata({
          ticket: fetchedTicket as unknown as Ticket,
          game_name: (fetchedTicket as unknown as Ticket).game_name,
          draw_datetime: (fetchedTicket as unknown as Ticket).draw_date,
        })
      : null

    const resolvedBetDetails =
      hasAnyBetDetail || !ticketBetDetails
        ? betDetails
        : {
            betType: ticketBetDetails.betType,
            lines: ticketBetDetails.lines,
            perLineStake: ticketBetDetails.perLineStake,
            game: ticketBetDetails.game,
            drawTime: ticketBetDetails.drawTime,
          }

    const formatDrawTime = (value: string) => {
      if (!value) {
        return 'Not available'
      }
      const hasDatePart = /\d{4}-\d{2}-\d{2}/.test(value) || value.includes('T')
      return hasDatePart ? formatDate(value) : value
    }

    return (
      <div className="space-y-4 sm:space-y-6">
        {/* Transaction Header */}
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 pb-4 border-b">
          <div className="min-w-0 flex-1">
            <h3 className="font-semibold text-base sm:text-lg break-all">
              {transaction.transaction_id || transaction.reference}
            </h3>
            <p className="text-muted-foreground text-xs sm:text-sm">
              {getTypeLabel(transaction.type)}
            </p>
          </div>
          <div className="text-left sm:text-right">
            <div className="text-xl sm:text-2xl font-bold">
              {formatCurrency(transaction.amount)}
            </div>
            <Badge variant={getStatusBadgeVariant(transaction.status)} className="mt-1 text-xs">
              {transaction.status}
            </Badge>
          </div>
        </div>

        {/* Transaction Details Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 sm:gap-6">
          <div className="space-y-4">
            <div>
              <h4 className="font-medium text-xs sm:text-sm text-muted-foreground mb-2">
                BASIC INFORMATION
              </h4>
              <div className="space-y-2">
                <div className="flex justify-between gap-2 text-xs sm:text-sm">
                  <span>Transaction ID:</span>
                  <span className="font-mono text-xs break-all text-right">{transaction.id}</span>
                </div>
                {transaction.transaction_id && (
                  <div className="flex justify-between gap-2 text-xs sm:text-sm">
                    <span>Reference:</span>
                    <span className="font-mono text-xs break-all text-right">
                      {transaction.transaction_id}
                    </span>
                  </div>
                )}
                <div className="flex justify-between gap-2 text-xs sm:text-sm">
                  <span>Type:</span>
                  <span>{getTypeLabel(transaction.type)}</span>
                </div>
                <div className="flex justify-between gap-2 text-xs sm:text-sm">
                  <span>Wallet Type:</span>
                  <span className="text-right">{getWalletTypeLabel(transaction.wallet_type)}</span>
                </div>
                <div className="flex justify-between gap-2 text-xs sm:text-sm">
                  <span>Amount:</span>
                  <span className="font-medium">{formatCurrency(transaction.amount)}</span>
                </div>
                <div className="flex justify-between gap-2 text-xs sm:text-sm">
                  <span>Balance Before:</span>
                  <span className="font-medium">{formatCurrency(transaction.balance_before)}</span>
                </div>
                <div className="flex justify-between gap-2 text-xs sm:text-sm">
                  <span>Balance After:</span>
                  <span className="font-medium">{formatCurrency(transaction.balance_after)}</span>
                </div>
                <div className="flex justify-between gap-2 text-xs sm:text-sm">
                  <span>Status:</span>
                  <Badge variant={getStatusBadgeVariant(transaction.status)} className="text-xs">
                    {transaction.status}
                  </Badge>
                </div>
                <div className="flex justify-between gap-2 text-xs sm:text-sm">
                  <span>Created At:</span>
                  <span className="text-right">{formatDate(transaction.created_at)}</span>
                </div>
                {transaction.completed_at && (
                  <div className="flex justify-between gap-2 text-xs sm:text-sm">
                    <span>Completed At:</span>
                    <span className="text-right">{formatDate(transaction.completed_at)}</span>
                  </div>
                )}
              </div>
            </div>
          </div>

          <div className="space-y-4">
            <div>
              <h4 className="font-medium text-xs sm:text-sm text-muted-foreground mb-2">
                STAKED NUMBERS
              </h4>
              <div className="rounded-md border bg-muted/20 p-3 sm:p-4">
                {resolvedStakeNumbers.length > 0 ? (
                  <div className="flex flex-wrap gap-2">
                    {resolvedStakeNumbers.map((item, index) => (
                      <Badge
                        key={`${transaction.id}-stake-${item.value}-${index}-${item.role}`}
                        variant={item.role === 'opposed' ? 'destructive' : 'secondary'}
                        className={cn(
                          'font-mono',
                          item.role === 'banker' &&
                            'border border-amber-400 bg-amber-100 text-amber-900 hover:bg-amber-100',
                        )}
                      >
                        {item.value}
                      </Badge>
                    ))}
                  </div>
                ) : isTicketSale && isTicketLoading ? (
                  <div className="flex items-center gap-2 text-xs sm:text-sm text-muted-foreground">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span>Loading stake numbers...</span>
                  </div>
                ) : (
                  <p className="text-xs sm:text-sm text-muted-foreground">
                    Stake numbers are not available for this transaction.
                  </p>
                )}
                {resolvedStakeNumbers.some(item => item.role === 'opposed') && (
                  <p className="mt-2 text-xs text-muted-foreground">
                    Red badges indicate against numbers.
                  </p>
                )}
                {resolvedStakeNumbers.some(item => item.role === 'banker') && (
                  <p className="text-xs text-muted-foreground">
                    Amber badge indicates banker number.
                  </p>
                )}
              </div>
              {isTicketSale && (
                <div className="mt-2 space-y-1 text-xs sm:text-sm">
                  <p>
                    <span className="text-muted-foreground">Bet Type:</span>{' '}
                    <span className="font-medium">
                      {resolvedBetDetails.betType ||
                        (isTicketLoading ? 'Loading...' : 'Not available')}
                    </span>
                  </p>
                  <p>
                    <span className="text-muted-foreground">Lines:</span>{' '}
                    <span className="font-medium">
                      {resolvedBetDetails.lines ??
                        (isTicketLoading ? 'Loading...' : 'Not available')}
                    </span>
                  </p>
                  <p>
                    <span className="text-muted-foreground">Per-Line Stake:</span>{' '}
                    <span className="font-medium">
                      {typeof resolvedBetDetails.perLineStake === 'number'
                        ? formatCurrency(resolvedBetDetails.perLineStake / 100)
                        : isTicketLoading
                          ? 'Loading...'
                          : 'Not available'}
                    </span>
                  </p>
                  <p>
                    <span className="text-muted-foreground">Game:</span>{' '}
                    <span className="font-medium">
                      {resolvedBetDetails.game || (isTicketLoading ? 'Loading...' : 'Not available')}
                    </span>
                  </p>
                  <p>
                    <span className="text-muted-foreground">Draw Time:</span>{' '}
                    <span className="font-medium">
                      {resolvedBetDetails.drawTime
                        ? formatDrawTime(resolvedBetDetails.drawTime)
                        : isTicketLoading
                          ? 'Loading...'
                          : 'Not available'}
                    </span>
                  </p>
                </div>
              )}
            </div>

            {/* Owner Information */}
            {(transaction.wallet_owner_name || transaction.wallet_owner_code) && (
              <div>
                <h4 className="font-medium text-xs sm:text-sm text-muted-foreground mb-2">
                  {getOwnerType(transaction.wallet_type)?.toUpperCase()} INFORMATION
                </h4>
                <div className="space-y-2">
                  <div className="flex justify-between gap-2 text-xs sm:text-sm">
                    <span>Type:</span>
                    <Badge variant="outline" className="text-xs">
                      {getOwnerType(transaction.wallet_type)}
                    </Badge>
                  </div>
                  {transaction.wallet_owner_name && (
                    <div className="flex justify-between gap-2 text-xs sm:text-sm">
                      <span>{getOwnerType(transaction.wallet_type)} Name:</span>
                      <span className="font-medium text-blue-600 text-right break-all">
                        {transaction.wallet_owner_name}
                      </span>
                    </div>
                  )}
                  {transaction.wallet_owner_code && (
                    <div className="flex justify-between gap-2 text-xs sm:text-sm">
                      <span>{getOwnerType(transaction.wallet_type)} Code:</span>
                      <span className="font-mono text-xs">{transaction.wallet_owner_code}</span>
                    </div>
                  )}
                  <div className="flex justify-between gap-2 text-xs sm:text-sm">
                    <span>{getOwnerType(transaction.wallet_type)} ID:</span>
                    <span className="font-mono text-xs break-all text-right">
                      {transaction.wallet_owner_id}
                    </span>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Description */}
        {transaction.description && (
          <div className="pt-4 border-t">
            <h4 className="font-medium text-xs sm:text-sm text-muted-foreground mb-2">
              DESCRIPTION
            </h4>
            <p className="text-xs sm:text-sm">{transaction.description}</p>
          </div>
        )}

      </div>
    )
  }

  return (
    <div className="p-3 sm:p-4 md:p-6 space-y-3 sm:space-y-4 md:space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div className="space-y-1 sm:space-y-2 min-w-0 flex-1">
          <h1 className="text-xl sm:text-2xl md:text-3xl font-bold tracking-tight">Transactions</h1>
          <p className="text-xs sm:text-sm text-muted-foreground">
            Monitor and manage all financial transactions across the lottery platform
          </p>
        </div>
        <div className="flex items-center gap-2 w-full sm:w-auto">
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isLoading}
            className="flex-1 sm:flex-none"
          >
            <RefreshCw className={cn('w-3 sm:w-4 h-3 sm:h-4 mr-2', isLoading && 'animate-spin')} />
            <span className="hidden sm:inline">Refresh</span>
            <span className="sm:hidden">Sync</span>
          </Button>
          <Button variant="outline" size="sm" className="flex-1 sm:flex-none">
            <Download className="w-3 sm:w-4 h-3 sm:h-4 mr-2" />
            <span className="hidden sm:inline">Export</span>
            <span className="sm:hidden">Export</span>
          </Button>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Total Volume Card */}
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div className="min-w-0 flex-1">
                <div className="flex items-center space-x-2 mb-2">
                  <DollarSign className="w-4 h-4 text-muted-foreground" />
                  <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                    Transaction Volume
                  </p>
                </div>
                <p className="text-2xl font-bold truncate">{formatCurrency(stats.totalVolume)}</p>
                <div className="flex items-center space-x-2 mt-2">
                  <Badge variant="secondary" className="text-xs">
                    {pagination.total.toLocaleString()} transactions
                  </Badge>
                  {transactionsData?.statistics && (
                    <span className="text-xs text-muted-foreground">system-wide</span>
                  )}
                </div>
              </div>
              <div className="w-12 h-12 bg-muted rounded-full flex items-center justify-center flex-shrink-0 ml-2">
                <TrendingUp className="w-6 h-6 text-muted-foreground" />
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Total Credits Card */}
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div className="min-w-0 flex-1">
                <div className="flex items-center space-x-2 mb-2">
                  <Plus className="w-4 h-4 text-muted-foreground" />
                  <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                    Total Credits
                  </p>
                </div>
                <p className="text-2xl font-bold truncate">{formatCurrency(stats.totalCredits)}</p>
                <div className="flex items-center space-x-2 mt-2">
                  <Badge variant="secondary" className="text-xs">
                    {transactionsData?.statistics
                      ? `${transactionsData.statistics.credit_count.toLocaleString()} credits`
                      : `${transactions.filter(tx => tx.type === 'CREDIT').length} on page`}
                  </Badge>
                  {transactionsData?.statistics && (
                    <span className="text-xs text-muted-foreground">system-wide</span>
                  )}
                </div>
              </div>
              <div className="w-12 h-12 bg-muted rounded-full flex items-center justify-center flex-shrink-0 ml-2">
                <CreditCard className="w-6 h-6 text-muted-foreground" />
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Total Debits Card */}
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div className="min-w-0 flex-1">
                <div className="flex items-center space-x-2 mb-2">
                  <Building2 className="w-4 h-4 text-muted-foreground" />
                  <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                    Total Debits
                  </p>
                </div>
                <p className="text-2xl font-bold truncate">{formatCurrency(stats.totalDebits)}</p>
                <div className="flex items-center space-x-2 mt-2">
                  <Badge variant="secondary" className="text-xs">
                    {transactionsData?.statistics
                      ? `${transactionsData.statistics.debit_count.toLocaleString()} debits`
                      : `${transactions.filter(tx => tx.type === 'DEBIT').length} on page`}
                  </Badge>
                  {transactionsData?.statistics && (
                    <span className="text-xs text-muted-foreground">system-wide</span>
                  )}
                </div>
              </div>
              <div className="w-12 h-12 bg-muted rounded-full flex items-center justify-center flex-shrink-0 ml-2">
                <Wallet className="w-6 h-6 text-muted-foreground" />
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Pending Transactions Card */}
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div className="min-w-0 flex-1">
                <div className="flex items-center space-x-2 mb-2">
                  <Clock className="w-4 h-4 text-muted-foreground" />
                  <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                    Pending Amount
                  </p>
                </div>
                <p className="text-2xl font-bold truncate">{formatCurrency(stats.pendingAmount)}</p>
                <div className="flex items-center space-x-2 mt-2">
                  <Badge variant="secondary" className="text-xs">
                    {stats.pendingCount} pending
                  </Badge>
                  <Badge variant="outline" className="text-xs">
                    {stats.completedCount} completed
                  </Badge>
                  {stats.failedCount > 0 && (
                    <Badge variant="destructive" className="text-xs">
                      {stats.failedCount} failed
                    </Badge>
                  )}
                </div>
              </div>
              <div className="w-12 h-12 bg-muted rounded-full flex items-center justify-center flex-shrink-0 ml-2">
                <AlertTriangle className="w-6 h-6 text-muted-foreground" />
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Filters and Search */}
      <Card>
        <CardHeader>
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 sm:gap-3 md:gap-4 flex-wrap">
            <div className="flex items-center gap-2 flex-1 min-w-0">
              <Search className="h-3 sm:h-4 w-3 sm:w-4 shrink-0" />
              <Input
                placeholder="Search transactions..."
                value={search}
                onChange={e => setSearch(e.target.value)}
                className="w-full sm:max-w-sm text-xs sm:text-sm"
              />
            </div>

            <Select value={typeFilter} onValueChange={setTypeFilter}>
              <SelectTrigger className="w-full sm:w-40 md:w-56 text-xs sm:text-sm">
                <SelectValue placeholder="Filter by type" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Types</SelectItem>
                <SelectItem value="CREDIT">Credit</SelectItem>
                <SelectItem value="DEBIT">Debit</SelectItem>
                <SelectItem value="TRANSFER">Transfer</SelectItem>
                <SelectItem value="COMMISSION">Commission</SelectItem>
                <SelectItem value="PAYOUT">Payout</SelectItem>
              </SelectContent>
            </Select>

            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="w-full sm:w-32 md:w-40 text-xs sm:text-sm">
                <SelectValue placeholder="Status" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Status</SelectItem>
                <SelectItem value="COMPLETED">Completed</SelectItem>
                <SelectItem value="PENDING">Pending</SelectItem>
                <SelectItem value="FAILED">Failed</SelectItem>
                <SelectItem value="REVERSED">Reversed</SelectItem>
              </SelectContent>
            </Select>

            <div className="flex items-center gap-2 w-full sm:w-auto">
              <Popover>
                <PopoverTrigger asChild>
                  <Button
                    id="date"
                    variant="outline"
                    size="sm"
                    className={cn(
                      'w-full sm:w-48 md:w-60 justify-start text-left font-normal text-xs sm:text-sm',
                      !dateFrom && 'text-muted-foreground'
                    )}
                  >
                    <Calendar className="mr-2 h-3 sm:h-4 w-3 sm:w-4 shrink-0" />
                    {dateFrom ? (
                      dateTo ? (
                        <>
                          <span className="hidden sm:inline">
                            {format(dateFrom, 'LLL dd, y')} - {format(dateTo, 'LLL dd, y')}
                          </span>
                          <span className="sm:hidden">
                            {format(dateFrom, 'MMM dd')} - {format(dateTo, 'MMM dd')}
                          </span>
                        </>
                      ) : (
                        format(dateFrom, 'LLL dd, y')
                      )
                    ) : (
                      <span>Pick a date range</span>
                    )}
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-auto p-0" align="start">
                  <div className="flex">
                    {!showCustomPicker ? (
                      <div className="p-3 space-y-2">
                        <div className="grid grid-cols-1 gap-2">
                          <Button
                            variant="ghost"
                            className="justify-start"
                            onClick={() => {
                              const today = new Date()
                              setDateFrom(today)
                              setDateTo(today)
                              setShowCustomPicker(false)
                            }}
                          >
                            Today
                          </Button>
                          <Button
                            variant="ghost"
                            className="justify-start"
                            onClick={() => {
                              const today = new Date()
                              const weekAgo = new Date(today)
                              weekAgo.setDate(today.getDate() - 7)
                              setDateFrom(weekAgo)
                              setDateTo(today)
                              setShowCustomPicker(false)
                            }}
                          >
                            Last 7 days
                          </Button>
                          <Button
                            variant="ghost"
                            className="justify-start"
                            onClick={() => {
                              const today = new Date()
                              const monthAgo = new Date(today)
                              monthAgo.setDate(today.getDate() - 30)
                              setDateFrom(monthAgo)
                              setDateTo(today)
                              setShowCustomPicker(false)
                            }}
                          >
                            Last 30 days
                          </Button>
                          <Button
                            variant="ghost"
                            className="justify-start"
                            onClick={() => {
                              setDateFrom(undefined)
                              setDateTo(undefined)
                              setShowCustomPicker(false)
                            }}
                          >
                            All time
                          </Button>
                          <div className="border-t pt-2 mt-2">
                            <Button
                              variant="ghost"
                              className="justify-start w-full"
                              onClick={() => setShowCustomPicker(true)}
                            >
                              <Calendar className="mr-2 h-4 w-4" />
                              Custom Range
                            </Button>
                          </div>
                        </div>
                      </div>
                    ) : (
                      <div className="p-3">
                        <div className="flex items-center justify-between mb-3">
                          <h4 className="font-semibold text-sm">Select Custom Range</h4>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setShowCustomPicker(false)}
                          >
                            Back
                          </Button>
                        </div>
                        <DayPicker
                          mode="range"
                          selected={{ from: dateFrom, to: dateTo }}
                          onSelect={range => {
                            if (range) {
                              setDateFrom(range.from)
                              setDateTo(range.to)
                            }
                          }}
                          numberOfMonths={2}
                          className="border-0"
                        />
                        {dateFrom && dateTo && (
                          <div className="flex justify-end space-x-2 mt-3 pt-3 border-t">
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => {
                                setDateFrom(undefined)
                                setDateTo(undefined)
                                setShowCustomPicker(false)
                              }}
                            >
                              Clear
                            </Button>
                            <Button size="sm" onClick={() => setShowCustomPicker(false)}>
                              Apply
                            </Button>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </PopoverContent>
              </Popover>
            </div>
          </div>
        </CardHeader>

        <CardContent>
          {isError && (
            <div className="mb-4 rounded-md border border-destructive/30 bg-destructive/5 p-3">
              <div className="flex items-start gap-2">
                <AlertTriangle className="h-4 w-4 text-destructive mt-0.5 shrink-0" />
                <div className="text-sm min-w-0">
                  <p className="font-medium text-destructive">Could not load transactions.</p>
                  <p className="text-muted-foreground truncate">
                    {(error as Error)?.message || 'Unknown error'}
                  </p>
                </div>
                <Button variant="outline" size="sm" onClick={() => refetch()} className="shrink-0">
                  Retry
                </Button>
              </div>
            </div>
          )}

          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-xs sm:text-sm">Reference</TableHead>
                  <TableHead className="text-xs sm:text-sm">Description</TableHead>
                  <TableHead className="text-xs sm:text-sm">Amount</TableHead>
                  <TableHead className="text-xs sm:text-sm">Status</TableHead>
                  <TableHead className="text-xs sm:text-sm">Owner</TableHead>
                  <TableHead className="text-xs sm:text-sm">Timestamp</TableHead>
                  <TableHead className="w-[100px] text-xs sm:text-sm">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {transactions.length > 0 ? (
                  transactions.map(transaction => (
                    <HoverCard key={transaction.id} openDelay={300} closeDelay={100}>
                      <HoverCardTrigger asChild>
                        <TableRow className="cursor-pointer hover:bg-muted/50">
                          <TableCell className="font-medium">
                            <div className="font-mono text-xs sm:text-sm">
                              {transaction.transaction_id ||
                                transaction.reference ||
                                transaction.id.substring(0, 8)}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center space-x-2">
                              {getTypeIcon(transaction.type)}
                              <Badge
                                variant={getTypeBadgeVariant(transaction.type)}
                                className="text-xs"
                              >
                                {getTransactionDescription(transaction)}
                              </Badge>
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="font-medium text-xs sm:text-sm">
                              {formatCurrency(transaction.amount)}
                            </div>
                            <div className="text-xs text-muted-foreground">
                              Bal: {formatCurrency(transaction.balance_after)}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center space-x-2">
                              {getStatusIcon(transaction.status)}
                              <Badge
                                variant={getStatusBadgeVariant(transaction.status)}
                                className="text-xs"
                              >
                                {transaction.status}
                              </Badge>
                            </div>
                          </TableCell>
                          <TableCell>
                            {transaction.wallet_owner_name ? (
                              <div>
                                <div className="flex items-center gap-1 mb-0.5">
                                  <Badge
                                    variant="outline"
                                    className="text-xs px-1.5 py-0 h-4 font-normal"
                                  >
                                    {getOwnerType(transaction.wallet_type)}
                                  </Badge>
                                </div>
                                <div className="font-medium text-blue-600 text-xs sm:text-sm">
                                  {transaction.wallet_owner_name}
                                </div>
                                {transaction.wallet_owner_code && (
                                  <div className="text-xs text-muted-foreground">
                                    {transaction.wallet_owner_code}
                                  </div>
                                )}
                                {transaction.wallet_owner_id && (
                                  <div className="text-xs text-muted-foreground font-mono truncate max-w-[150px]">
                                    ID: {transaction.wallet_owner_id.substring(0, 8)}...
                                  </div>
                                )}
                              </div>
                            ) : (
                              <span className="text-muted-foreground">-</span>
                            )}
                          </TableCell>
                          <TableCell>
                            <div className="text-xs sm:text-sm">{formatDate(transaction.created_at)}</div>
                          </TableCell>
                          <TableCell onClick={e => e.stopPropagation()}>
                            <Dialog>
                              <DialogTrigger asChild>
                                <Button variant="outline" size="sm">
                                  <Eye className="h-3 sm:h-4 w-3 sm:w-4" />
                                </Button>
                              </DialogTrigger>
                              <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
                                <DialogHeader>
                                  <DialogTitle className="text-base sm:text-lg">
                                    Transaction Details
                                  </DialogTitle>
                                  <DialogDescription className="text-xs sm:text-sm">
                                    Complete information for transaction{' '}
                                    {transaction.transaction_id || transaction.id}
                                  </DialogDescription>
                                </DialogHeader>
                                <TransactionDetails transaction={transaction} />
                              </DialogContent>
                            </Dialog>
                          </TableCell>
                        </TableRow>
                      </HoverCardTrigger>
                      <HoverCardContent className="w-80" side="top" align="center" sideOffset={8}>
                        <TransactionPreview transaction={transaction} />
                      </HoverCardContent>
                    </HoverCard>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={7} className="text-center py-10 text-muted-foreground">
                      <div className="flex flex-col items-center gap-2">
                        {isLoading ? (
                          <Loader2 className="h-5 w-5 animate-spin" />
                        ) : (
                          <Wallet className="h-5 w-5" />
                        )}
                        <p>
                          {isLoading
                            ? 'Loading transactions...'
                            : 'No transactions found for the selected filters.'}
                        </p>
                      </div>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>

          {/* Pagination */}
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 py-4">
            <div className="text-xs sm:text-sm text-muted-foreground">
              Showing {pagination.total === 0 ? 0 : (pagination.page - 1) * pagination.page_size + 1} to{' '}
              {Math.min(pagination.page * pagination.page_size, pagination.total)} of{' '}
              {pagination.total} transactions
            </div>
            <div className="flex items-center gap-2 w-full sm:w-auto justify-between sm:justify-end">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCurrentPage(prev => Math.max(1, prev - 1))}
                disabled={currentPage <= 1 || isLoading}
                className="text-xs sm:text-sm"
              >
                Previous
              </Button>
              <div className="flex items-center gap-1">
                <span className="text-xs sm:text-sm text-muted-foreground">
                  Page {pagination.page} of {Math.max(1, pagination.total_pages)}
                </span>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCurrentPage(prev => Math.min(Math.max(1, pagination.total_pages), prev + 1))}
                disabled={!pagination.has_more || isLoading}
                className="text-xs sm:text-sm"
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
