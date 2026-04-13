import api from '@/lib/api'

export interface BetLine {
  line_number?: number
  bet_type: string // "DIRECT-1", "PERM-2", "PERM-3", "BANKER", "BANKER ALL", "BANKER AG", "AGAINST"

  // For DIRECT and PERM bets
  selected_numbers?: number[] // Player's chosen numbers (new format)

  // For BANKER and AGAINST bets
  banker?: number[]
  opposed?: number[]

  // For PERM and Banker bets (compact format)
  number_of_combinations?: number // C(n,r) - calculated value
  amount_per_combination?: number // Amount per combination in pesewas

  // Common fields
  total_amount?: number // Total bet amount in pesewas
  potential_win?: number // Potential winning amount in pesewas
}

export interface Ticket {
  id: string // proto field name
  serial_number: string
  schedule_id: string
  draw_id?: string
  game_code: string
  game_name: string
  player_id: string
  player_username?: string
  player_email?: string
  player_phone?: string
  bet_lines: BetLine[]
  selected_numbers: number[]
  banker_numbers?: number[] // For Banker bet types
  opposed_numbers?: number[] // For Banker Against bet types
  bonus_numbers?: number[]
  is_quick_pick: boolean
  stake_amount: number
  multiplier: number
  total_amount: number
  status: 'issued' | 'active' | 'won' | 'lost' | 'cancelled' | 'expired'
  purchase_date: string
  draw_date: string
  is_multi_draw: boolean
  number_of_draws: number
  qr_code?: string
  barcode?: string
  created_at: string
  updated_at: string
}

export interface TicketFilters {
  schedule_id?: string
  draw_id?: string
  game_code?: string
  player_id?: string // For backward compatibility, maps to issuer_id when issuer_type=player
  issuer_type?: string
  issuer_id?: string
  status?: string
  start_date?: string
  end_date?: string
  limit?: number
  page?: number
}

export interface ApiResponse<T> {
  success: boolean
  message: string
  data: T
  meta?: {
    request_id: string
    version: string
  }
  timestamp: string
}

export interface PaginatedApiResponse<T> {
  success: boolean
  message: string
  data: T[]
  pagination: {
    page: number
    page_size: number
    total_count: number
    total_pages: number
  }
  meta?: {
    request_id: string
    version: string
  }
  timestamp: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total_count: number
  page: number
  page_size: number
  total_pages: number
}

export const ticketService = {
  async getTickets(filters: TicketFilters = {}): Promise<PaginatedResponse<Ticket>> {
    const params = new URLSearchParams()

    if (filters.schedule_id) params.append('schedule_id', filters.schedule_id)
    if (filters.draw_id) params.append('draw_id', filters.draw_id)
    if (filters.game_code) params.append('game_code', filters.game_code)

    // Handle player_id for backward compatibility - maps to issuer_id with issuer_type=player
    if (filters.player_id) {
      params.append('issuer_type', 'player')
      params.append('issuer_id', filters.player_id)
    }

    // Direct issuer_type and issuer_id parameters (takes precedence over player_id)
    if (filters.issuer_type) params.append('issuer_type', filters.issuer_type)
    if (filters.issuer_id) params.append('issuer_id', filters.issuer_id)

    if (filters.status) params.append('status', filters.status)
    if (filters.start_date) params.append('start_date', filters.start_date)
    if (filters.end_date) params.append('end_date', filters.end_date)
    if (filters.limit) params.append('limit', filters.limit.toString())
    if (filters.page) params.append('page', filters.page.toString())

    const response = await api.get<{
      data: {
        tickets?: Ticket[]
        total?: number
        page?: number
        page_size?: number
      }
    }>(`/admin/tickets?${params.toString()}`)

    // Handle the actual API response structure which has tickets in data.data.tickets
    const ticketsData = response.data.data || {}

    return {
      data: ticketsData.tickets || [],
      total_count: ticketsData.total || 0,
      page: ticketsData.page || 1,
      page_size: ticketsData.page_size || 10,
      total_pages: Math.ceil((ticketsData.total || 0) / (ticketsData.page_size || 10)),
    }
  },

  async getTicket(id: string): Promise<Ticket> {
    const response = await api.get<ApiResponse<{ ticket: Ticket }>>(`/admin/tickets/${id}`)
    return (response.data.data as { ticket: Ticket }).ticket ?? (response.data.data as unknown as Ticket)
  },

  async getTicketBySerialNumber(serialNumber: string): Promise<Ticket> {
    const response = await api.get<ApiResponse<{ ticket: Ticket }>>(`/admin/tickets/serial/${serialNumber}`)
    return (response.data.data as { ticket: Ticket }).ticket ?? (response.data.data as unknown as Ticket)
  },

  async cancelTicket(id: string, reason: string): Promise<Ticket> {
    const response = await api.post<ApiResponse<Ticket>>(`/admin/tickets/${id}/cancel`, { reason })
    return response.data.data
  },
}
