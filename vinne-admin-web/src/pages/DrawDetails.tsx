import React, { useState, useEffect } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft,
  Clock,
  DollarSign,
  Users,
  Trophy,
  AlertCircle,
  RefreshCw,
  CheckCircle,
  Eye,
} from 'lucide-react'
import { isPast } from 'date-fns'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Progress } from '@/components/ui/progress'
import { Separator } from '@/components/ui/separator'
import { toast } from '@/hooks/use-toast'
import NumberInputSlots from '@/components/ui/number-input-slots'
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
import { HoverCard, HoverCardContent, HoverCardTrigger } from '@/components/ui/hover-card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { drawService, type Ticket, type BetLine } from '@/services/draws'
import { formatCurrency } from '@/lib/utils'
import { PermCombinationViewer } from '@/components/PermCombinationViewer'
import { isPermBet, isBankerBet, getBetLineNumbers, getBetLineAmount } from '@/lib/bet-utils'
import { protoTimestampToDate, formatInGhanaTime } from '@/lib/date-utils'

// Helper to convert protobuf enum status to string
const getStatusString = (status: number | string): string => {
  if (typeof status === 'string') {
    // Handle protobuf enum strings (e.g., "DRAW_STATUS_IN_PROGRESS")
    const protoEnumMap: Record<string, string> = {
      DRAW_STATUS_UNSPECIFIED: 'unspecified',
      DRAW_STATUS_SCHEDULED: 'scheduled',
      DRAW_STATUS_IN_PROGRESS: 'in_progress',
      DRAW_STATUS_COMPLETED: 'completed',
      DRAW_STATUS_FAILED: 'failed',
      DRAW_STATUS_CANCELLED: 'cancelled',
    }
    return protoEnumMap[status] || status.toLowerCase()
  }
  // Map protobuf enum values to status strings
  // From draw.proto: UNSPECIFIED=0, SCHEDULED=1, IN_PROGRESS=2, COMPLETED=3, FAILED=4, CANCELLED=5
  const statusMap: Record<number, string> = {
    0: 'unspecified',
    1: 'scheduled',
    2: 'in_progress',
    3: 'completed',
    4: 'failed',
    5: 'cancelled',
  }
  return statusMap[status] || 'scheduled'
}

// Alias for compatibility
const protoStatusToString = getStatusString

// Helper to check if a stage is completed based on stage_status
const isStageCompleted = (stageStatus?: string): boolean => {
  return stageStatus === 'STAGE_STATUS_COMPLETED' || stageStatus === 'completed'
}

// Helper to format bet type for display
const formatBetType = (betType: string): string => {
  // Map internal bet type codes to display names
  const betTypeMap: Record<string, string> = {
    DIRECT_1: 'Direct-1',
    direct_1: 'Direct-1',
    DIRECT_2: 'Direct-2',
    direct_2: 'Direct-2',
    PERM_2: 'Perm-2',
    perm_2: 'Perm-2',
    PERM_3: 'Perm-3',
    perm_3: 'Perm-3',
    PERM_4: 'Perm-4',
    perm_4: 'Perm-4',
    PERM_5: 'Perm-5',
    perm_5: 'Perm-5',
    BANKER_ALL: 'Banker All',
    banker_all: 'Banker All',
    BANKER_AG: 'Banker AG',
    banker_ag: 'Banker AG',
    BANKER_NAP: 'Banker NAP',
    banker_nap: 'Banker NAP',
  }

  return betTypeMap[betType] || betType
}

