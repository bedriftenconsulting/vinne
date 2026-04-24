export type TicketTypeLike = {
  serial_number?: string
  ticket_number?: string
  issuer_type?: string
  channel?: string
  game_type?: string
  type?: string
}

export const formatTicketType = (rawType?: string): string => {
  if (!rawType) return 'Draw Entry'

  const normalized = rawType.trim().toUpperCase()
  const ticketTypeMap: Record<string, string> = {
    ACCESS_PASS: 'Access Pass',
    DRAW_ENTRY: 'Draw Entry',
  }

  return ticketTypeMap[normalized] || rawType
}

export const getTicketTypeLabel = (ticket: TicketTypeLike): string => {
  const serialNumber = String(ticket.serial_number || ticket.ticket_number || '')
    .trim()
    .toUpperCase()
  const channel = String(ticket.issuer_type || ticket.channel || '')
    .trim()
    .toLowerCase()
  const rawType = String(ticket.game_type || ticket.type || '').trim()

  if (channel === 'ussd' && serialNumber.startsWith('CP-ACC')) {
    return 'Access Pass'
  }

  if (rawType) {
    return formatTicketType(rawType)
  }

  return serialNumber.startsWith('CP-ACC') ? 'Access Pass' : 'Draw Entry'
}
