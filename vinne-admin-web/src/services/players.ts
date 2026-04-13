import api from '@/lib/api'

export interface Player {
  id: string
  phone_number: string
  email: string
  first_name: string
  last_name: string
  national_id: string
  mobile_money_phone: string
  status: 'ACTIVE' | 'SUSPENDED' | 'BANNED'
  date_of_birth?: string
  created_at: string
  last_login: string
  wallet_id?: string
}

export interface WalletBalance {
  balance: number
  pending_balance: number
  currency: string
}

export interface SearchPlayersParams {
  query?: string
  page?: number
  per_page?: number
  status?: 'ACTIVE' | 'SUSPENDED' | 'BANNED'
}

export interface SuspendPlayerRequest {
  reason: string
  suspended_by: string
}

export interface ActivatePlayerRequest {
  activated_by: string
}

// Standard API response format
export interface ApiResponse<T> {
  success?: boolean
  message?: string
  player?: T
  players?: T[]
  total?: number
  page?: number
  per_page?: number
}

export interface WalletApiResponse {
  balance: number
  pending_balance: number
  currency: string
}

export const playerService = {
  // Search/List players
  async searchPlayers(params: SearchPlayersParams = {}): Promise<{
    players: Player[]
    total: number
    page: number
    per_page: number
  }> {
    const queryParams = new URLSearchParams()

    if (params.query) queryParams.append('query', params.query)
    if (params.page) queryParams.append('page', params.page.toString())
    if (params.per_page) queryParams.append('per_page', params.per_page.toString())
    if (params.status) queryParams.append('status', params.status)

    const response = await api.get<ApiResponse<Player>>(
      `/admin/players/search?${queryParams.toString()}`
    )

    return {
      players: response.data.players || [],
      total: response.data.total || 0,
      page: response.data.page || 1,
      per_page: response.data.per_page || 20,
    }
  },

  // Get player by ID
  async getPlayer(id: string): Promise<Player> {
    const response = await api.get<ApiResponse<Player>>(`/admin/players/${id}`)
    return response.data.player!
  },

  // Get player wallet balance
  async getPlayerWallet(playerId: string): Promise<WalletBalance> {
    const response = await api.get<WalletApiResponse>(`/admin/players/${playerId}/wallet/balance`)
    return {
      balance: response.data.balance,
      pending_balance: response.data.pending_balance,
      currency: response.data.currency,
    }
  },

  // Suspend player
  async suspendPlayer(id: string, data: SuspendPlayerRequest): Promise<void> {
    await api.post(`/admin/players/${id}/suspend`, data)
  },

  // Activate player
  async activatePlayer(id: string, data: ActivatePlayerRequest): Promise<void> {
    await api.post(`/admin/players/${id}/activate`, data)
  },
}
