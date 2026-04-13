import api from '@/lib/api'

// Bet type definition
export interface BetType {
  id: string
  name: string
  enabled: boolean
  multiplier: number
}

// Game types based on backend models
export interface Game {
  id: string
  code: string
  name: string
  description?: string
  // New format-based fields
  game_category?: 'national' | 'private'
  format?: '5_by_90' | '5_by_30' | '6_by_90' | '6_by_49' | '4_by_90' | '3_by_90'
  bet_types?: BetType[]
  // Backend fields (primary)
  game_type?: string
  game_format?: string
  base_price?: number
  // Legacy frontend fields (fallback)
  type?:
    | 'national'
    | 'private'
    | 'special'
    | '5_by_90'
    | 'direct'
    | 'perm'
    | 'banker'
    | 'super_6'
    | 'midweek'
    | 'aseda'
    | 'bonanza'
    | 'noon_rush'
    | 'evening'
  ticket_price?: number
  status:
    | 'Draft'
    | 'PendingApproval'
    | 'Active'
    | 'Suspended'
    | 'Archived'
    | 'draft'
    | 'pending_approval'
    | 'active'
    | 'suspended'
    | 'archived'
  number_range_min: number
  number_range_max: number
  selection_count: number
  draw_frequency: 'daily' | 'weekly' | 'bi_weekly' | 'monthly' | 'special'
  draw_days?: string[]
  draw_time?: string
  sales_cutoff_minutes: number
  max_tickets_per_player: number
  multi_draw_enabled: boolean
  max_multi_draws?: number
  max_draws_advance?: number
  min_stake?: number
  max_stake?: number
  organizer?: 'nla' | 'rand_lottery'
  logo_url?: string
  brand_color?: string
  version: number
  created_at: string
  updated_at: string
}

export interface PrizeStructure {
  id: string
  game_id: string
  prize_pool_percentage: number
  rollover_enabled: boolean
  rollover_percentage?: number
  jackpot_cap?: number
  guaranteed_minimum?: number
  created_at: string
  updated_at: string
}

export interface PrizeTier {
  id: string
  prize_structure_id: string
  tier: number
  matches_required: number
  bonus_matches_required?: number
  prize_type: 'Fixed' | 'Percentage' | 'PariMutuel'
  fixed_amount?: number
  percentage?: number
  estimated_value?: number
  created_at: string
  updated_at: string
}

export interface DrawSchedule {
  id: string
  game_id: string
  draw_date: string
  draw_time: string
  sales_cutoff_time: string
  status: 'Scheduled' | 'InProgress' | 'Completed' | 'Cancelled'
  is_special: boolean
  special_name?: string
  created_at: string
  updated_at: string
}

export interface GameSchedule {
  id: string
  game_id: string
  game_name?: string
  draw_id?: string
  scheduled_start: string
  scheduled_end: string
  scheduled_draw: string
  frequency: string
  is_active: boolean
  status?: string
  notes?: string
}

export interface UpdateScheduledGameRequest {
  scheduled_end?: string
  scheduled_draw?: string
  status?: 'SCHEDULED' | 'IN_PROGRESS' | 'COMPLETED' | 'CANCELLED' | 'FAILED'
  is_active?: boolean
  notes?: string
}

export interface WeeklyScheduleResponse {
  schedules: GameSchedule[]
  schedules_created: number
  success: boolean
  message: string
}

export interface GameApproval {
  id: string
  game_id: string
  approval_stage: 'SUBMITTED' | 'FIRST_APPROVED' | 'APPROVED' | 'REJECTED'
  approved_by?: string
  approved_date?: string
  rejection_reason?: string
  notes?: string
  change_type?: string
  change_description?: string
  created_at: string
  updated_at: string
  // Extended fields from the backend
  game?: Game
  approver_name?: string
  requester_name?: string
  requested_by?: string
  requested_at?: string
  first_approver_id?: string
}

export interface CreateGameRequest {
  code: string
  name: string
  description?: string
  game_category: 'national' | 'private'
  format: '5_by_90' | '5_by_30' | '6_by_90' | '6_by_49' | '4_by_90' | '3_by_90'
  bet_types: BetType[]
  number_range_min: number
  number_range_max: number
  selection_count: number
  draw_frequency: 'daily' | 'weekly' | 'bi_weekly' | 'monthly' | 'special'
  draw_days?: string[]
  draw_time?: string
  sales_cutoff_minutes: number
  base_price: number
  max_tickets_per_player: number
  max_tickets_per_transaction: number
  multi_draw_enabled: boolean
  max_draws_advance?: number
  status?: 'Draft' | 'PendingApproval' | 'Active' | 'Suspended' | 'Archived'
  bonus_number_enabled?: boolean
  bonus_range_min?: number
  bonus_range_max?: number
  bonus_count?: number
  start_time?: string
  end_time?: string
  start_date?: string
  end_date?: string
  // Legacy fields for backward compatibility
  organizer?: 'nla' | 'rand_lottery'
  game_format?: string
  game_type?: string
  min_stake?: number
  max_stake?: number
  weekly_schedule?: boolean
}

