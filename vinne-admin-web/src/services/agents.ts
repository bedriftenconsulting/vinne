import api from '@/lib/api'

export interface Agent {
  id: string
  agent_code: string
  name: string
  email: string
  phone_number: string
  address: string
  commission_percentage: number
  status: 'ACTIVE' | 'SUSPENDED' | 'UNDER_REVIEW' | 'INACTIVE' | 'TERMINATED'
  created_by: string
  updated_by?: string
  created_at: string
  updated_at: string
  initial_password?: string
}

export interface Retailer {
  id: string
  retailer_code: string
  name: string
  email: string
  phone_number: string
  address: string
  agent_id?: string
  status: 'ACTIVE' | 'SUSPENDED' | 'UNDER_REVIEW' | 'INACTIVE' | 'TERMINATED'
  created_by: string
  updated_by?: string
  created_at: string
  updated_at: string
}

// Removed: Commission tiers have been replaced with simple commission percentage on agents

export interface CreateAgentRequest {
  name: string
  email?: string
  phone_number: string
  address: string
  commission_percentage?: number
  created_by: string
}

export interface UpdateAgentRequest {
  name?: string
  email?: string
  phone_number?: string
  address?: string
  commission_percentage?: number
  updated_by?: string
}

export interface CreateRetailerRequest {
  name: string
  email: string
  phone_number: string
  address: string
  agent_id?: string
  created_by: string
}

export interface UpdateRetailerRequest {
  name: string
  email: string
  phone_number: string
  address: string
  updated_by: string
}

// Removed: Commission tier interfaces - replaced with simple percentage on agents

export interface UpdateStatusRequest {
  status: 'ACTIVE' | 'SUSPENDED' | 'UNDER_REVIEW' | 'INACTIVE' | 'TERMINATED'
  // updated_by is extracted from JWT token on backend, no need to send
}

// Agent filters for list endpoint
export interface AgentFilters {
  status?: string
  name?: string
  email?: string
}

// Retailer filters for list endpoint
export interface RetailerFilters {
  status?: string
  agent_id?: string
  name?: string
  email?: string
}

// Standard API response format
export interface ApiResponse<T> {
  success: boolean
  message: string
  data?: T
  agent?: T
  retailer?: T
}

// Paginated response for lists
export interface PaginatedResponse<T> {
  data: T[]
  pagination: {
    page: number
    page_size: number
    total_count: number
    total_pages?: number
  }
}

export const agentService = {
  // Agents
  async getAgents(
    page = 1,
    pageSize = 20,
    filters: AgentFilters = {}
  ): Promise<PaginatedResponse<Agent>> {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
    })

    if (filters.status) params.append('status', filters.status)
    if (filters.name) params.append('name', filters.name)
    if (filters.email) params.append('email', filters.email)

    const response = await api.get<PaginatedResponse<Agent>>(`/admin/agents?${params}`)
    return response.data
  },

  async getAgent(id: string): Promise<Agent> {
    const response = await api.get<ApiResponse<Agent>>(`/admin/agents/${id}`)
    return response.data.agent!
  },

  async createAgent(data: CreateAgentRequest): Promise<Agent> {
    const response = await api.post<ApiResponse<Agent>>('/admin/agents', data)
    return response.data.agent!
  },

  async updateAgent(id: string, data: UpdateAgentRequest): Promise<Agent> {
    const response = await api.put<ApiResponse<Agent>>(`/admin/agents/${id}`, data)
    return response.data.agent!
  },

  async updateAgentStatus(id: string, data: UpdateStatusRequest): Promise<void> {
    await api.put(`/admin/agents/${id}/status`, data)
  },

  // Retailers
  async getRetailers(
    page = 1,
    pageSize = 20,
    filters: RetailerFilters = {}
  ): Promise<PaginatedResponse<Retailer>> {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
    })

    if (filters.status) params.append('status', filters.status)
    if (filters.agent_id) params.append('agent_id', filters.agent_id)
    if (filters.name) params.append('name', filters.name)
    if (filters.email) params.append('email', filters.email)

    const response = await api.get<PaginatedResponse<Retailer>>(`/admin/retailers?${params}`)
    return response.data
  },

  async getRetailer(id: string): Promise<Retailer> {
    const response = await api.get<ApiResponse<Retailer>>(`/admin/retailers/${id}`)
    return response.data.retailer!
  },

  async getRetailerById(id: string): Promise<Retailer> {
    return this.getRetailer(id)
  },

  async createRetailer(data: CreateRetailerRequest): Promise<any> {
    const response = await api.post<any>('/admin/retailers', data)
    return response.data
  },

  async updateRetailer(id: string, data: UpdateRetailerRequest): Promise<Retailer> {
    const response = await api.put<ApiResponse<Retailer>>(`/admin/retailers/${id}`, data)
    return response.data.retailer!
  },

  async updateRetailerStatus(id: string, data: UpdateStatusRequest): Promise<void> {
    await api.put(`/admin/retailers/${id}/status`, data)
  },

  // Agent Commission Data
  async getAgentCommissions(agentId: string): Promise<unknown> {
    const response = await api.get(`/admin/agents/${agentId}/commissions`)
    return response.data
  },
}