const DrawDetails: React.FC = () => {
  const { drawId } = useParams({ from: '/draw/$drawId' })
  const navigate = useNavigate()

  const queryClient = useQueryClient()

  const [selectedTab, setSelectedTab] = useState('overview')
  const [ticketFilter, setTicketFilter] = useState('')
  const [restartDialogOpen, setRestartDialogOpen] = useState(false)
  const [restartReason, setRestartReason] = useState('')
  const [countdown, setCountdown] = useState<string>('')

  // Physical draw number entry state
  const [verificationNumbers, setVerificationNumbers] = useState<number[]>([])
  const [hasValidationErrors, setHasValidationErrors] = useState(false)
  const [hasDuplicateNumbers, setHasDuplicateNumbers] = useState(false)

  // Machine numbers state
  const [machineNumbersDialogOpen, setMachineNumbersDialogOpen] = useState(false)
  const [machineNumbers, setMachineNumbers] = useState<number[]>([])
  const [machineNumbersErrors, setMachineNumbersErrors] = useState(false)
  const [machineNumbersDuplicates, setMachineNumbersDuplicates] = useState(false)

  // Fetch draw details
  const { data: draw, isLoading: drawLoading } = useQuery({
    queryKey: ['draw', drawId],
    queryFn: () => drawService.getDrawById(drawId),
  })

  // Fetch draw statistics
  const { data: statistics } = useQuery({
    queryKey: ['draw-statistics', drawId],
    queryFn: () => drawService.getDrawStatistics(drawId),
    enabled: !!draw,
  })

  // Fetch tickets
  const { data: tickets, isLoading: ticketsLoading } = useQuery({
    queryKey: ['draw-tickets', drawId, ticketFilter],
    queryFn: () =>
      drawService.getDrawTickets(drawId, {
        retailer_id: ticketFilter || undefined,
        limit: 100,
      }),
    enabled: !!draw,
  })

  // Countdown timer for in-progress draws
  useEffect(() => {
    if (!draw || getStatusString(draw.status) !== 'in_progress') return

    const updateCountdown = () => {
      const endDate = protoTimestampToDate(draw.end_date)
      // Check if endDate is Unix epoch (invalid/missing date)
      if (endDate.getTime() === 0) {
        setCountdown('End date not set')
        return
      }

      const now = new Date()

      if (isPast(endDate)) {
        setCountdown('Draw ended')
        return
      }

      const diff = endDate.getTime() - now.getTime()
      const days = Math.floor(diff / (1000 * 60 * 60 * 24))
      const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
      const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))
      const seconds = Math.floor((diff % (1000 * 60)) / 1000)

      if (days > 0) {
        setCountdown(`${days}d ${hours}h ${minutes}m ${seconds}s`)
      } else if (hours > 0) {
        setCountdown(`${hours}h ${minutes}m ${seconds}s`)
      } else if (minutes > 0) {
        setCountdown(`${minutes}m ${seconds}s`)
      } else {
        setCountdown(`${seconds}s`)
      }
    }

    updateCountdown()
    const interval = setInterval(updateCountdown, 1000)
    return () => clearInterval(interval)
  }, [draw])

  // Stage 1: Draw Preparation - Start
  const prepareDrawMutation = useMutation({
    mutationFn: () => drawService.prepareDraw(drawId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['draw', drawId] })
      toast({ title: 'Draw preparation started' })
    },
  })

  // Stage 1: Draw Preparation - Complete
  const completeDrawPreparationMutation = useMutation({
    mutationFn: () => drawService.completeDrawPreparation(drawId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['draw', drawId] })
      toast({ title: 'Draw preparation completed' })
    },
  })

  // Stage 2: Physical Draw Recording
  const recordPhysicalDrawMutation = useMutation({
    mutationFn: (data: {
      numbers: number[]
      nla_draw_reference?: string
      draw_location?: string
      nla_official_signature?: string
    }) => drawService.recordPhysicalDraw(drawId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['draw', drawId] })

      // Determine which attempt was just submitted
      const currentAttemptCount =
        (draw?.stage?.number_selection_data?.verification_attempts?.length || 0) + 1

      if (currentAttemptCount === 3) {
        toast({
          title: 'Final Attempt Submitted',
          description: 'All 3 verification attempts recorded. Validating numbers...',
        })
      } else {
        toast({
          title: `Attempt ${currentAttemptCount} of 3 Recorded`,
          description: `Please enter the numbers again for verification.`,
        })
      }

      setVerificationNumbers([])
    },
    onError: (error: Error) => {
      toast({
        title: 'Recording Error',
        description: error.message,
        variant: 'destructive',
      })
    },
  })

  // Stage 3: Result Commitment
  const commitResultsMutation = useMutation({
    mutationFn: () => drawService.commitDrawResults(drawId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['draw', drawId] })
      toast({ title: 'Results committed successfully' })
    },
  })

  // Stage 4: Payout Processing
  const processPayoutMutation = useMutation({
    mutationFn: (data: { payout_mode: 'auto' | 'manual'; exclude_big_wins?: boolean }) =>
      drawService.processPayout(drawId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['draw', drawId] })
      queryClient.invalidateQueries({ queryKey: ['draw-winners', drawId] })
      toast({ title: 'Payouts processed successfully' })
    },
  })

  // Restart draw
  const restartDrawMutation = useMutation({
    mutationFn: (data: { reason: string }) => drawService.restartDrawAPI(drawId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['draw', drawId] })
      setRestartDialogOpen(false)
      setRestartReason('')
      toast({ title: 'Draw restarted successfully' })
    },
  })

  // Reset verification attempts
  const resetVerificationMutation = useMutation({
    mutationFn: () => drawService.resetVerificationAttempts(drawId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['draw', drawId] })
      toast({
        title: 'Verification Reset',
        description: 'You can now enter the winning numbers again (3 attempts)',
      })
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to reset verification attempts. Please try again.',
        variant: 'destructive',
      })
    },
  })

  const updateMachineNumbersMutation = useMutation({
    mutationFn: (numbers: number[]) => drawService.updateMachineNumbers(drawId, numbers),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['draw', drawId] })
      toast({
        title: 'Machine Numbers Updated',
        description: 'Machine numbers have been successfully saved.',
      })
      setMachineNumbersDialogOpen(false)
      setMachineNumbers([])
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to update machine numbers. Please try again.',
        variant: 'destructive',
      })
    },
  })

  const handleSaveMachineNumbers = () => {
    // Validation: check if all numbers are filled
    if (machineNumbers.length !== 5 || machineNumbers.some(num => !num || num < 1 || num > 90)) {
      setMachineNumbersErrors(true)
      return
    }

    // Check for duplicates
    const uniqueNumbers = new Set(machineNumbers)
    if (uniqueNumbers.size !== machineNumbers.length) {
      setMachineNumbersDuplicates(true)
      return
    }

    updateMachineNumbersMutation.mutate(machineNumbers)
  }

  const getStatusBadge = (status: number | string) => {
    const statusStr = getStatusString(status)
    const statusColors: Record<string, string> = {
      unspecified: 'bg-gray-100 text-gray-800',
      scheduled: 'bg-blue-100 text-blue-800',
      in_progress: 'bg-green-100 text-green-800',
      completed: 'bg-purple-100 text-purple-800',
      failed: 'bg-red-100 text-red-800',
      cancelled: 'bg-red-100 text-red-800',
    }
    const displayName =
      statusStr === 'in_progress'
        ? 'In Progress'
        : statusStr.charAt(0).toUpperCase() + statusStr.slice(1)
    return (
      <Badge className={statusColors[statusStr] || 'bg-gray-100 text-gray-800'}>
        {displayName}
      </Badge>
    )
  }

  const getStageProgress = () => {
    if (!draw?.stage) return 0
    const { current_stage } = draw.stage
    return (current_stage / 4) * 100
  }

  // Component for ticket preview
  const TicketPreview = ({ ticket }: { ticket: Ticket }) => (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="font-medium text-sm font-mono">
          {ticket.ticket_number || ticket.serial_number}
        </span>
        <Badge variant={ticket.status === 'won' ? 'default' : 'secondary'} className="text-xs">
          {ticket.status}
        </Badge>
      </div>
      <div className="text-sm space-y-1">
        <div>
          <strong>Stake:</strong> {formatCurrency(ticket.stake_amount || ticket.total_amount || 0)}
        </div>
        <div>
          <strong>Channel:</strong>{' '}
          {((ticket.channel || ticket.issuer_type || 'pos') as string).toUpperCase()}
        </div>
        {ticket.bet_lines && ticket.bet_lines.length > 0 && (
          <div>
            <strong>Bet Types:</strong>
            <div className="flex flex-wrap gap-1 mt-1">
              {[...new Set((ticket.bet_lines as BetLine[]).map(line => line.bet_type))].map(
                (betType, idx: number) => (
                  <Badge key={idx} variant="outline" className="text-xs">
                    {formatBetType(betType)}
                  </Badge>
                )
              )}
            </div>
          </div>
        )}
        {ticket.issuer_id && (
          <div>
            <strong>
              {(ticket.issuer_type as string) === 'retailer'
                ? 'Retailer'
                : (ticket.issuer_type as string) === 'agent'
                  ? 'Agent'
                  : 'Issuer'}
              :
            </strong>{' '}
            {(ticket.issuer_id as string) || 'Unknown'}
          </div>
        )}
        <div>
          <strong>Numbers:</strong>
        </div>
        {(() => {
          // Extract banker and opposed numbers from bet_lines
          const bankerNumbers =
            ticket.bet_lines
              ?.flatMap((line: BetLine) => line.banker || [])
              .filter((num: number, idx: number, arr: number[]) => arr.indexOf(num) === idx) || []
          const opposedNumbers =
            ticket.bet_lines
              ?.flatMap((line: BetLine) => line.opposed || [])
              .filter((num: number, idx: number, arr: number[]) => arr.indexOf(num) === idx) || []

          return (
            <div className="space-y-1">
              {bankerNumbers.length > 0 && (
                <div>
                  <span className="text-xs text-gray-600">Banker:</span>
                  <div className="flex gap-1 mt-0.5">
                    {bankerNumbers.map((num: number, idx: number) => (
                      <div
                        key={idx}
                        className="h-6 w-6 rounded-full bg-green-600 text-white flex items-center justify-center text-xs font-bold border border-green-400"
                      >
                        {num}
                      </div>
                    ))}
                  </div>
                </div>
              )}
              {opposedNumbers.length > 0 && (
                <div>
                  <span className="text-xs text-gray-600">Opposed:</span>
                  <div className="flex gap-1 mt-0.5">
                    {opposedNumbers.map((num: number, idx: number) => (
                      <div
                        key={idx}
                        className="h-6 w-6 rounded-full bg-red-600 text-white flex items-center justify-center text-xs font-bold border border-red-400"
                      >
                        {num}
                      </div>
                    ))}
                  </div>
                </div>
              )}
              {ticket.selected_numbers && ticket.selected_numbers.length > 0 && (
                <div>
                  {bankerNumbers.length > 0 && (
                    <span className="text-xs text-gray-600">Selected:</span>
                  )}
                  <div className="flex gap-1 mt-0.5">
                    {ticket.selected_numbers.map((num, idx) => (
                      <div
                        key={idx}
                        className="h-6 w-6 rounded-full bg-blue-600 text-white flex items-center justify-center text-xs font-bold"
                      >
                        {num}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )
        })()}
        <div>
          <strong>Purchased:</strong>{' '}
          {ticket.purchased_at || ticket.created_at
            ? formatInGhanaTime(ticket.purchased_at || ticket.created_at, 'PP p')
            : 'N/A'}
        </div>
      </div>
    </div>
  )

  // Component for ticket details dialog
  const TicketDetails = ({ ticket }: { ticket: Ticket }) => (
    <div className="space-y-6">
      {/* Ticket Format */}
      <div className="max-w-md mx-auto bg-white border-2 border-dashed border-gray-300 rounded-lg p-6 font-mono text-sm">
        {/* Ticket Header */}
        <div className="text-center border-b border-dashed border-gray-300 pb-4 mb-4">
          <h2 className="font-bold text-lg">Spiel</h2>
          <p className="text-xs text-gray-600">Licensed by National Lottery Authority</p>
          <p className="text-xs text-gray-600">Ghana</p>
        </div>

        {/* Game Title */}
        <div className="text-center mb-4">
          <h3 className="font-bold text-base">{draw?.game_name || 'LOTTERY GAME'}</h3>
          <p className="text-xs text-gray-600">Official Ticket</p>
        </div>

        {/* Ticket Details */}
        <div className="space-y-2 mb-4">
          <div className="flex justify-between">
            <span>TICKET NO:</span>
            <span className="font-bold">{ticket.ticket_number || ticket.serial_number}</span>
          </div>
          <div className="flex justify-between">
            <span>DRAW:</span>
            <span>#{draw?.draw_number}</span>
          </div>
          <div className="flex justify-between">
            <span>STAKE:</span>
            <span className="font-bold">
              {formatCurrency(ticket.stake_amount || ticket.total_amount || 0)}
            </span>
          </div>
          <div className="flex justify-between">
            <span>DATE:</span>
            <span>
              {ticket.purchased_at || ticket.created_at
                ? protoTimestampToDate(ticket.purchased_at || ticket.created_at).toLocaleDateString(
                    'en-GB'
                  )
                : 'N/A'}
            </span>
          </div>
          <div className="flex justify-between">
            <span>TIME:</span>
            <span>
              {ticket.purchased_at || ticket.created_at
                ? protoTimestampToDate(ticket.purchased_at || ticket.created_at).toLocaleTimeString(
                    'en-GB',
                    { hour12: false }
                  )
                : 'N/A'}
            </span>
          </div>
          <div className="pt-2">
            <span className="text-xs">NUMBERS:</span>
            {(() => {
              const bankerNumbers =
                ticket.bet_lines
                  ?.flatMap((line: BetLine) => line.banker || [])
                  .filter((num: number, idx: number, arr: number[]) => arr.indexOf(num) === idx) ||
                []
              const opposedNumbers =
                ticket.bet_lines
                  ?.flatMap((line: BetLine) => line.opposed || [])
                  .filter((num: number, idx: number, arr: number[]) => arr.indexOf(num) === idx) ||
                []

              return (
                <div className="space-y-2 mt-1">
                  {bankerNumbers.length > 0 && (
                    <div>
                      <span className="text-xs">BANKER:</span>
                      <div className="flex gap-1 mt-0.5 justify-center">
                        {bankerNumbers.map((num: number, idx: number) => (
                          <div
                            key={idx}
                            className="h-6 w-6 rounded-full bg-green-600 text-white flex items-center justify-center text-xs font-bold"
                          >
                            {num}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                  {opposedNumbers.length > 0 && (
                    <div>
                      <span className="text-xs">OPPOSED:</span>
                      <div className="flex gap-1 mt-0.5 justify-center">
                        {opposedNumbers.map((num: number, idx: number) => (
                          <div
                            key={idx}
                            className="h-6 w-6 rounded-full bg-red-600 text-white flex items-center justify-center text-xs font-bold"
                          >
                            {num}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                  {ticket.selected_numbers && ticket.selected_numbers.length > 0 && (
                    <div>
                      {bankerNumbers.length > 0 && <span className="text-xs">SELECTED:</span>}
                      <div className="flex gap-1 mt-0.5 justify-center">
                        {ticket.selected_numbers.map((num, idx) => (
                          <div
                            key={idx}
                            className="h-6 w-6 rounded-full bg-blue-600 text-white flex items-center justify-center text-xs font-bold"
                          >
                            {num}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )
            })()}
          </div>
        </div>

        {/* Agent/Retailer Info */}
        <div className="border-t border-dashed border-gray-300 pt-4 mb-4 space-y-1">
          {ticket.retailer_name && (
            <>
              <div className="flex justify-between text-xs">
                <span>RETAILER:</span>
                <span>{ticket.retailer_name}</span>
              </div>
              {ticket.retailer_code && (
                <div className="flex justify-between text-xs">
                  <span>CODE:</span>
                  <span>{ticket.retailer_code}</span>
                </div>
              )}
            </>
          )}
          {ticket.agent_name && (
            <>
              <div className="flex justify-between text-xs">
                <span>AGENT:</span>
                <span>{ticket.agent_name}</span>
              </div>
              {ticket.agent_code && (
                <div className="flex justify-between text-xs">
                  <span>AGENT CODE:</span>
                  <span>{ticket.agent_code}</span>
                </div>
              )}
            </>
          )}
        </div>

        {/* Status */}
        <div className="border-t border-dashed border-gray-300 pt-4 mb-4">
          <div className="flex justify-between items-center">
            <span>STATUS:</span>
            <Badge variant={ticket.status === 'won' ? 'default' : 'secondary'} className="text-xs">
              {ticket.status}
            </Badge>
          </div>
          <div className="flex justify-between text-xs mt-2">
            <span>CHANNEL:</span>
            <span>{((ticket.channel || ticket.issuer_type || 'pos') as string).toUpperCase()}</span>
          </div>
        </div>

        {/* Footer */}
        <div className="border-t border-dashed border-gray-300 pt-4 text-center text-xs text-gray-500">
          <p>Keep this ticket safe</p>
          <p>Valid for 90 days from draw date</p>
          {ticket.status === 'won' && (
            <p className="text-green-600 font-bold mt-2">*** WINNER ***</p>
          )}
        </div>
      </div>

      {/* Technical Details */}
      <div className="border-t pt-6 space-y-6">
        <h4 className="font-medium text-sm text-muted-foreground mb-4">TECHNICAL DETAILS</h4>

        {/* Ticket Information Table */}
        <div className="rounded-md border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="text-left p-3 font-medium">Field</th>
                <th className="text-left p-3 font-medium">Value</th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-b">
                <td className="p-3 text-muted-foreground">Ticket ID</td>
                <td className="p-3 font-mono">{ticket.ticket_id || ticket.id}</td>
              </tr>
              <tr className="border-b">
                <td className="p-3 text-muted-foreground">Ticket Number</td>
                <td className="p-3 font-mono font-bold">
                  {ticket.ticket_number || ticket.serial_number}
                </td>
              </tr>
              <tr className="border-b">
                <td className="p-3 text-muted-foreground">Draw Number</td>
                <td className="p-3">#{draw?.draw_number}</td>
              </tr>
              <tr className="border-b">
                <td className="p-3 text-muted-foreground">Game</td>
                <td className="p-3">
                  <Badge variant="outline" className="text-purple-600">
                    {draw?.game_name}
                  </Badge>
                </td>
              </tr>
              <tr className="border-b">
                <td className="p-3 text-muted-foreground align-top">Numbers</td>
                <td className="p-3">
                  {(() => {
                    const bankerNumbers =
                      ticket.bet_lines
                        ?.flatMap((line: BetLine) => line.banker || [])
                        .filter(
                          (num: number, idx: number, arr: number[]) => arr.indexOf(num) === idx
                        ) || []
                    const opposedNumbers =
                      ticket.bet_lines
                        ?.flatMap((line: BetLine) => line.opposed || [])
                        .filter(
                          (num: number, idx: number, arr: number[]) => arr.indexOf(num) === idx
                        ) || []

                    return (
                      <div className="space-y-2">
                        {bankerNumbers.length > 0 && (
                          <div>
                            <span className="text-xs font-medium text-gray-600 mr-2">Banker:</span>
                            <div className="flex gap-1 mt-1">
                              {bankerNumbers.map((num: number, idx: number) => (
                                <Badge
                                  key={idx}
                                  variant="outline"
                                  className="h-8 w-8 rounded-full flex items-center justify-center p-0 bg-green-100 text-green-800 border-green-300"
                                >
                                  {num}
                                </Badge>
                              ))}
                            </div>
                          </div>
                        )}
                        {opposedNumbers.length > 0 && (
                          <div>
                            <span className="text-xs font-medium text-gray-600 mr-2">Opposed:</span>
                            <div className="flex gap-1 mt-1">
                              {opposedNumbers.map((num: number, idx: number) => (
                                <Badge
                                  key={idx}
                                  variant="outline"
                                  className="h-8 w-8 rounded-full flex items-center justify-center p-0 bg-red-100 text-red-800 border-red-300"
                                >
                                  {num}
                                </Badge>
                              ))}
                            </div>
                          </div>
                        )}
                        {ticket.selected_numbers && ticket.selected_numbers.length > 0 && (
                          <div>
                            {bankerNumbers.length > 0 && (
                              <span className="text-xs font-medium text-gray-600 mr-2">
                                Selected:
                              </span>
                            )}
                            <div className="flex gap-1 mt-1">
                              {ticket.selected_numbers.map((num, idx) => (
                                <Badge
                                  key={idx}
                                  variant="outline"
                                  className="h-8 w-8 rounded-full flex items-center justify-center p-0"
                                >
                                  {num}
                                </Badge>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    )
                  })()}
                </td>
              </tr>
              {ticket.bet_lines && ticket.bet_lines.length > 0 && (
                <tr className="border-b">
                  <td className="p-3 text-muted-foreground align-top">Bet Lines</td>
                  <td className="p-3">
                    <div className="space-y-3">
                      {(ticket.bet_lines as BetLine[]).map((line, idx: number) => {
                        const numbers = getBetLineNumbers(line)
                        const amount = getBetLineAmount(line)

                        // For PERM and Banker bets, use PermCombinationViewer
                        if (isPermBet(line.bet_type) || isBankerBet(line.bet_type)) {
                          return <PermCombinationViewer key={idx} betLine={line} />
                        }

                        // For regular bets, use original display
                        return (
                          <div key={idx} className="p-2 bg-gray-50 rounded border">
                            <div className="flex items-center justify-between mb-1">
                              <Badge variant="default" className="text-xs">
                                {line.bet_type}
                              </Badge>
                              <span className="text-xs font-semibold">
                                {formatCurrency(amount)}
                              </span>
                            </div>
                            <div className="flex gap-1 flex-wrap">
                              {numbers.map((num, numIdx: number) => (
                                <Badge
                                  key={numIdx}
                                  variant="outline"
                                  className="h-6 w-6 rounded-full flex items-center justify-center p-0 text-xs"
                                >
                                  {num}
                                </Badge>
                              ))}
                            </div>
                          </div>
                        )
                      })}
                    </div>
                  </td>
                </tr>
              )}
              <tr className="border-b">
                <td className="p-3 text-muted-foreground">Stake Amount</td>
                <td className="p-3 font-semibold">
                  {formatCurrency(ticket.stake_amount || ticket.total_amount || 0)}
                </td>
              </tr>
              <tr className="border-b">
                <td className="p-3 text-muted-foreground">Status</td>
                <td className="p-3">
                  <Badge variant={ticket.status === 'won' ? 'default' : 'secondary'}>
                    {ticket.status}
                  </Badge>
                </td>
              </tr>
              <tr className="border-b">
                <td className="p-3 text-muted-foreground">Channel</td>
                <td className="p-3">
                  <Badge variant="outline">
                    {((ticket.channel || ticket.issuer_type || 'pos') as string).toUpperCase()}
                  </Badge>
                </td>
              </tr>
              {ticket.retailer_name && (
                <tr className="border-b">
                  <td className="p-3 text-muted-foreground">Retailer</td>
                  <td className="p-3">
                    <div>
                      <p className="font-medium text-green-600">{ticket.retailer_name}</p>
                      <p className="text-xs text-muted-foreground">{ticket.retailer_code}</p>
                    </div>
                  </td>
                </tr>
              )}
              {ticket.agent_name && (
                <tr className="border-b">
                  <td className="p-3 text-muted-foreground">Agent</td>
                  <td className="p-3">
                    <div>
                      <p className="font-medium text-blue-600">{ticket.agent_name}</p>
                      <p className="text-xs text-muted-foreground">{ticket.agent_code}</p>
                    </div>
                  </td>
                </tr>
              )}
              <tr>
                <td className="p-3 text-muted-foreground">Purchased At</td>
                <td className="p-3">
                  {ticket.purchased_at || ticket.created_at
                    ? formatInGhanaTime(ticket.purchased_at || ticket.created_at, 'PPp')
                    : 'N/A'}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )

  if (drawLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    )
  }

  if (!draw) {
    return (
      <Alert>
        <AlertCircle className="h-4 w-4" />
        <AlertTitle>Draw not found</AlertTitle>
        <AlertDescription>The requested draw could not be found.</AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="icon" onClick={() => navigate({ to: '/draws' })}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h1 className="text-2xl font-bold">Draw #{draw.draw_number}</h1>
            <p className="text-sm text-muted-foreground">
              {draw.game_name || 'Unknown Game'} •{' '}
              {formatInGhanaTime(draw.draw_date || draw.scheduled_time, 'PPP')}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {getStatusBadge(draw.status)}
          {draw.stage?.can_restart && (
            <Button variant="outline" size="sm" onClick={() => setRestartDialogOpen(true)}>
              <RefreshCw className="h-4 w-4 mr-2" />
              Restart Draw
            </Button>
          )}
          {getStatusString(draw.status) === 'in_progress' && countdown && (
            <Badge variant="outline" className="text-lg px-4">
              <Clock className="h-4 w-4 mr-2" />
              {countdown}
            </Badge>
          )}
        </div>
      </div>

      {/* Statistics Cards */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Tickets</CardTitle>
            <Users className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {ticketsLoading ? (
                <span className="text-muted-foreground">...</span>
              ) : (
                (
                  (tickets?.total ?? 0) > 0 ? tickets!.total :
                  (statistics?.total_tickets ?? 0) > 0 ? statistics!.total_tickets :
                  (statistics?.total_tickets_sold ?? 0) > 0 ? statistics!.total_tickets_sold :
                  (draw.total_tickets_sold ?? 0) > 0 ? draw.total_tickets_sold :
                  0
                ).toLocaleString()
              )}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Stakes</CardTitle>
            <DollarSign className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatCurrency(
                (() => {
                  const fromTickets = tickets?.tickets?.reduce(
                    (sum: number, t: Record<string, unknown>) =>
                      sum + ((t.total_amount as number) || 0),
                    0
                  ) ?? 0
                  if (fromTickets > 0) return fromTickets
                  if ((statistics?.total_stakes ?? 0) > 0) return statistics!.total_stakes
                  if ((draw.total_stakes ?? 0) > 0) return draw.total_stakes
                  return draw.total_prize_pool ?? 0
                })()
              )}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Winners</CardTitle>
            <Trophy className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {statistics?.total_winners?.toLocaleString() || '-'}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Winnings</CardTitle>
            <DollarSign className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatCurrency(statistics?.total_winnings || draw.total_winnings || 0)}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Draw Execution Flow for Scheduled and In-Progress Draws */}
      {(getStatusString(draw.status) === 'scheduled' ||
        getStatusString(draw.status) === 'in_progress') && (
        <Card>
          <CardHeader>
            <CardTitle>Draw Execution</CardTitle>
            <CardDescription>Complete the draw execution process</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-6">
              {/* Initialize Draw Execution if no stage exists */}
              {!draw.stage && (
                <div className="text-center py-8 space-y-4">
                  <div>
                    <h3 className="font-semibold text-lg mb-2">Ready to Execute Draw</h3>
                    <p className="text-sm text-muted-foreground">
                      This draw is scheduled and ready for execution. Click the button below to
                      begin the draw execution process.
                    </p>
                  </div>
                  <Button
                    size="lg"
                    onClick={() => prepareDrawMutation.mutate()}
                    disabled={prepareDrawMutation.isPending}
                  >
                    Initialize Draw Execution
                  </Button>
                </div>
              )}

              {/* Progress Bar */}
              {draw.stage && (
                <>
                  <div className="space-y-2">
                    <div className="flex justify-between text-sm">
                      <span>Stage {draw.stage?.current_stage || 0} of 4</span>
                      <span>{getStageProgress()}%</span>
                    </div>
                    <Progress value={getStageProgress()} />
                  </div>

                  {/* Stage 1: Draw Preparation */}
                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <div
                          className={`h-8 w-8 rounded-full flex items-center justify-center ${
                            isStageCompleted(draw.stage?.stage_status) &&
                            draw.stage?.current_stage === 1
                              ? 'bg-green-500 text-white'
                              : draw.stage?.current_stage === 1
                                ? 'bg-blue-500 text-white'
                                : 'bg-gray-200'
                          }`}
                        >
                          {isStageCompleted(draw.stage?.stage_status) &&
                          draw.stage?.current_stage === 1 ? (
                            <CheckCircle className="h-5 w-5" />
                          ) : (
                            '1'
                          )}
                        </div>
                        <div>
                          <h3 className="font-semibold">Draw Preparation</h3>
                          <p className="text-sm text-muted-foreground">
                            Review configuration and lock sales
                          </p>
                        </div>
                      </div>
                      {draw.stage?.current_stage === 1 &&
                        !isStageCompleted(draw.stage?.stage_status) && (
                          <div className="flex gap-2">
                            <Button
                              size="sm"
                              onClick={() => completeDrawPreparationMutation.mutate()}
                              disabled={completeDrawPreparationMutation.isPending}
                            >
                              Prepare Draw
                            </Button>
                          </div>
                        )}
                    </div>
                    {draw.stage?.preparation_data && (
                      <div className="ml-10 p-3 bg-gray-50 rounded-lg text-sm">
                        <div className="grid grid-cols-2 gap-2">
                          <div>Total Tickets: {draw.stage.preparation_data.tickets_locked}</div>
                          <div>
                            Total Stakes: {formatCurrency(draw.stage.preparation_data.total_stakes)}
                          </div>
                          <div>
                            Sales Locked: {draw.stage.preparation_data.sales_locked ? 'Yes' : 'No'}
                          </div>
                          <div>
                            Locked At:{' '}
                            {draw.stage.preparation_data.lock_time
                              ? formatInGhanaTime(draw.stage.preparation_data.lock_time, 'PPp')
                              : 'N/A'}
                          </div>
                        </div>
                      </div>
                    )}
                  </div>

                  <Separator />

                  {/* Stage 2: Number Selection */}
                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <div
                          className={`h-8 w-8 rounded-full flex items-center justify-center ${
                            isStageCompleted(draw.stage?.stage_status) &&
                            draw.stage?.current_stage === 2
                              ? 'bg-green-500 text-white'
                              : draw.stage?.current_stage === 2
                                ? 'bg-blue-500 text-white'
                                : 'bg-gray-200'
                          }`}
                        >
                          {isStageCompleted(draw.stage?.stage_status) &&
                          draw.stage?.current_stage === 2 ? (
                            <CheckCircle className="h-5 w-5" />
                          ) : (
                            '2'
                          )}
                        </div>
                        <div>
                          <h3 className="font-semibold">Physical Draw Recording</h3>
                          <p className="text-sm text-muted-foreground">
                            Record winning numbers from physical draw
                          </p>
                        </div>
                      </div>
                      {draw.stage?.current_stage === 2 &&
                        !isStageCompleted(draw.stage?.stage_status) && (
                          <div className="flex gap-2">
                            {!draw.stage.number_selection_data?.is_verified && (
                              <div className="flex flex-col gap-3 w-full">
                                {/* Show attempt count only - not the numbers (for triple-entry validation integrity) */}
                                {draw.stage.number_selection_data?.verification_attempts &&
                                  draw.stage.number_selection_data.verification_attempts.length >
                                    0 && (
                                    <div className="bg-blue-50 border border-blue-200 rounded-lg p-3">
                                      <div className="flex items-center gap-2">
                                        <div className="flex-shrink-0 h-8 w-8 rounded-full bg-blue-500 text-white flex items-center justify-center font-bold">
                                          {
                                            draw.stage.number_selection_data.verification_attempts
                                              .length
                                          }
                                        </div>
                                        <div>
                                          <p className="text-sm font-medium text-blue-900">
                                            {
                                              draw.stage.number_selection_data.verification_attempts
                                                .length
                                            }{' '}
                                            of 3 attempts completed
                                          </p>
                                          <p className="text-xs text-blue-700">
                                            Previous entries are hidden to ensure independent
                                            verification
                                          </p>
                                        </div>
                                      </div>
                                    </div>
                                  )}

                                {/* Number entry form - show if less than 3 attempts */}
                                {(!draw.stage.number_selection_data?.verification_attempts ||
                                  draw.stage.number_selection_data.verification_attempts.length <
                                    3) && (
                                  <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
                                    <div className="flex items-center justify-between mb-2">
                                      <h4 className="font-semibold text-blue-900">
                                        Enter Physical Draw Numbers
                                      </h4>
                                      <Badge variant="default">
                                        Attempt{' '}
                                        {(draw.stage.number_selection_data?.verification_attempts
                                          ?.length || 0) + 1}{' '}
                                        of 3
                                      </Badge>
                                    </div>
                                    <p className="text-sm text-blue-800 mb-4">
                                      Enter the 5 winning numbers (1-90) from the physical draw.
                                      Numbers must be entered 3 times for verification.
                                    </p>
                                    <NumberInputSlots
                                      value={verificationNumbers}
                                      onChange={setVerificationNumbers}
                                      onValidationChange={(isValid, hasDuplicates) => {
                                        setHasValidationErrors(!isValid)
                                        setHasDuplicateNumbers(hasDuplicates)
                                      }}
                                      disabled={recordPhysicalDrawMutation.isPending}
                                      className="justify-center"
                                    />
                                    {hasDuplicateNumbers && (
                                      <Alert variant="destructive" className="mt-3">
                                        <AlertDescription>
                                          Each number must be unique. Duplicate numbers are not
                                          allowed.
                                        </AlertDescription>
                                      </Alert>
                                    )}
                                    <div className="flex justify-center mt-4">
                                      <Button
                                        onClick={() => {
                                          if (
                                            verificationNumbers.length === 5 &&
                                            !hasDuplicateNumbers
                                          ) {
                                            recordPhysicalDrawMutation.mutate({
                                              numbers: verificationNumbers,
                                            })
                                          } else {
                                            toast({
                                              title: 'Invalid Entry',
                                              description: 'Please enter all 5 unique numbers',
                                              variant: 'destructive',
                                            })
                                          }
                                        }}
                                        disabled={
                                          verificationNumbers.length !== 5 ||
                                          hasValidationErrors ||
                                          hasDuplicateNumbers ||
                                          recordPhysicalDrawMutation.isPending
                                        }
                                      >
                                        {recordPhysicalDrawMutation.isPending
                                          ? 'Recording...'
                                          : 'Submit Attempt'}
                                      </Button>
                                    </div>
                                  </div>
                                )}

                                {/* Show warning if 3 attempts don't match */}
                                {draw.stage.number_selection_data?.verification_attempts?.length ===
                                  3 &&
                                  !draw.stage.number_selection_data?.is_verified && (
                                    <Alert variant="destructive">
                                      <AlertDescription className="flex items-center justify-between">
                                        <span>
                                          The 3 verification attempts do not match. You can start
                                          over to re-enter the winning numbers.
                                        </span>
                                        <Button
                                          variant="outline"
                                          size="sm"
                                          onClick={() => resetVerificationMutation.mutate()}
                                          disabled={resetVerificationMutation.isPending}
                                          className="ml-4 flex-shrink-0"
                                        >
                                          {resetVerificationMutation.isPending ? (
                                            <>
                                              <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                                              Resetting...
                                            </>
                                          ) : (
                                            <>
                                              <RefreshCw className="mr-2 h-4 w-4" />
                                              Start Over
                                            </>
                                          )}
                                        </Button>
                                      </AlertDescription>
                                    </Alert>
                                  )}
                              </div>
                            )}
                          </div>
                        )}
                    </div>
                    {draw.stage?.number_selection_data && (
                      <div className="ml-10 p-3 bg-gray-50 rounded-lg">
                        <div className="space-y-2">
                          {draw.stage.number_selection_data.winning_numbers?.length > 0 && (
                            <div>
                              <p className="text-sm font-medium mb-2">Winning Numbers:</p>
                              <div className="flex gap-2">
                                {draw.stage.number_selection_data.winning_numbers.map(
                                  (num: number, idx: number) => (
                                    <div
                                      key={idx}
                                      className="h-10 w-10 rounded-full bg-blue-500 text-white flex items-center justify-center font-bold"
                                    >
                                      {num}
                                    </div>
                                  )
                                )}
                              </div>
                            </div>
                          )}
                          {draw.stage.number_selection_data.is_verified && (
                            <div className="flex items-center gap-2 mt-2">
                              <Badge variant="default">
                                <CheckCircle className="h-3 w-3 mr-1" />
                                Verified
                              </Badge>
                              {draw.stage.number_selection_data.verified_by && (
                                <span className="text-sm text-muted-foreground">
                                  by {draw.stage.number_selection_data.verified_by}
                                  {draw.stage.number_selection_data.verified_at && (
                                    <>
                                      {' '}
                                      at{' '}
                                      {formatInGhanaTime(
                                        draw.stage.number_selection_data.verified_at,
                                        'PPp'
                                      )}
                                    </>
                                  )}
                                </span>
                              )}
                            </div>
                          )}
                        </div>
                      </div>
                    )}
                  </div>

                  <Separator />

                  {/* Stage 3: Result Commitment */}
                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <div
                          className={`h-8 w-8 rounded-full flex items-center justify-center ${
                            isStageCompleted(draw.stage?.stage_status) &&
                            draw.stage?.current_stage === 3
                              ? 'bg-green-500 text-white'
                              : draw.stage?.current_stage === 3
                                ? 'bg-blue-500 text-white'
                                : 'bg-gray-200'
                          }`}
                        >
                          {isStageCompleted(draw.stage?.stage_status) &&
                          draw.stage?.current_stage === 3 ? (
                            <CheckCircle className="h-5 w-5" />
                          ) : (
                            '3'
                          )}
                        </div>
                        <div>
                          <h3 className="font-semibold">Result Commitment</h3>
                          <p className="text-sm text-muted-foreground">
                            Commit results and calculate winners
                          </p>
                        </div>
                      </div>
                      {((draw.stage?.current_stage === 3 &&
                        !isStageCompleted(draw.stage?.stage_status)) ||
                        (draw.stage?.current_stage === 2 &&
                          isStageCompleted(draw.stage?.stage_status))) && (
                        <Button
                          size="sm"
                          onClick={() => commitResultsMutation.mutate()}
                          disabled={commitResultsMutation.isPending}
                        >
                          Commit Results
                        </Button>
                      )}
                    </div>
                    {draw.stage?.result_calculation_data && (
                      <div className="ml-10 space-y-4">
                        {/* Summary Stats */}
                        <div className="p-4 bg-gradient-to-r from-green-50 to-blue-50 rounded-lg border">
                          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 text-sm">
                            <div className="flex flex-col">
                              <span className="text-muted-foreground">Total Winners</span>
                              <span className="text-lg font-bold text-green-600">
                                {typeof draw.stage.result_calculation_data
                                  ?.winning_tickets_count === 'string'
                                  ? parseInt(
                                      draw.stage.result_calculation_data.winning_tickets_count
                                    ).toLocaleString()
                                  : (
                                      draw.stage.result_calculation_data?.winning_tickets_count || 0
                                    ).toLocaleString()}
                              </span>
                            </div>
                            <div className="flex flex-col">
                              <span className="text-muted-foreground">Total Winnings</span>
                              <span className="text-lg font-bold text-blue-600">
                                {formatCurrency(
                                  draw.stage.result_calculation_data.total_winnings || 0
                                )}
                              </span>
                            </div>
                            <div className="flex flex-col">
                              <span className="text-muted-foreground">Calculated At</span>
                              <span className="text-sm font-semibold">
                                {draw.stage.result_calculation_data.calculated_at
                                  ? formatInGhanaTime(
                                      draw.stage.result_calculation_data.calculated_at,
                                      'PPp'
                                    )
                                  : 'N/A'}
                              </span>
                            </div>
                          </div>
                        </div>

                        {/* Winning Tickets Table */}
                        {draw.stage.result_calculation_data.winning_tickets &&
                        draw.stage.result_calculation_data.winning_tickets.length > 0 ? (
                          <div className="border rounded-lg overflow-hidden">
                            <div className="px-4 py-2 bg-gray-50 border-b">
                              <h4 className="font-semibold text-sm">Winning Tickets</h4>
                            </div>
                            <Table>
                              <TableHeader>
                                <TableRow className="text-xs">
                                  <TableHead>Ticket Serial</TableHead>
                                  <TableHead>Sale Date/Time</TableHead>
                                  <TableHead>Draw Date/Time</TableHead>
                                  <TableHead>Numbers</TableHead>
                                  <TableHead>Game Type</TableHead>
                                  <TableHead>No. of Lines</TableHead>
                                  <TableHead className="w-20">Matches</TableHead>
                                  <TableHead className="text-right">Stake</TableHead>
                                  <TableHead className="text-right">Amount Won</TableHead>
                                  <TableHead>Issuer</TableHead>
                                  <TableHead>Terminal ID</TableHead>
                                  <TableHead>Phone</TableHead>
                                  <TableHead>Payment</TableHead>
                                  <TableHead>Status</TableHead>
                                  <TableHead className="w-20">Big Win?</TableHead>
                                </TableRow>
                              </TableHeader>
                              <TableBody>
                                {draw.stage.result_calculation_data.winning_tickets.map(
                                  (ticket: Record<string, unknown>, index: number) => (
                                    <TableRow
                                      key={
                                        (ticket.ticket_id as string) ||
                                        (ticket.serial_number as string) ||
                                        `ticket-${index}`
                                      }
                                      className="text-xs"
                                    >
                                      {/* Ticket Serial */}
                                      <TableCell className="font-medium font-mono">
                                        {ticket.serial_number as string}
                                      </TableCell>

                                      {/* Sale Date/Time */}
                                      <TableCell className="text-xs">
                                        {ticket.created_at || ticket.purchased_at
                                          ? new Date(
                                              (ticket.created_at || ticket.purchased_at) as string
                                            ).toLocaleString()
                                          : '-'}
                                      </TableCell>

                                      {/* Draw Date/Time */}
                                      <TableCell className="text-xs">
                                        {draw.executed_time
                                          ? new Date(draw.executed_time).toLocaleString()
                                          : draw.scheduled_time
                                            ? new Date(draw.scheduled_time).toLocaleString()
                                            : '-'}
                                      </TableCell>

                                      {/* Numbers */}
                                      <TableCell className="font-mono text-xs">
                                        {Array.isArray(ticket.numbers)
                                          ? (ticket.numbers as number[]).join(', ')
                                          : 'N/A'}
                                      </TableCell>

                                      {/* Game Type (Bet Type) */}
                                      <TableCell>
                                        <Badge variant="outline">{ticket.bet_type as string}</Badge>
                                      </TableCell>

                                      {/* No. of Lines */}
                                      <TableCell className="text-center">
                                        {(ticket.lines_count as number) || 1}
                                      </TableCell>

                                      {/* Matches */}
                                      <TableCell>{ticket.matches_count as number} of 5</TableCell>

                                      {/* Stake */}
                                      <TableCell className="text-right">
                                        {formatCurrency(
                                          typeof ticket.stake_amount === 'string'
                                            ? parseInt(ticket.stake_amount)
                                            : (ticket.stake_amount as number)
                                        )}
                                      </TableCell>

                                      {/* Amount Won */}
                                      <TableCell className="text-right font-bold text-green-600">
                                        {formatCurrency(
                                          typeof ticket.winning_amount === 'string'
                                            ? parseInt(ticket.winning_amount)
                                            : (ticket.winning_amount as number)
                                        )}
                                      </TableCell>

                                      {/* POS Terminal (optional) */}
                                      {/* Issuer */}
                                      <TableCell className="font-mono text-xs">
                                        {String(
                                          (ticket.issuer_details as Record<string, unknown>)
                                            ?.retailer_code ||
                                            (ticket.issuer_details as Record<string, unknown>)
                                              ?.player_id ||
                                            (ticket.retailer_code as string) ||
                                            (ticket.player_id as string) ||
                                            '-'
                                        )}
                                      </TableCell>

                                      {/* Terminal ID (optional) */}
                                      <TableCell className="font-mono text-xs">
                                        {String(
                                          (ticket.issuer_details as Record<string, unknown>)
                                            ?.terminal_id ||
                                            (ticket.terminal_id as string) ||
                                            '-'
                                        )}
                                      </TableCell>

                                      {/* Phone */}
                                      <TableCell className="text-xs">
                                        {(ticket.customer_phone as string) || '-'}
                                      </TableCell>

                                      {/* Payment */}
                                      <TableCell className="text-xs capitalize">
                                        {(ticket.payment_method as string) || '-'}
                                      </TableCell>

                                      {/* Status */}
                                      <TableCell>
                                        <Badge variant="outline" className="text-xs capitalize">
                                          {(ticket.status as string) || 'active'}
                                        </Badge>
                                      </TableCell>

                                      {/* Big Win? */}
                                      <TableCell>
                                        {ticket.is_big_win ? (
                                          <Badge className="bg-orange-100 text-orange-800 text-xs">
                                            Yes
                                          </Badge>
                                        ) : (
                                          <span className="text-gray-400 text-xs">No</span>
                                        )}
                                      </TableCell>
                                    </TableRow>
                                  )
                                )}
                              </TableBody>
                            </Table>
                          </div>
                        ) : draw.stage.result_calculation_data.winning_tiers &&
                          draw.stage.result_calculation_data.winning_tiers.length > 0 ? (
                          <div className="border rounded-lg overflow-hidden">
                            <div className="px-4 py-2 bg-gray-50 border-b">
                              <h4 className="font-semibold text-sm">
                                Winning Breakdown by Bet Type
                              </h4>
                              <p className="text-xs text-muted-foreground mt-1">
                                Individual ticket details not available for this draw
                              </p>
                            </div>
                            <Table>
                              <TableHeader>
                                <TableRow className="text-xs">
                                  <TableHead>Bet Type</TableHead>
                                  <TableHead className="text-right">Winners</TableHead>
                                  <TableHead className="text-right">Total Winnings</TableHead>
                                  <TableHead className="text-right">Avg Per Winner</TableHead>
                                </TableRow>
                              </TableHeader>
                              <TableBody>
                                {draw.stage.result_calculation_data.winning_tiers.map(
                                  (tier: Record<string, unknown>) => {
                                    const winnersCount =
                                      typeof tier.winners_count === 'string'
                                        ? parseInt(tier.winners_count)
                                        : (tier.winners_count as number)
                                    const totalAmount =
                                      typeof tier.total_amount === 'string'
                                        ? parseInt(tier.total_amount)
                                        : (tier.total_amount as number)
                                    const avgPerWinner =
                                      winnersCount > 0 ? totalAmount / winnersCount : 0

                                    return (
                                      <TableRow key={tier.bet_type as string} className="text-xs">
                                        <TableCell>
                                          <Badge variant="outline">{tier.bet_type as string}</Badge>
                                        </TableCell>
                                        <TableCell className="text-right font-semibold">
                                          {winnersCount.toLocaleString()}
                                        </TableCell>
                                        <TableCell className="text-right font-bold text-green-600">
                                          {formatCurrency(totalAmount)}
                                        </TableCell>
                                        <TableCell className="text-right text-muted-foreground">
                                          {formatCurrency(avgPerWinner)}
                                        </TableCell>
                                      </TableRow>
                                    )
                                  }
                                )}
                              </TableBody>
                            </Table>
                          </div>
                        ) : null}
                      </div>
                    )}
                  </div>

                  <Separator />

                  {/* Stage 4: Payout Processing */}
                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <div
                          className={`h-8 w-8 rounded-full flex items-center justify-center ${
                            isStageCompleted(draw.stage?.stage_status) &&
                            draw.stage?.current_stage === 4
                              ? 'bg-green-500 text-white'
                              : draw.stage?.current_stage === 4
                                ? 'bg-blue-500 text-white'
                                : 'bg-gray-200'
                          }`}
                        >
                          {isStageCompleted(draw.stage?.stage_status) &&
                          draw.stage?.current_stage === 4 ? (
                            <CheckCircle className="h-5 w-5" />
                          ) : (
                            '4'
                          )}
                        </div>
                        <div>
                          <h3 className="font-semibold">Payout Processing</h3>
                          <p className="text-sm text-muted-foreground">
                            Process payouts to retailer wallets
                          </p>
                        </div>
                      </div>
                      {draw.stage?.current_stage === 4 &&
                        !isStageCompleted(draw.stage?.stage_status) && (
                          <div className="flex gap-2">
                            <Button
                              size="sm"
                              onClick={() =>
                                processPayoutMutation.mutate({
                                  payout_mode: 'auto',
                                  exclude_big_wins: true,
                                })
                              }
                              disabled={processPayoutMutation.isPending}
                            >
                              Process Normal Payouts
                            </Button>
                          </div>
                        )}
                    </div>
                    {draw.stage?.payout_data && (
                      <div className="ml-10 space-y-3">
                        <div className="p-3 bg-gray-50 rounded-lg">
                          <div className="grid grid-cols-2 gap-3 text-sm">
                            <div>
                              <span className="text-muted-foreground">Auto Processed:</span>
                              <p className="font-semibold">
                                {draw.stage.payout_data.auto_processed_count} tickets
                              </p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Manual Approval Needed:</span>
                              <p className="font-semibold">
                                {draw.stage.payout_data.manual_approval_count} tickets
                              </p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Processed:</span>
                              <p className="font-semibold">
                                {draw.stage.payout_data.processed_count} tickets
                              </p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Pending:</span>
                              <p className="font-semibold">
                                {draw.stage.payout_data.pending_count} tickets
                              </p>
                            </div>
                          </div>
                        </div>
                      </div>
                    )}
                  </div>
                </>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Tabs for Additional Information */}
      <Tabs value={selectedTab} onValueChange={setSelectedTab}>
        <TabsList className="grid w-full grid-cols-2">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="tickets">Tickets</TabsTrigger>
        </TabsList>

        {/* Overview Tab */}
        <TabsContent value="overview" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Draw Information</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label>Draw Number</Label>
                  <p className="font-semibold">{draw.draw_number}</p>
                </div>
                <div>
                  <Label>Game</Label>
                  <p className="font-semibold">{draw.game_name || 'Unknown'}</p>
                </div>
                <div>
                  <Label>Date Created</Label>
                  <p className="font-semibold">{formatInGhanaTime(draw.created_at, 'PPP p')}</p>
                </div>
                <div>
                  <Label>Draw Date</Label>
                  <p className="font-semibold">{formatInGhanaTime(draw.scheduled_time, 'PPP p')}</p>
                </div>
                <div>
                  <Label>Status</Label>
                  <div className="mt-1">{getStatusBadge(draw.status)}</div>
                </div>
              </div>
              {draw.winning_numbers && draw.winning_numbers.length > 0 && (
                <div>
                  <Label>Winning Numbers</Label>
                  <div className="flex gap-2 mt-2">
                    {draw.winning_numbers.map((num: number, idx: number) => (
                      <div
                        key={idx}
                        className="h-12 w-12 rounded-full bg-green-500 text-white flex items-center justify-center font-bold text-lg"
                      >
                        {num}
                      </div>
                    ))}
                  </div>
                </div>
              )}
              {draw.machine_numbers && draw.machine_numbers.length > 0 && (
                <div>
                  <Label>Machine Numbers</Label>
                  <div className="flex gap-2 mt-2">
                    {draw.machine_numbers.map((num: number, idx: number) => (
                      <div
                        key={idx}
                        className="h-12 w-12 rounded-full bg-blue-500 text-white flex items-center justify-center font-bold text-lg"
                      >
                        {num}
                      </div>
                    ))}
                  </div>
                </div>
              )}
              {protoStatusToString(draw.status) === 'completed' &&
                (!draw.machine_numbers || draw.machine_numbers.length === 0) && (
                  <div>
                    <Button onClick={() => setMachineNumbersDialogOpen(true)} variant="outline">
                      Add Machine Numbers
                    </Button>
                  </div>
                )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Tickets Tab */}
        <TabsContent value="tickets">
          <Card>
            <CardHeader>
              <CardTitle>Tickets Sold</CardTitle>
              <CardDescription>All tickets purchased for this draw</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="mb-4">
                <Input
                  placeholder="Filter by issuer ID (retailer/agent)..."
                  value={ticketFilter}
                  onChange={e => setTicketFilter(e.target.value)}
                  className="max-w-sm"
                />
              </div>
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Ticket Number</TableHead>
                      <TableHead>Sale Date/Time</TableHead>
                      <TableHead>Draw Date/Time</TableHead>
                      <TableHead>Issuer</TableHead>
                      <TableHead>Game Type</TableHead>
                      <TableHead>No. of Lines</TableHead>
                      <TableHead>Numbers</TableHead>
                      <TableHead>Stake</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Amount Won</TableHead>
                      <TableHead>Terminal ID</TableHead>
                      <TableHead>Channel</TableHead>
                      <TableHead className="w-[100px]">Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {tickets?.tickets?.map((ticket: Record<string, unknown>) => (
                      <HoverCard key={ticket.id as string} openDelay={300} closeDelay={100}>
                        <HoverCardTrigger asChild>
                          <TableRow className="cursor-pointer hover:bg-muted/50">
                            {/* Ticket Number */}
                            <TableCell className="font-mono">
                              {ticket.serial_number as string}
                            </TableCell>

                            {/* Sale Date/Time */}
                            <TableCell className="text-xs">
                              {ticket.created_at &&
                              (typeof ticket.created_at === 'string' ||
                                typeof ticket.created_at === 'object')
                                ? formatInGhanaTime(
                                    ticket.created_at as
                                      | string
                                      | { seconds: number; nanos?: number },
                                    'PP p'
                                  )
                                : '-'}
                            </TableCell>

                            {/* Draw Date/Time */}
                            <TableCell className="text-xs">
                              {draw.executed_time
                                ? new Date(draw.executed_time).toLocaleString()
                                : draw.scheduled_time
                                  ? new Date(draw.scheduled_time).toLocaleString()
                                  : '-'}
                            </TableCell>

                            {/* Issuer */}
                            <TableCell>
                              <div>
                                <p className="font-medium text-xs">
                                  {String(
                                    (ticket.issuer_details as Record<string, unknown>)
                                      ?.retailer_code ||
                                      (ticket.issuer_details as Record<string, unknown>)
                                        ?.player_id ||
                                      (ticket.retailer_code as string) ||
                                      (ticket.player_id as string) ||
                                      (ticket.issuer_id as string) ||
                                      'Unknown'
                                  )}
                                </p>
                                <p className="text-xs text-muted-foreground">
                                  {ticket.issuer_type as string}
                                </p>
                              </div>
                            </TableCell>

                            {/* Game Type */}
                            <TableCell>
                              {(ticket.bet_lines as BetLine[]) &&
                              (ticket.bet_lines as BetLine[]).length > 0 ? (
                                <div className="flex flex-wrap gap-1">
                                  {[
                                    ...new Set(
                                      (ticket.bet_lines as BetLine[]).map(line => line.bet_type)
                                    ),
                                  ].map((betType, idx: number) => (
                                    <Badge key={idx} variant="outline" className="text-xs">
                                      {formatBetType(betType)}
                                    </Badge>
                                  ))}
                                </div>
                              ) : (
                                '-'
                              )}
                            </TableCell>

                            {/* No. of Lines */}
                            <TableCell className="text-center text-xs">
                              {(ticket.bet_lines as BetLine[])?.length || 1}
                            </TableCell>
                            <TableCell>
                              {(() => {
                                const bankerNumbers =
                                  (ticket.bet_lines as BetLine[])
                                    ?.flatMap((line: BetLine) => line.banker || [])
                                    .filter(
                                      (num: number, idx: number, arr: number[]) =>
                                        arr.indexOf(num) === idx
                                    ) || []
                                const opposedNumbers =
                                  (ticket.bet_lines as BetLine[])
                                    ?.flatMap((line: BetLine) => line.opposed || [])
                                    .filter(
                                      (num: number, idx: number, arr: number[]) =>
                                        arr.indexOf(num) === idx
                                    ) || []

                                return (
                                  <div className="space-y-1">
                                    {bankerNumbers.length > 0 && (
                                      <div className="flex gap-1 items-center">
                                        <span className="text-xs text-gray-500">B:</span>
                                        {bankerNumbers.map((num: number, idx: number) => (
                                          <Badge
                                            key={idx}
                                            variant="outline"
                                            className="bg-green-100 text-green-800 border-green-300"
                                          >
                                            {num}
                                          </Badge>
                                        ))}
                                      </div>
                                    )}
                                    {opposedNumbers.length > 0 && (
                                      <div className="flex gap-1 items-center">
                                        <span className="text-xs text-gray-500">O:</span>
                                        {opposedNumbers.map((num: number, idx: number) => (
                                          <Badge
                                            key={idx}
                                            variant="outline"
                                            className="bg-red-100 text-red-800 border-red-300"
                                          >
                                            {num}
                                          </Badge>
                                        ))}
                                      </div>
                                    )}
                                    {(ticket.selected_numbers as number[]) &&
                                      (ticket.selected_numbers as number[]).length > 0 && (
                                        <div className="flex gap-1 items-center">
                                          {bankerNumbers.length > 0 && (
                                            <span className="text-xs text-gray-500">S:</span>
                                          )}
                                          {(ticket.selected_numbers as number[]).map(
                                            (num: number, idx: number) => (
                                              <Badge key={idx} variant="outline">
                                                {num}
                                              </Badge>
                                            )
                                          )}
                                        </div>
                                      )}
                                  </div>
                                )
                              })()}
                            </TableCell>

                            {/* Stake */}
                            <TableCell className="text-xs">
                              {formatCurrency(ticket.total_amount as number)}
                            </TableCell>

                            {/* Status */}
                            <TableCell>
                              <Badge
                                variant={
                                  (ticket.status as string) === 'won' ? 'default' : 'secondary'
                                }
                                className="text-xs"
                              >
                                {ticket.status as string}
                              </Badge>
                            </TableCell>

                            {/* Amount Won */}
                            <TableCell className="text-xs">
                              {(ticket.status as string) === 'won' && ticket.winning_amount
                                ? formatCurrency(
                                    typeof ticket.winning_amount === 'string'
                                      ? parseInt(ticket.winning_amount)
                                      : (ticket.winning_amount as number)
                                  )
                                : '-'}
                            </TableCell>

                            {/* Terminal ID (optional) */}
                            <TableCell className="font-mono text-xs">
                              {String(
                                (ticket.issuer_details as Record<string, unknown>)?.terminal_id ||
                                  (ticket.terminal_id as string) ||
                                  '-'
                              )}
                            </TableCell>

                            {/* Channel */}
                            <TableCell>
                              <Badge variant="outline" className="text-xs">
                                {(ticket.issuer_type as string) || 'POS'}
                              </Badge>
                            </TableCell>
                            <TableCell onClick={e => e.stopPropagation()}>
                              <Dialog>
                                <DialogTrigger asChild>
                                  <Button variant="outline" size="sm">
                                    <Eye className="h-4 w-4" />
                                  </Button>
                                </DialogTrigger>
                                <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
                                  <DialogHeader>
                                    <DialogTitle>Ticket Details</DialogTitle>
                                    <DialogDescription>
                                      Complete information for ticket{' '}
                                      {ticket.serial_number as string}
                                    </DialogDescription>
                                  </DialogHeader>
                                  <TicketDetails ticket={ticket as unknown as Ticket} />
                                </DialogContent>
                              </Dialog>
                            </TableCell>
                          </TableRow>
                        </HoverCardTrigger>
                        <HoverCardContent className="w-80" side="top" align="center" sideOffset={8}>
                          <TicketPreview ticket={ticket as unknown as Ticket} />
                        </HoverCardContent>
                      </HoverCard>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Restart Draw Dialog */}
      <Dialog open={restartDialogOpen} onOpenChange={setRestartDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Restart Draw</DialogTitle>
            <DialogDescription>
              This will restart the draw execution process. All progress will be saved.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>Reason for Restart</Label>
              <Textarea
                value={restartReason}
                onChange={e => setRestartReason(e.target.value)}
                placeholder="Provide a reason for restarting this draw..."
                className="mt-2"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRestartDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={() =>
                restartDrawMutation.mutate({
                  reason: restartReason,
                })
              }
              disabled={!restartReason.trim() || restartDrawMutation.isPending}
            >
              Restart Draw
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Machine Numbers Dialog */}
      <Dialog open={machineNumbersDialogOpen} onOpenChange={setMachineNumbersDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add Machine Numbers</DialogTitle>
            <DialogDescription>
              Enter the 5 machine numbers (1-90). These are cosmetic and will be displayed alongside
              the winning numbers.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>Machine Numbers</Label>
              <div className="mt-2">
                <NumberInputSlots
                  slotCount={5}
                  min={1}
                  max={90}
                  value={machineNumbers}
                  onChange={numbers => {
                    setMachineNumbers(numbers)
                    setMachineNumbersErrors(false)
                    setMachineNumbersDuplicates(false)
                  }}
                />
              </div>
              {machineNumbersErrors && (
                <p className="text-sm text-red-600 mt-2">
                  Please fill all 5 machine numbers (1-90)
                </p>
              )}
              {machineNumbersDuplicates && (
                <p className="text-sm text-red-600 mt-2">
                  Machine numbers must be unique (no duplicates allowed)
                </p>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setMachineNumbersDialogOpen(false)
                setMachineNumbers([])
                setMachineNumbersErrors(false)
                setMachineNumbersDuplicates(false)
              }}
            >
              Cancel
            </Button>
            <Button
              onClick={handleSaveMachineNumbers}
              disabled={updateMachineNumbersMutation.isPending}
            >
              {updateMachineNumbersMutation.isPending ? 'Saving...' : 'Save Machine Numbers'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default DrawDetails