export interface UpdateGameRequest {
  name?: string
  description?: string
  bet_types?: BetType[]
  draw_frequency?: 'daily' | 'weekly' | 'bi_weekly' | 'monthly' | 'special'
  draw_days?: string[]
  draw_time?: string
  sales_cutoff_minutes?: number
  min_stake?: number
  max_stake?: number
  base_price?: number
  max_tickets_per_player?: number
  multi_draw_enabled?: boolean
  max_draws_advance?: number
  weekly_schedule?: boolean
}

export interface CreatePrizeStructureRequest {
  game_id: string
  prize_pool_percentage: number
  rollover_enabled: boolean
  rollover_percentage?: number
  jackpot_cap?: number
  guaranteed_minimum?: number
  tiers: {
    tier: number
    matches_required: number
    bonus_matches_required?: number
    prize_type: 'Fixed' | 'Percentage' | 'PariMutuel'
    fixed_amount?: number
    percentage?: number
    estimated_value?: number
  }[]
}

export interface ScheduleDrawRequest {
  scheduled_date: string
  scheduled_time: string
  is_special?: boolean
  special_name?: string
}

export interface ApprovalRequest {
  game_id: string
  change_type: 'Create' | 'Update' | 'Activate' | 'Suspend'
  change_description: string
}

export interface ApprovalResponse {
  approval_id: string
  action: 'approve' | 'reject'
  rejection_reason?: string
}

class GameService {
  // Game CRUD operations
  async createGame(data: CreateGameRequest): Promise<Game> {
    const response = await api.post('/admin/games', data)
    // Handle nested response structure from API Gateway
    return response.data.data?.game || response.data.data || response.data
  }

  async getGames(page = 1, limit = 20): Promise<{ data: Game[]; total: number }> {
    const response = await api.get('/admin/games', {
      params: { page, limit },
    })
    // Handle nested response structure with games array
    const responseData = response.data.data || {}
    return {
      data: responseData.games || [],
      total: responseData.total || 0,
    }
  }

  async getGame(id: string): Promise<Game> {
    const response = await api.get(`/admin/games/${id}`)
    return response.data.data.game
  }

  async updateGame(id: string, data: UpdateGameRequest): Promise<Game> {
    const response = await api.put(`/admin/games/${id}`, data)
    return response.data.data
  }

  async deleteGame(id: string): Promise<void> {
    await api.delete(`/admin/games/${id}`)
  }

  async getActiveGames(): Promise<Game[]> {
    const response = await api.get('/games/active')
    return response.data.data
  }

  async activateGame(id: string): Promise<Game> {
    const response = await api.put(`/admin/games/${id}/status`, { status: 'Active' })
    return response.data.data
  }

  async suspendGame(id: string, reason: string): Promise<void> {
    await api.put(`/admin/games/${id}/status`, { status: 'Suspended', reason })
  }

  // Prize structure management
  async createPrizeStructure(
    gameId: string,
    data: CreatePrizeStructureRequest
  ): Promise<PrizeStructure> {
    const response = await api.post(`/admin/games/${gameId}/prize-structure`, data)
    return response.data.data
  }

  async getPrizeStructure(
    gameId: string
  ): Promise<{ structure: PrizeStructure; tiers: PrizeTier[] }> {
    const response = await api.get(`/admin/games/${gameId}/prize-structure`)
    return response.data.data
  }

  // Draw scheduling
  async scheduleDraw(gameId: string, data: ScheduleDrawRequest): Promise<DrawSchedule> {
    const response = await api.post(`/admin/games/${gameId}/draws`, data)
    return response.data.data
  }

  async getUpcomingDraws(gameId: string): Promise<DrawSchedule[]> {
    const response = await api.get(`/admin/games/${gameId}/draws/upcoming`)
    return response.data.data
  }

  // Approval workflow
  async submitForApproval(gameId: string, notes?: string): Promise<void> {
    await api.post(`/admin/games/${gameId}/submit-approval`, { notes })
  }

  async approveGame(gameId: string, notes?: string): Promise<void> {
    await api.post(`/admin/games/${gameId}/approve`, { notes })
  }

  async rejectGame(gameId: string, reason: string): Promise<void> {
    await api.post(`/admin/games/${gameId}/reject`, { reason })
  }

  async getApprovalStatus(gameId: string): Promise<GameApproval> {
    const response = await api.get(`/admin/games/${gameId}/approval-status`)
    return response.data.data
  }

  async getPendingApprovals(
    page = 1,
    limit = 20
  ): Promise<{ approvals: GameApproval[]; total: number }> {
    const response = await api.get('/admin/games/pending-approvals', {
      params: { page, limit },
    })
    const responseData = response.data.data || {}
    return {
      approvals: responseData.approvals || [],
      total: responseData.total || 0,
    }
  }

