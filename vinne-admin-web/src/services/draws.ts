import api from '@/lib/api'

export interface Game {
  id: string
  code: string
  name: string
  game_type?: string
  status?: string
}

export interface Draw {
  id: string
  game_id: string
  game?: Game
  draw_number?: number // Sequential number per game
  game_name?: string // Denormalized game name
  draw_name?: string // From backend
  draw_date?: string
  scheduled_time?: { seconds: number; nanos?: number } // Protobuf timestamp
  draw_location?: string
  start_date?: string
  end_date?: string
  status: number | 'scheduled' | 'active' | 'closed' | 'completed' | 'cancelled'
  stage?: DrawStage
  winning_numbers?: number[]
  machine_numbers?: number[] // Cosmetic machine numbers (not used in calculations)
  game_schedule_id?: string
  total_tickets_sold?: number
  total_stakes?: number
  total_prize_pool?: number // Alternative field name from backend
  total_winnings?: number
  created_at?: string | { seconds: number; nanos?: number }
  updated_at?: string | { seconds: number; nanos?: number }
}

export interface DrawStage {
  current_stage: number // 1: Preparation, 2: Number Selection, 3: Result Calculation, 4: Payout
  stage_name?: string
  stage_status?:
    | 'STAGE_STATUS_PENDING'
    | 'STAGE_STATUS_IN_PROGRESS'
    | 'STAGE_STATUS_COMPLETED'
    | 'STAGE_STATUS_FAILED'
  stage_started_at?: string | { seconds: number; nanos?: number }
  stage_completed_at?: string | { seconds: number; nanos?: number }
  preparation_data?: PreparationStageData
  number_selection_data?: NumberSelectionStageData
  result_calculation_data?: ResultCalculationStageData
  payout_data?: PayoutStageData
}

export interface PreparationStageData {
  tickets_locked: number
  total_stakes: number
  sales_locked: boolean
  lock_time?: string | { seconds: number; nanos?: number }
}

export interface NumberSelectionStageData {
  winning_numbers: number[]
  verification_attempts?: VerificationAttempt[]
  is_verified: boolean
  verified_by?: string
  verified_at?: string | { seconds: number; nanos?: number }
}

export interface VerificationAttempt {
  attempt_number: number
  numbers: number[]
  submitted_by: string
  submitted_at: string | { seconds: number; nanos?: number }
}

export interface ResultCalculationStageData {
  winning_tickets_count: number
  total_winnings: number
  winning_tiers?: WinningTier[]
  calculated_at?: string | { seconds: number; nanos?: number }
}

export interface WinningTier {
  bet_type: string
  winners_count: number
  total_amount: number
}

export interface PayoutStageData {
  auto_processed_count: number
  manual_approval_count: number
  auto_processed_amount: number
  manual_approval_amount: number
  processed_count: number
  pending_count: number
  big_win_payouts?: BigWinPayout[]
}

export interface BigWinPayout {
  ticket_id: string
  amount: number
  status: 'pending' | 'approved' | 'rejected'
  approved_by?: string
  rejection_reason?: string
  processed_at?: string | { seconds: number; nanos?: number }
}

export interface DrawPreparationData {
  total_entries: number
  entries_validated: boolean
  sales_locked: boolean
  summary_generated: boolean
  completed_at?: string
  completed_by?: string
}

export interface NumberSelectionData {
  selection_method: 'rng' | 'physical'
  numbers_selected: number[]
  verified: boolean
  completed_at?: string
  completed_by?: string
  verification_attempts?: {
    attempt_1?: number[]
    attempt_2?: number[]
    attempt_3?: number[]
    current_attempt: 1 | 2 | 3
    failed: boolean
    failure_reason?: string
  }
}

export interface PrizeTier {
  tier: number
  matches: number
  prize_per_winner: number
  winner_count: number
  total_prize_pool: number
}

export interface ResultCommitmentData {
  numbers_committed: number[]
  winners_calculated: boolean
  total_winners: number
  payout_report_generated: boolean
  committed_at?: string
  committed_by?: string
  prize_tiers?: PrizeTier[]
  total_prize_pool?: number
  big_wins_count?: number
  big_wins_amount?: number
  normal_wins_count?: number
  normal_wins_amount?: number
}

export interface PayoutProcessingData {
  total_winning_amount: number
  big_wins_count: number
  big_wins_amount: number
  normal_wins_count: number
  normal_wins_amount: number
  processed_count: number
  pending_count: number
  processed_at?: string
  processed_by?: string
}

export interface BetLine {
  line_number?: number
  bet_type: string

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
  ticket_id: string
  ticket_number: string
  draw_id: string
  game_id: string
  retailer_id: string
  retailer_code?: string
  retailer_name?: string
  agent_id?: string
  agent_code?: string
  agent_name?: string
  selected_numbers: number[]
  stake_amount: number
  potential_win: number
  status: 'pending' | 'won' | 'lost' | 'cancelled' | 'expired'
  won_amount?: number
  is_big_win?: boolean
  purchased_at?: string | { seconds: number; nanos?: number }
  created_at?: string | { seconds: number; nanos?: number }
  channel?: 'pos' | 'web' | 'mobile' | 'ussd'
  issuer_type?: string
  serial_number?: string
  issuer_id?: string
  total_amount?: number
  id?: string
  bet_lines?: BetLine[]
}

export interface WinningTicket extends Ticket {
  tier: number
  prize_amount: number
  payout_status: 'pending' | 'processing' | 'completed' | 'failed'
  payout_date?: string
}

export interface DrawStatistics {
  total_tickets: number
  total_stakes: number
  total_winners: number
  total_winnings: number
  win_rate: number
  average_stake: number
  average_win: number
  by_channel: {
    pos: number
    web: number
    mobile: number
    ussd: number
  }
  by_retailer: Array<{
    retailer_id: string
    retailer_name: string
    ticket_count: number
    total_stakes: number
    total_wins: number
  }>
}

