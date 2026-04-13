import { useQuery } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { ticketService } from '@/services/tickets'
import { Ticket as TicketIcon, ChevronLeft, ChevronRight, Eye } from 'lucide-react'
import { useState } from 'react'
import type { Ticket, BetLine } from '@/services/tickets'
import { formatCurrency } from '@/lib/utils'
import { PermCombinationViewer } from '@/components/PermCombinationViewer'
import { isPermBet, isBankerBet, getBetLineNumbers, getBetLineAmount } from '@/lib/bet-utils'

interface PlayerTicketsCardProps {
  playerId: string
}

export function PlayerTicketsCard({ playerId }: PlayerTicketsCardProps) {
  const [page, setPage] = useState(1)
  const pageSize = 10

  const { data: ticketsData, isLoading } = useQuery({
    queryKey: ['player-tickets', playerId, page],
    queryFn: () =>
      ticketService.getTickets({
        player_id: playerId,
        page,
        limit: pageSize,
      }),
  })

  const getStatusBadgeVariant = (status: string) => {
    switch (status.toLowerCase()) {
      case 'active':
      case 'issued':
        return 'default'
      case 'won':
        return 'default'
      case 'lost':
        return 'secondary'
      case 'cancelled':
      case 'expired':
        return 'destructive'
      default:
        return 'secondary'
    }
  }

  const formatAmount = (amount: number) => {
    return new Intl.NumberFormat('en-GH', {
      style: 'currency',
      currency: 'GHS',
      minimumFractionDigits: 2,
    }).format(amount / 100)
  }

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('en-GH', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

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
          <h3 className="font-bold text-base">{ticket.game_name || 'LOTTERY GAME'}</h3>
          <p className="text-xs text-gray-600">Official Ticket</p>
        </div>

        {/* Ticket Details */}
        <div className="space-y-2 mb-4">
          <div className="flex justify-between">
            <span>TICKET NO:</span>
            <span className="font-bold">{ticket.serial_number}</span>
          </div>
          <div className="flex justify-between">
            <span>STAKE:</span>
            <span className="font-bold">{formatAmount(ticket.total_amount)}</span>
          </div>
          <div className="flex justify-between">
            <span>DATE:</span>
            <span>{new Date(ticket.created_at).toLocaleDateString('en-GB')}</span>
          </div>
          <div className="flex justify-between">
            <span>TIME:</span>
            <span>
              {new Date(ticket.created_at).toLocaleTimeString('en-GB', { hour12: false })}
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
                      <div className="flex gap-1 mt-0.5 justify-center flex-wrap">
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
                      <div className="flex gap-1 mt-0.5 justify-center flex-wrap">
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
                      <div className="flex gap-1 mt-0.5 justify-center flex-wrap">
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

        {/* Status */}
        <div className="border-t border-dashed border-gray-300 pt-4 mb-4">
          <div className="flex justify-between items-center">
            <span>STATUS:</span>
            <Badge variant={ticket.status === 'won' ? 'default' : 'secondary'} className="text-xs">
              {ticket.status}
            </Badge>
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
      <div className="border-t pt-6 space-y-4">
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
                <td className="p-3 font-mono">{ticket.id}</td>
              </tr>
              <tr className="border-b">
                <td className="p-3 text-muted-foreground">Serial Number</td>
                <td className="p-3 font-mono font-bold">{ticket.serial_number}</td>
              </tr>
              <tr className="border-b">
                <td className="p-3 text-muted-foreground">Game</td>
                <td className="p-3">
                  <Badge variant="outline" className="text-purple-600">
                    {ticket.game_name}
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
                            <div className="flex gap-1 mt-1 flex-wrap">
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
                            <div className="flex gap-1 mt-1 flex-wrap">
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
                            <div className="flex gap-1 mt-1 flex-wrap">
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
                <td className="p-3 font-semibold">{formatCurrency(ticket.total_amount)}</td>
              </tr>
              <tr className="border-b">
                <td className="p-3 text-muted-foreground">Status</td>
                <td className="p-3">
                  <Badge variant={ticket.status === 'won' ? 'default' : 'secondary'}>
                    {ticket.status}
                  </Badge>
                </td>
              </tr>
              <tr>
                <td className="p-3 text-muted-foreground">Purchased At</td>
                <td className="p-3">{formatDate(ticket.created_at)}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <TicketIcon className="h-5 w-5" />
            Tickets
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="animate-pulse space-y-3">
            <div className="h-12 bg-muted rounded" />
            <div className="h-12 bg-muted rounded" />
            <div className="h-12 bg-muted rounded" />
          </div>
        </CardContent>
      </Card>
    )
  }

  const tickets = ticketsData?.data || []
  const totalPages = ticketsData?.total_pages || 1

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <TicketIcon className="h-5 w-5" />
          Tickets
        </CardTitle>
      </CardHeader>
      <CardContent>
        {tickets.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            <TicketIcon className="h-12 w-12 mx-auto mb-4 opacity-50" />
            <p>No tickets found</p>
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="text-xs">Serial Number</TableHead>
                    <TableHead className="text-xs">Game</TableHead>
                    <TableHead className="text-xs">Amount</TableHead>
                    <TableHead className="text-xs">Status</TableHead>
                    <TableHead className="text-xs">Date</TableHead>
                    <TableHead className="text-xs">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tickets.map(ticket => (
                    <TableRow key={ticket.id}>
                      <TableCell className="font-mono text-xs py-2">
                        {ticket.serial_number}
                      </TableCell>
                      <TableCell className="text-xs py-2">
                        {ticket.game_name || ticket.game_code}
                      </TableCell>
                      <TableCell className="text-xs py-2">
                        {formatAmount(ticket.total_amount)}
                      </TableCell>
                      <TableCell className="py-2">
                        <Badge variant={getStatusBadgeVariant(ticket.status)} className="text-xs">
                          {ticket.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs py-2 whitespace-nowrap">
                        {formatDate(ticket.created_at)}
                      </TableCell>
                      <TableCell className="py-2">
                        <Dialog>
                          <DialogTrigger asChild>
                            <Button variant="ghost" size="sm">
                              <Eye className="h-4 w-4" />
                            </Button>
                          </DialogTrigger>
                          <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
                            <DialogHeader>
                              <DialogTitle>Ticket Details</DialogTitle>
                              <DialogDescription>
                                Complete information for ticket {ticket.serial_number}
                              </DialogDescription>
                            </DialogHeader>
                            <TicketDetails ticket={ticket} />
                          </DialogContent>
                        </Dialog>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>

            {totalPages > 1 && (
              <div className="flex items-center justify-between mt-4 pt-4 border-t">
                <div className="text-sm text-muted-foreground">
                  Page {page} of {totalPages}
                </div>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage(p => Math.max(1, p - 1))}
                    disabled={page <= 1}
                  >
                    <ChevronLeft className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                    disabled={page >= totalPages}
                  >
                    <ChevronRight className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
