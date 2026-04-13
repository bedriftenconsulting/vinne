import api from '@/lib/api'

export interface WalletBalance {
  agent_id?: string
  retailer_id?: string
  balance: number
  pending_balance: number
  available_balance: number
  last_updated: string
}

export interface WalletTransaction {
  id: string
  transaction_id?: string
  wallet_owner_id?: string
  wallet_owner_name?: string
  wallet_owner_code?: string
  wallet_type: string
  type: string
  amount: number
  balance_before: number
  balance_after: number
  reference?: string
  description?: string
  status: string
  created_at: string
  completed_at?: string
  reversed_at?: string
  metadata?: Record<string, unknown>
}

export interface TransactionHistoryResponse {
  transactions: WalletTransaction[]
  total_count: number
  page: number
  page_size: number
  has_more: boolean
}

export interface AllTransactionsFilters {
  transaction_types?: ('CREDIT' | 'DEBIT' | 'TRANSFER' | 'COMMISSION' | 'PAYOUT')[]
  wallet_types?: ('AGENT_STAKE' | 'RETAILER_STAKE' | 'RETAILER_WINNING')[]
  statuses?: ('PENDING' | 'COMPLETED' | 'FAILED' | 'REVERSED')[]
  start_date?: string // RFC3339 format
  end_date?: string // RFC3339 format
  search?: string
  page?: number
  page_size?: number
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

export interface TransactionStatistics {
  total_volume: number
  total_credits: number
  total_debits: number
  pending_amount: number
  pending_count: number
  completed_count: number
  failed_count: number
  credit_count: number
  debit_count: number
  transfer_count: number
  commission_count: number
  payout_count: number
}

export interface AllTransactionsResponse {
  transactions: WalletTransaction[]
  pagination: {
    page: number
    page_size: number
    total: number
    total_pages: number
    has_more: boolean
  }
  statistics?: TransactionStatistics
}

export interface CreditWalletResponse {
  success: boolean
  data: {
    transaction_id: string
    base_amount: number
    commission_amount: number
    gross_amount: number
    new_balance: number
    currency: string
    credited_at: string
    credit_type: string
    payment_method: string
    reference: string
  }
  message: string
  timestamp: string
}

export interface CommissionRate {
  agent_id: string
  rate: number
  effective_from: string
  created_at: string
  created_by: string
}

export interface TransferRequest {
  amount: number
  reference?: string
  notes?: string
}

export interface TransferResponse {
  success: boolean
  transaction_id: string
  transferred_amount: number
  commission_charged: number
  total_deducted: number
  agent_new_balance: number
  retailer_new_balance: number
  message: string
  timestamp: string
}

class WalletService {
  // Agent wallet operations
  async getAgentWalletBalance(agentId: string): Promise<WalletBalance> {
    const response = await api.get(`/admin/agents/${agentId}/wallet/balance`)
    return response.data.data || response.data
  }

  async creditAgentWallet(
    agentId: string,
    data: {
      amount: number
      credit_type: 'payment' | 'credit_loan'
      payment_method: string
      reference?: string
      notes?: string
    }
  ): Promise<CreditWalletResponse> {
    const response = await api.post(`/admin/agents/${agentId}/wallet/credit`, data)
    return response.data
  }

  async getTransactionHistory(
    entityId: string,
    entityType: 'agent' | 'retailer',
    params?: {
      wallet_type?: string
      page?: number
      page_size?: number
    }
  ): Promise<TransactionHistoryResponse> {
    const endpoint =
      entityType === 'agent'
        ? `/admin/agents/${entityId}/wallet/transactions`
        : `/admin/retailers/${entityId}/wallet/transactions`

    const response = await api.get(endpoint, { params })
    // Handle nested data structure from API
    if (response.data.data) {
      return {
        transactions: response.data.data.transactions || [],
        total_count: response.data.data.pagination?.total || 0,
        page: response.data.data.pagination?.page || 1,
        page_size: response.data.data.pagination?.page_size || 20,
        has_more: response.data.data.pagination?.has_more || false,
      }
    }
    return response.data
  }