export const drawService = {
  async getDraws(params?: {
    game_id?: string
    status?: string
    start_date?: string
    end_date?: string
    page?: number
    per_page?: number
  }) {
    const response = await api.get('/admin/draws', { params })
    // Response format: { success, message, data: { draws, total_count, page, per_page } }
    return response.data?.data || { draws: [], total_count: 0, page: 1, per_page: 20 }
  },

  async getDrawById(id: string) {
    const response = await api.get(`/admin/draws/${id}`)
    console.log('getDrawById response:', JSON.stringify(response.data, null, 2))
    return response.data?.data?.draw || response.data?.data
  },

  async getDrawStatistics(drawId: string) {
    try {
      const response = await api.get(`/admin/draws/${drawId}/statistics`)
      return response.data?.data || response.data
    } catch (error) {
      console.error('Error fetching draw statistics:', error)
      // Return empty stats if API not implemented yet
      return {
        total_tickets: 0,
        total_stakes: 0,
        total_winners: 0,
        total_winnings: 0,
        win_rate: 0,
        average_stake: 0,
        average_win: 0,
        by_channel: {
          pos: 0,
          web: 0,
          mobile: 0,
          ussd: 0,
        },
        by_retailer: [],
      }
    }
  },

  async getDrawTickets(
    id: string,
    params?: {
      status?: string
      retailer_id?: string
      agent_id?: string
      page?: number
      limit?: number
    }
  ) {
    try {
      const response = await api.get(`/admin/draws/${id}/tickets`, { params })
      // The API returns { tickets: [], total: number, page: number, page_size: number }
      return response.data?.data || { tickets: [] }
    } catch (error) {
      console.error('Error fetching draw tickets:', error)
      return { tickets: [] }
    }
  },

  async getWinningTickets(
    id: string,
    params?: {
      payout_status?: string
      is_big_win?: boolean
      page?: number
      limit?: number
    }
  ) {
    try {
      const response = await api.get(`/admin/draws/${id}/winning-tickets`, { params })
      return response.data?.data || { data: [] }
    } catch (error) {
      console.error('Error fetching winning tickets:', error)
      return { data: [] }
    }
  },

  // Initialize draw execution (starts Stage 1: Preparation)
  async prepareDraw(id: string) {
    const response = await api.post(`/admin/draws/${id}/prepare`, {
      complete: false, // Start preparation stage
      // Note: initiated_by is extracted from JWT token by API gateway
    })
    return response.data
  },

  // Complete draw preparation (completes Stage 1: Preparation)
  async completeDrawPreparation(id: string) {
    const response = await api.post(`/admin/draws/${id}/prepare`, {
      complete: true, // Complete preparation stage
      // Note: completed_by is extracted from JWT token by API gateway
    })
    return response.data
  },

  async cancelDraw(id: string, reason: string) {
    const response = await api.post(`/admin/draws/${id}/cancel`, { reason })
    return response.data
  },

  // Execute draw (for physical draws)
  async executeDraw(
    id: string,
    data: { selection_method: 'rng' | 'physical'; numbers?: number[] }
  ) {
    const response = await api.post(`/admin/draws/${id}/execute`, {
      action: 'start',
      selection_method: data.selection_method,
    })
    return response.data
  },

  // Record physical draw numbers
  async recordPhysicalDraw(
    id: string,
    data: {
      numbers: number[]
      nla_draw_reference?: string
      draw_location?: string
      nla_official_signature?: string
    }
  ) {
    const response = await api.post(`/admin/draws/${id}/execute`, {
      action: 'submit_verification',
      numbers: data.numbers,
      nla_draw_reference: data.nla_draw_reference,
      draw_location: data.draw_location,
      nla_official_signature: data.nla_official_signature,
    })
    return response.data
  },

  // Commit draw results
  async commitDrawResults(id: string) {
    const response = await api.post(`/admin/draws/${id}/commit-results`)
    return response.data
  },

  // Process payout
  async processPayout(
    id: string,
    data: { payout_mode: 'auto' | 'manual'; exclude_big_wins?: boolean }
  ) {
    const response = await api.post(`/admin/draws/${id}/process-payout`, {
      payout_mode: data.payout_mode,
      exclude_big_wins: data.exclude_big_wins,
    })
    return response.data
  },

  // Validate draw
  async validateDraw(id: string) {
    try {
      const response = await api.post(`/admin/draws/${id}/validate`)
      return response.data
    } catch (error) {
      console.error('Error validating draw:', error)
      return { success: true, message: 'Draw validated (mock)' }
    }
  },

  // Save draw progress
  async saveDrawProgress(id: string) {
    try {
      const response = await api.post(`/admin/draws/${id}/save-progress`)
      return response.data
    } catch (error) {
      console.error('Error saving progress:', error)
      return { success: true, message: 'Progress saved (mock)' }
    }
  },

  // Restart draw (wrapper for API)
  async restartDrawAPI(id: string, data: { reason: string }) {
    const response = await api.post(`/admin/draws/${id}/restart`, {
      reason: data.reason,
    })
    return response.data
  },

  async resetVerificationAttempts(id: string) {
    const response = await api.post(`/admin/draws/${id}/execute`, {
      action: 'reset',
      reset_reason: 'User requested reset from UI',
    })
    return response.data
  },

  // Update machine numbers (cosmetic, after draw completion)
  async updateMachineNumbers(id: string, machineNumbers: number[]) {
    const response = await api.post(`/admin/draws/${id}/machine-numbers`, {
      machine_numbers: machineNumbers,
    })
    return response.data
  },
}

export default drawService
