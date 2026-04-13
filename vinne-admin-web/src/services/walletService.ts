import { apiClient } from '@/lib/api-client'

export interface WalletBalance {
  walletId: string
  ownerId: string
  ownerType: 'agent' | 'retailer'
  balance: number
  pendingBalance?: number
  availableBalance?: number
  currency: string
  lastTransactionAt?: string
}

export interface WalletTransaction {
  id: string
  transactionId: string
  walletId: string
  walletType: string
  type: 'credit' | 'debit' | 'transfer' | 'commission'
  amount: number
  balanceBefore: number
  balanceAfter: number
  description: string
  reference?: string
  status: 'completed' | 'pending' | 'failed'
  metadata?: Record<string, unknown>
  createdAt: string
  updatedAt?: string
}

export interface CreditWalletRequest {
  amount: number // In pesewas
  description: string
  reference?: string
}

export interface TransferRequest {
  fromAgentId: string
  toRetailerId: string
  amount: number // In pesewas
  description: string
}

export interface TransactionHistoryParams {
  walletOwnerId: string
  walletType: 'agent_stake' | 'retailer_stake' | 'retailer_winning'
  page?: number
  pageSize?: number
  type?: string
  status?: string
  startDate?: string
  endDate?: string
}

export interface TransactionHistoryResponse {
  transactions: WalletTransaction[]
  totalCount: number
  page: number
  pageSize: number
}

export interface CommissionRate {
  agentId: string
  rate: number // In basis points
  effectiveFrom: string
  effectiveTo?: string
  createdBy: string
  createdAt: string
}

export interface CommissionReport {
  agentId: string
  totalCommission: number
  transactionCount: number
  period: {
    start: string
    end: string
  }
  breakdown: {
    deposits: {
      count: number
      totalCommission: number
    }
    transfers: {
      count: number
      totalCommission: number
    }
  }
}

class WalletService {
  private basePath = '/admin/wallets'

  // Agent wallet operations
  async getAgentBalance(agentId: string): Promise<WalletBalance> {
    return await apiClient.get<WalletBalance>(`/admin/agents/${agentId}/wallet/balance`)
  }

  async creditAgentWallet(
    agentId: string,
    request: CreditWalletRequest
  ): Promise<WalletTransaction> {
    return await apiClient.post<WalletTransaction>(
      `/admin/agents/${agentId}/wallet/credit`,
      request
    )
  }

  async getAgentTransactionHistory(
    agentId: string,
    params?: Omit<TransactionHistoryParams, 'walletOwnerId' | 'walletType'>
  ): Promise<TransactionHistoryResponse> {
    // Note: apiClient.get doesn't support params in second argument
    const queryParams: Record<string, string> = {
      walletType: 'agent_stake',
    }
    if (params) {
      if (params.page !== undefined) queryParams.page = params.page.toString()
      if (params.pageSize !== undefined) queryParams.pageSize = params.pageSize.toString()
      if (params.type) queryParams.type = params.type
      if (params.status) queryParams.status = params.status
      if (params.startDate) queryParams.startDate = params.startDate
      if (params.endDate) queryParams.endDate = params.endDate
    }
    const queryString = new URLSearchParams(queryParams).toString()
    return await apiClient.get<TransactionHistoryResponse>(
      `/admin/agents/${agentId}/wallet/transactions${queryString ? '?' + queryString : ''}`
    )
  }

  // Retailer wallet operations
  async getRetailerStakeBalance(retailerId: string): Promise<WalletBalance> {
    return await apiClient.get<WalletBalance>(`/admin/retailers/${retailerId}/wallet/stake/balance`)
  }

  async getRetailerWinningBalance(retailerId: string): Promise<WalletBalance> {
    return await apiClient.get<WalletBalance>(
      `/admin/retailers/${retailerId}/wallet/winning/balance`
    )
  }

  async creditRetailerStakeWallet(
    retailerId: string,
    request: CreditWalletRequest
  ): Promise<WalletTransaction> {
    return await apiClient.post<WalletTransaction>(
      `/admin/retailers/${retailerId}/wallet/stake/credit`,
      request
    )
  }

  async creditRetailerWinningWallet(
    retailerId: string,
    request: CreditWalletRequest
  ): Promise<WalletTransaction> {
    return await apiClient.post<WalletTransaction>(
      `/admin/retailers/${retailerId}/wallet/winning/credit`,
      request
    )
  }

  async getRetailerTransactionHistory(
    retailerId: string,
    walletType: 'stake' | 'winning',
    params?: Omit<TransactionHistoryParams, 'walletOwnerId' | 'walletType'>
  ): Promise<TransactionHistoryResponse> {
    const queryString = params
      ? new URLSearchParams(params as Record<string, string>).toString()
      : ''
    return await apiClient.get<TransactionHistoryResponse>(
      `/admin/retailers/${retailerId}/wallet/${walletType}/transactions${queryString ? '?' + queryString : ''}`
    )
  }

  // Transfer operations
  async transferAgentToRetailer(request: TransferRequest): Promise<WalletTransaction> {
    return await apiClient.post<WalletTransaction>(
      `${this.basePath}/transfer/agent-to-retailer`,
      request
    )
  }

  // Commission operations
  async getAgentCommissionRate(agentId: string): Promise<CommissionRate> {
    return await apiClient.get<CommissionRate>(`/admin/agents/${agentId}/commission/rate`)
  }

  async setAgentCommissionRate(
    agentId: string,
    rate: number,
    effectiveFrom?: string
  ): Promise<CommissionRate> {
    return await apiClient.post<CommissionRate>(`/admin/agents/${agentId}/commission/rate`, {
      rate,
      effectiveFrom: effectiveFrom || new Date().toISOString(),
    })
  }

  async getCommissionReport(
    agentId: string,
    startDate: string,
    endDate: string
  ): Promise<CommissionReport> {
    const queryString = new URLSearchParams({ startDate, endDate }).toString()
    return await apiClient.get<CommissionReport>(
      `/admin/agents/${agentId}/commission/report?${queryString}`
    )
  }

  // Export operations
  async exportTransactionHistory(
    ownerId: string,
    ownerType: 'agent' | 'retailer',
    walletType?: 'stake' | 'winning',
    params?: {
      startDate?: string
      endDate?: string
      type?: string
      status?: string
    }
  ): Promise<Blob> {
    const endpoint =
      ownerType === 'agent'
        ? `/admin/agents/${ownerId}/wallet/transactions/export`
        : `/admin/retailers/${ownerId}/wallet/${walletType}/transactions/export`

    const queryString = params
      ? new URLSearchParams(params as Record<string, string>).toString()
      : ''
    // Note: apiClient doesn't support blob responses, using fetch directly
    const token = localStorage.getItem('access_token')
    const response = await fetch(`${endpoint}${queryString ? '?' + queryString : ''}`, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    })
    if (!response.ok) {
      throw new Error(`Export failed: ${response.status}`)
    }
    return await response.blob()
  }

  // Utility methods
  formatCurrency(amountInPesewas: number): string {
    return new Intl.NumberFormat('en-GH', {
      style: 'currency',
      currency: 'GHS',
      minimumFractionDigits: 2,
    }).format(amountInPesewas / 100)
  }

  formatCommissionRate(basisPoints: number): string {
    return `${(basisPoints / 100).toFixed(2)}%`
  }

  convertGHSToPesewas(ghs: number): number {
    return Math.round(ghs * 100)
  }

  convertPesewasToGHS(pesewas: number): number {
    return pesewas / 100
  }
}

export const walletService = new WalletService()