  // Get all transactions across all wallets (admin only)
  async getAllTransactions(filters?: AllTransactionsFilters): Promise<AllTransactionsResponse> {
    // Build query parameters
    const params: Record<string, string | number | string[]> = {}

    if (filters?.transaction_types && filters.transaction_types.length > 0) {
      params['transaction_types[]'] = filters.transaction_types
    }

    if (filters?.wallet_types && filters.wallet_types.length > 0) {
      params['wallet_types[]'] = filters.wallet_types
    }

    if (filters?.statuses && filters.statuses.length > 0) {
      params['statuses[]'] = filters.statuses
    }

    if (filters?.start_date) {
      params.start_date = filters.start_date
    }

    if (filters?.end_date) {
      params.end_date = filters.end_date
    }

    if (filters?.search) {
      params.search = filters.search
    }

    if (filters?.page) {
      params.page = filters.page
    }

    if (filters?.page_size) {
      params.page_size = filters.page_size
    }

    if (filters?.sort_by) {
      params.sort_by = filters.sort_by
    }

    if (filters?.sort_order) {
      params.sort_order = filters.sort_order
    }

    const response = await api.get('/admin/wallet/transactions', { params })

    // Handle nested data structure from API
    if (response.data.data) {
      return {
        transactions: response.data.data.transactions || [],
        pagination: response.data.data.pagination || {
          page: 1,
          page_size: 20,
          total: 0,
          total_pages: 0,
          has_more: false,
        },
        statistics: response.data.data.statistics,
      }
    }

    return response.data
  }

  async transferAgentToRetailer(
    agentId: string,
    retailerId: string,
    data: TransferRequest
  ): Promise<TransferResponse> {
    const response = await api.post(`/admin/agents/${agentId}/transfer/${retailerId}`, data)
    return response.data
  }

  // Retailer wallet operations
  async getRetailerWalletBalance(
    retailerId: string,
    walletType: 'stake' | 'winning' = 'stake'
  ): Promise<WalletBalance> {
    const response = await api.get(`/admin/retailers/${retailerId}/wallet/${walletType}/balance`)
    return response.data.data || response.data
  }

  async creditRetailerWallet(
    retailerId: string,
    data: {
      wallet_type: 'stake' | 'winning'
      amount: number
      credit_type: 'payment' | 'credit_loan'
      payment_method?: string
      reference?: string
      agent_id?: string
      notes?: string
    }
  ): Promise<CreditWalletResponse> {
    const response = await api.post(`/admin/retailers/${retailerId}/wallet/credit`, data)
    return response.data
  }

  // Commission operations
  async getCommissionRate(agentId: string): Promise<CommissionRate> {
    const response = await api.get(`/admin/agents/${agentId}/commission/rate`)
    return response.data
  }

  async setCommissionRate(
    agentId: string,
    data: {
      rate: number
      effective_from?: string
      notes?: string
    }
  ): Promise<{
    success: boolean
    message: string
    new_rate: number
    effective_from: string
  }> {
    const response = await api.post(`/admin/agents/${agentId}/commission/rate`, data)
    return response.data
  }

  async getCommissionReport(
    agentId: string,
    params?: {
      page?: number
      page_size?: number
    }
  ): Promise<{
    entries: Array<{
      id: string
      transaction_id: string
      agent_id: string
      original_amount: number
      gross_amount: number
      commission_amount: number
      commission_rate: number
      type: string
      created_at: string
      reference?: string
    }>
    total_commission: number
    total_count: number
    page: number
    page_size: number
    has_more: boolean
  }> {
    const response = await api.get(`/admin/agents/${agentId}/commission/report`, {
      params,
    })
    return response.data
  }

  // Reverse a transaction (Admin only)
  async reverseTransaction(
    transactionId: string,
    data: {
      reason: string
      confirmed: boolean
    }
  ): Promise<{
    success: boolean
    message: string
    data: {
      reversal_transaction_id: string
      reversed_amount: number
      new_wallet_balance: number
      balance_is_negative: boolean
      reversed_at: string
      original_transaction: WalletTransaction
      reversal_transaction: WalletTransaction
      reversed_by: {
        admin_id: string
        admin_name: string
        admin_email: string
      }
    }
    timestamp: string
  }> {
    const response = await api.post(`/admin/wallet/transactions/${transactionId}/reverse`, data)
    return response.data
  }
}

export const walletService = new WalletService()