  // Legacy approval methods (kept for backward compatibility)
  async requestApproval(gameId: string, data: ApprovalRequest): Promise<GameApproval> {
    const response = await api.post(`/admin/games/${gameId}/approval`, data)
    return response.data.data
  }

  async processApproval(approvalId: string, data: ApprovalResponse): Promise<void> {
    await api.post(`/admin/approvals/${approvalId}/process`, data)
  }

  // Prize Structure Management
  async updatePrizeStructure(id: string, data: Partial<PrizeStructure>): Promise<PrizeStructure> {
    const response = await api.put(`/admin/prize-structures/${id}`, data)
    return response.data.data
  }

  async getPrizeTiers(prizeStructureId: string): Promise<PrizeTier[]> {
    const response = await api.get(`/admin/prize-structures/${prizeStructureId}/tiers`)
    return response.data.data || []
  }

  async createPrizeTier(
    prizeStructureId: string,
    data: Omit<PrizeTier, 'id' | 'created_at' | 'updated_at'>
  ): Promise<PrizeTier> {
    const response = await api.post(`/admin/prize-structures/${prizeStructureId}/tiers`, data)
    return response.data.data
  }

  async updatePrizeTier(id: string, data: Partial<PrizeTier>): Promise<PrizeTier> {
    const response = await api.put(`/admin/prize-tiers/${id}`, data)
    return response.data.data
  }

  async deletePrizeTier(id: string): Promise<void> {
    await api.delete(`/admin/prize-tiers/${id}`)
  }

  // Draw Schedule Management
  async getDrawSchedules(params?: { game_id?: string }): Promise<DrawSchedule[]> {
    const response = await api.get('/admin/draw-schedules', { params })
    return response.data.data || []
  }

  async createDrawSchedule(
    data: Omit<DrawSchedule, 'id' | 'created_at' | 'updated_at'>
  ): Promise<DrawSchedule> {
    const response = await api.post('/admin/draw-schedules', data)
    return response.data.data
  }

  async updateDrawSchedule(id: string, data: Partial<DrawSchedule>): Promise<DrawSchedule> {
    const response = await api.put(`/admin/draw-schedules/${id}`, data)
    return response.data.data
  }

  async deleteDrawSchedule(id: string): Promise<void> {
    await api.delete(`/admin/draw-schedules/${id}`)
  }

  // Weekly game scheduling methods
  async generateWeeklySchedule(weekStart?: string): Promise<WeeklyScheduleResponse> {
    const data = weekStart ? { week_start: weekStart } : {}
    const response = await api.post('/admin/scheduling/weekly/generate', data)
    return response.data.data || response.data
  }

  async getWeeklySchedule(weekStart?: string): Promise<GameSchedule[]> {
    // If no week_start is provided, use a very early date to get all schedules
    // The backend requires week_start, so we can't pass undefined
    const defaultWeekStart = weekStart || '2020-01-01'
    const response = await api.get('/admin/scheduling/weekly', {
      params: { week_start: defaultWeekStart },
    })
    return response.data.data?.schedules || response.data.schedules || []
  }

  async clearWeeklySchedule(
    weekStart?: string
  ): Promise<{ success: boolean; message: string; schedules_deleted: number }> {
    const params = weekStart ? { week_start: weekStart } : undefined
    const response = await api.delete('/admin/scheduling/weekly/clear', { params })
    return response.data.data || response.data
  }

  async updateScheduledGame(
    scheduleId: string,
    data: UpdateScheduledGameRequest
  ): Promise<GameSchedule> {
    const response = await api.put(`/admin/games/schedules/${scheduleId}`, data)
    return response.data.data?.schedule || response.data.data || response.data
  }

  async getScheduleById(scheduleId: string): Promise<GameSchedule> {
    const response = await api.get(`/admin/scheduling/schedules/${scheduleId}`)
    return response.data.data?.schedule || response.data.data || response.data
  }

  // Game Logo and Branding
  async uploadGameLogo(
    gameId: string,
    file: File,
    brandColor?: string
  ): Promise<{ logo_url: string; cdn_url?: string; brand_color?: string }> {
    const formData = new FormData()
    formData.append('file', file)
    if (brandColor) {
      formData.append('brand_color', brandColor)
    }

    const response = await api.post(`/admin/games/${gameId}/logo`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
    })
    return response.data.data
  }

  async deleteGameLogo(gameId: string): Promise<void> {
    await api.delete(`/admin/games/${gameId}/logo`)
  }

  async updateBrandColor(gameId: string, brandColor: string): Promise<Game> {
    const response = await api.patch(`/admin/games/${gameId}/brand-color`, {
      brand_color: brandColor,
    })
    return response.data.data
  }
}

export const gameService = new GameService()
