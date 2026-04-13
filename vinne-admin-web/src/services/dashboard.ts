import api from '@/lib/api'

export interface DashboardMetrics {
  total_sales: number
  total_tickets: number
  active_games: number
  total_users: number
  today_sales: number
  today_tickets: number
  pending_payouts: number
  system_alerts: number
  revenue_trend: number[]
  sales_by_game: Record<string, number>
}

export interface Transaction {
  id: string
  type: string
  amount: number
  game: string
  time: string
  status: string
}

export interface DailyMetricsResponse {
  date: string
  metrics: {
    gross_revenue: {
      amount: number
      amount_ghs: number
      change_percentage: number
      previous_amount: number
      previous_amount_ghs: number
    }
    tickets: {
      count: number
      change_percentage: number
      previous_count: number
    }
    payouts: {
      amount: number
      amount_ghs: number
      change_percentage: number
      previous_amount: number
      previous_amount_ghs: number
    }
    win_rate: {
      percentage: number
      winning_tickets: number
      total_tickets: number
    }
    // New metrics for enhanced dashboard
    stakes: {
      count: number
      change_percentage: number
      previous_count: number
    }
    stakes_amount: {
      amount: number
      amount_ghs: number
      change_percentage: number
      previous_amount: number
      previous_amount_ghs: number
    }
    paid_tickets: {
      count: number
      change_percentage: number
      previous_count: number
    }
    payments_amount: {
      amount: number
      amount_ghs: number
      change_percentage: number
      previous_amount: number
      previous_amount_ghs: number
    }
    unpaid_tickets: {
      count: number
      change_percentage: number
      previous_count: number
    }
    unpaid_amount: {
      amount: number
      amount_ghs: number
      change_percentage: number
      previous_amount: number
      previous_amount_ghs: number
    }
    commissions?: {
      amount: number
      amount_ghs: number
      change_percentage: number
      previous_amount: number
      previous_amount_ghs: number
    }
    retailers?: {
      count: number
    }
  }
}

export interface MonthlyDataPoint {
  month: string
  year: number
  revenue: number
  revenue_ghs: number
  tickets: number
  payouts: number
  payouts_ghs: number
}

export interface MonthlyMetricsResponse {
  data: MonthlyDataPoint[]
}

export interface AgentPerformance {
  id: string
  agent_code: string
  name: string
  revenue: number
  tickets: number
  retailer_count: number
}

export interface TopPerformingAgentsResponse {
  agents: AgentPerformance[]
  period: string
  date: string
}

export const dashboardService = {
  async getMetrics(): Promise<DashboardMetrics> {
    const response = await api.get('/admin/dashboard/metrics')
    return response.data.data
  },

  async getRecentTransactions(): Promise<Transaction[]> {
    const response = await api.get('/admin/dashboard/transactions')
    return response.data.data
  },

  async getDailyMetrics(date?: string): Promise<DailyMetricsResponse> {
    const params = date ? `?date=${date}` : ''
    const response = await api.get(`/admin/analytics/daily-metrics${params}`)
    return response.data.data
  },

  async getMonthlyMetrics(months: number = 6): Promise<MonthlyMetricsResponse> {
    const response = await api.get(`/admin/analytics/monthly-metrics?months=${months}`)
    return response.data.data
  },

  async getTopPerformingAgents(
    options: {
      date?: string
      period?: string
      limit?: number
    } = {}
  ): Promise<TopPerformingAgentsResponse> {
    const params = new URLSearchParams()
    if (options.date) params.append('date', options.date)
    if (options.period) params.append('period', options.period)
    if (options.limit) params.append('limit', options.limit.toString())

    const queryString = params.toString()
    const response = await api.get(
      `/admin/analytics/top-performing-agents${queryString ? `?${queryString}` : ''}`
    )
    return response.data.data
  },
}
