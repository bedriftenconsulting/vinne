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
  game_name: string
  draw_number: string
  draw_name: string
  start_date: string
  end_date: string
  draw_date: string
  scheduled_time: string
  status: 'scheduled' | 'active' | 'closed' | 'completed' | 'cancelled'
  stage?: DrawStage
  winning_numbers?: number[]
  total_tickets_sold: number
  total_stakes: number
  total_winnings?: number
  created_at: string
  updated_at: string
}

export interface DrawStage {
  current_stage: 1 | 2 | 3 | 4
  stage_1_completed: boolean
  stage_2_completed: boolean
  stage_3_completed: boolean
  stage_4_completed: boolean
  stage_1_data?: DrawPreparationData
  stage_2_data?: NumberSelectionData
  stage_3_data?: ResultCommitmentData
  stage_4_data?: PayoutProcessingData
  can_restart: boolean
  restart_count: number
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
}

export interface ResultCommitmentData {
  numbers_committed: number[]
  winners_calculated: boolean
  total_winners: number
  payout_report_generated: boolean
  committed_at?: string
  committed_by?: string
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

export interface Ticket {
  id: string
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
  purchased_at: string
  channel: 'pos' | 'web' | 'mobile' | 'ussd'
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

class DrawService {
  async getDraws(params?: {
    game_id?: string
    status?: string
    from_date?: string
    to_date?: string
    page?: number
    limit?: number
  }) {
    const response = await api.get('/admin/draws', { params })
    return response.data
  }

  async getDrawById(id: string) {
    const response = await api.get(`/admin/draws/${id}`)
    return response.data
  }

  async getDrawStatistics(id: string) {
    const response = await api.get(`/admin/draws/${id}/statistics`)
    return response.data
  }

  async getDrawTickets(
    id: string,
    params?: {
      status?: string
      issuer_id?: string
      limit?: number
      offset?: number
    }
  ) {
    const response = await api.get(`/admin/draws/${id}/tickets`, { params })
    return response.data
  }

  async getDrawResults(id: string) {
    const response = await api.get(`/admin/draws/${id}/results`)
    return response.data
  }

  async getWinningNumbers(id: string) {
    const response = await api.get(`/admin/draws/${id}/winning-numbers`)
    return response.data
  }

  async validateDraw(id: string) {
    const response = await api.post(`/admin/draws/${id}/validate`)
    return response.data
  }

  // Draw execution - Stage 1: Preparation
  async prepareDraw(id: string) {
    const response = await api.post(`/admin/draws/${id}/prepare`)
    return response.data
  }

  // Draw execution - Stage 2: Number Selection/Draw Execution
  async executeDraw(
    id: string,
    data: {
      selection_method: 'rng' | 'physical'
      numbers?: number[]
    }
  ) {
    const response = await api.post(`/admin/draws/${id}/execute`, data)
    return response.data
  }

  async recordPhysicalDraw(
    id: string,
    data: {
      numbers: number[]
      nla_draw_reference?: string
      draw_location?: string
      nla_official_signature?: string
    }
  ) {
    const response = await api.post(`/admin/draws/${id}/record-physical`, data)
    return response.data
  }

  // Draw execution - Stage 3: Result Commitment
  async commitDrawResults(id: string) {
    const response = await api.post(`/admin/draws/${id}/commit-results`)
    return response.data
  }

  // Draw execution - Stage 4: Payout Processing
  async processPayout(
    id: string,
    data: {
      payout_mode: 'auto' | 'manual'
      exclude_big_wins?: boolean
    }
  ) {
    const response = await api.post(`/admin/draws/${id}/process-payout`, data)
    return response.data
  }

  // Draw management
  async saveDrawProgress(id: string) {
    const response = await api.post(`/admin/draws/${id}/save-progress`)
    return response.data
  }

  async restartDraw(
    id: string,
    data: {
      reason: string
    }
  ) {
    const response = await api.post(`/admin/draws/${id}/restart`, data)
    return response.data
  }

  async createDraw(data: {
    game_id: string
    draw_number: number
    game_name: string
    draw_name: string
    scheduled_time: string
    draw_location?: string
  }) {
    const response = await api.post('/admin/draws', data)
    return response.data
  }

  async updateDraw(
    id: string,
    data: Partial<{
      draw_name: string
      scheduled_time: string
      draw_location: string
      status: string
    }>
  ) {
    const response = await api.put(`/admin/draws/${id}`, data)
    return response.data
  }

  async deleteDraw(id: string) {
    const response = await api.delete(`/admin/draws/${id}`)
    return response.data
  }
}

export default new DrawService()
