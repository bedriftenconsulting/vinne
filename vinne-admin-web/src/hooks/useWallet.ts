import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { walletService } from '@/services/walletService'
import type {
  CreditWalletRequest,
  TransferRequest,
  TransactionHistoryParams,
} from '@/services/walletService'

// Query keys
const walletKeys = {
  all: ['wallets'] as const,
  agents: () => [...walletKeys.all, 'agents'] as const,
  agentBalance: (agentId: string) => [...walletKeys.agents(), agentId, 'balance'] as const,
  agentTransactions: (agentId: string, params?: Record<string, unknown>) =>
    [...walletKeys.agents(), agentId, 'transactions', params] as const,
  agentCommissionRate: (agentId: string) =>
    [...walletKeys.agents(), agentId, 'commission-rate'] as const,
  retailers: () => [...walletKeys.all, 'retailers'] as const,
  retailerStakeBalance: (retailerId: string) =>
    [...walletKeys.retailers(), retailerId, 'stake-balance'] as const,
  retailerWinningBalance: (retailerId: string) =>
    [...walletKeys.retailers(), retailerId, 'winning-balance'] as const,
  retailerTransactions: (
    retailerId: string,
    walletType: string,
    params?: Record<string, unknown>
  ) => [...walletKeys.retailers(), retailerId, walletType, 'transactions', params] as const,
}

// Agent wallet hooks
export function useAgentBalance(agentId: string | undefined) {
  return useQuery({
    queryKey: walletKeys.agentBalance(agentId!),
    queryFn: () => walletService.getAgentBalance(agentId!),
    enabled: !!agentId,
  })
}

export function useCreditAgentWallet() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ agentId, ...request }: { agentId: string } & CreditWalletRequest) =>
      walletService.creditAgentWallet(agentId, request),
    onSuccess: (_, variables) => {
      // Invalidate agent balance and transactions
      queryClient.invalidateQueries({
        queryKey: walletKeys.agentBalance(variables.agentId),
      })
      queryClient.invalidateQueries({
        queryKey: walletKeys.agentTransactions(variables.agentId),
      })
    },
  })
}

export function useAgentTransactionHistory(
  agentId: string | undefined,
  params?: Omit<TransactionHistoryParams, 'walletOwnerId' | 'walletType'>
) {
  return useQuery({
    queryKey: walletKeys.agentTransactions(agentId!, params),
    queryFn: () => walletService.getAgentTransactionHistory(agentId!, params),
    enabled: !!agentId,
  })
}

// Retailer wallet hooks
export function useRetailerStakeBalance(retailerId: string | undefined) {
  return useQuery({
    queryKey: walletKeys.retailerStakeBalance(retailerId!),
    queryFn: () => walletService.getRetailerStakeBalance(retailerId!),
    enabled: !!retailerId,
  })
}

export function useRetailerWinningBalance(retailerId: string | undefined) {
  return useQuery({
    queryKey: walletKeys.retailerWinningBalance(retailerId!),
    queryFn: () => walletService.getRetailerWinningBalance(retailerId!),
    enabled: !!retailerId,
  })
}

export function useCreditRetailerStakeWallet() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ retailerId, ...request }: { retailerId: string } & CreditWalletRequest) =>
      walletService.creditRetailerStakeWallet(retailerId, request),
    onSuccess: (_, variables) => {
      // Invalidate retailer balances and transactions
      queryClient.invalidateQueries({
        queryKey: walletKeys.retailerStakeBalance(variables.retailerId),
      })
      queryClient.invalidateQueries({
        queryKey: walletKeys.retailerTransactions(variables.retailerId, 'stake'),
      })
    },
  })
}

export function useCreditRetailerWinningWallet() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ retailerId, ...request }: { retailerId: string } & CreditWalletRequest) =>
      walletService.creditRetailerWinningWallet(retailerId, request),
    onSuccess: (_, variables) => {
      // Invalidate retailer balances and transactions
      queryClient.invalidateQueries({
        queryKey: walletKeys.retailerWinningBalance(variables.retailerId),
      })
      queryClient.invalidateQueries({
        queryKey: walletKeys.retailerTransactions(variables.retailerId, 'winning'),
      })
    },
  })
}

export function useRetailerTransactionHistory(
  retailerId: string | undefined,
  walletType: 'stake' | 'winning',
  params?: Omit<TransactionHistoryParams, 'walletOwnerId' | 'walletType'>
) {
  return useQuery({
    queryKey: walletKeys.retailerTransactions(retailerId!, walletType, params),
    queryFn: () => walletService.getRetailerTransactionHistory(retailerId!, walletType, params),
    enabled: !!retailerId,
  })
}

// Transfer operations
export function useTransferAgentToRetailer() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (request: TransferRequest) => walletService.transferAgentToRetailer(request),
    onSuccess: (_, variables) => {
      // Invalidate both agent and retailer data
      queryClient.invalidateQueries({
        queryKey: walletKeys.agentBalance(variables.fromAgentId),
      })
      queryClient.invalidateQueries({
        queryKey: walletKeys.agentTransactions(variables.fromAgentId),
      })
      queryClient.invalidateQueries({
        queryKey: walletKeys.retailerStakeBalance(variables.toRetailerId),
      })
      queryClient.invalidateQueries({
        queryKey: walletKeys.retailerTransactions(variables.toRetailerId, 'stake'),
      })
    },
  })
}

// Commission hooks
export function useAgentCommissionRate(agentId: string | undefined) {
  return useQuery({
    queryKey: walletKeys.agentCommissionRate(agentId!),
    queryFn: () => walletService.getAgentCommissionRate(agentId!),
    enabled: !!agentId,
  })
}

export function useSetAgentCommissionRate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      agentId,
      rate,
      effectiveFrom,
    }: {
      agentId: string
      rate: number
      effectiveFrom?: string
    }) => walletService.setAgentCommissionRate(agentId, rate, effectiveFrom),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: walletKeys.agentCommissionRate(variables.agentId),
      })
    },
  })
}

export function useCommissionReport(
  agentId: string | undefined,
  startDate: string,
  endDate: string
) {
  return useQuery({
    queryKey: [...walletKeys.agents(), agentId, 'commission-report', { startDate, endDate }],
    queryFn: () => walletService.getCommissionReport(agentId!, startDate, endDate),
    enabled: !!agentId && !!startDate && !!endDate,
  })
}

// Export operations
export function useExportTransactions() {
  return useMutation({
    mutationFn: ({
      ownerId,
      ownerType,
      walletType,
      params,
    }: {
      ownerId: string
      ownerType: 'agent' | 'retailer'
      walletType?: 'stake' | 'winning'
      params?: Record<string, unknown>
    }) => walletService.exportTransactionHistory(ownerId, ownerType, walletType, params),
    onSuccess: (data, variables) => {
      // Create download link
      const url = window.URL.createObjectURL(new Blob([data]))
      const link = document.createElement('a')
      link.href = url
      const filename = `${variables.ownerType}_${variables.ownerId}_transactions_${
        new Date().toISOString().split('T')[0]
      }.csv`
      link.setAttribute('download', filename)
      document.body.appendChild(link)
      link.click()
      link.remove()
      window.URL.revokeObjectURL(url)
    },
  })
}
