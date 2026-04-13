import api from '@/lib/api'

export interface User {
  id: string
  email: string
  username: string
  first_name?: string
  last_name?: string
  mfa_enabled: boolean
  is_active: boolean
  last_login?: string
  last_login_ip?: string
  ip_whitelist: string[]
  roles: Role[]
  created_at: string
  updated_at: string
}

export interface Role {
  id: string
  name: string
  description: string
  permissions: Permission[]
  created_at: string
}

export interface Permission {
  id: string
  resource: string
  action: string
  description?: string
  created_at?: string
}

export interface AuditLog {
  id: string
  admin_user_id: string
  action: string
  resource?: string
  resource_id?: string
  ip_address: string
  user_agent: string
  request_data: Record<string, unknown>
  response_status: number
  created_at: string
  admin_user?: User
}

export interface CreateUserRequest {
  email: string
  username: string
  password: string
  first_name?: string
  last_name?: string
  role_ids?: string[]
  ip_whitelist?: string[]
}

export interface UpdateUserRequest {
  email?: string
  username?: string
  first_name?: string
  last_name?: string
  ip_whitelist?: string[]
  mfa_enabled?: boolean
  role_ids?: string[]
}

export interface CreateRoleRequest {
  name: string
  description: string
  permission_ids: string[]
}

export interface UpdateRoleRequest {
  name?: string
  description?: string
  permission_ids: string[]
}

export interface RoleAssignmentRequest {
  user_id: string
  role_id: string
}

// Standard API response format from PRD
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

// Paginated response with standard format
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

export const adminService = {
  // Users
  async getUsers(
    page = 1,
    pageSize = 20,
    filters: {
      email?: string
      username?: string
      role_id?: string
      is_active?: boolean
      mfa_enabled?: boolean
    } = {}
  ): Promise<PaginatedResponse<User>> {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
    })

    if (filters.email) params.append('email', filters.email)
    if (filters.username) params.append('username', filters.username)
    if (filters.role_id) params.append('role_id', filters.role_id)
    if (filters.is_active !== undefined) params.append('is_active', filters.is_active.toString())
    if (filters.mfa_enabled !== undefined)
      params.append('mfa_enabled', filters.mfa_enabled.toString())

    const response = await api.get<PaginatedApiResponse<User>>(`/admin/users?${params}`)
    return {
      data: response.data.data,
      total_count: response.data.pagination.total_count,
      page: response.data.pagination.page,
      page_size: response.data.pagination.page_size,
      total_pages: response.data.pagination.total_pages,
    }
  },

  async getUser(id: string): Promise<User> {
    const response = await api.get<ApiResponse<User>>(`/admin/users/${id}`)
    return response.data.data
  },

  async createUser(data: CreateUserRequest): Promise<User> {
    const response = await api.post<ApiResponse<User>>('/admin/users', data)
    return response.data.data
  },

  async updateUser(id: string, data: UpdateUserRequest): Promise<User> {
    const response = await api.put<ApiResponse<User>>(`/admin/users/${id}`, data)
    return response.data.data
  },

  async assignRoleToUser(userId: string, roleId: string): Promise<void> {
    await api.post('/admin/role-assignments', { user_id: userId, role_id: roleId })
  },

  async removeRoleFromUser(userId: string, roleId: string): Promise<void> {
    await api.delete('/admin/role-assignments', { data: { user_id: userId, role_id: roleId } })
  },

  async deleteUser(id: string): Promise<void> {
    await api.delete(`/admin/users/${id}`)
  },

  async activateUser(id: string): Promise<User> {
    const response = await api.post<ApiResponse<User>>(`/admin/users/${id}/activate`)
    return response.data.data
  },

  async deactivateUser(id: string): Promise<User> {
    const response = await api.post<ApiResponse<User>>(`/admin/users/${id}/deactivate`)
    return response.data.data
  },

  // Roles
  async getRoles(page = 1, pageSize = 20): Promise<PaginatedResponse<Role>> {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
    })

    const response = await api.get<PaginatedApiResponse<Role>>(`/admin/roles?${params}`)
    return {
      data: response.data.data,
      total_count: response.data.pagination.total_count,
      page: response.data.pagination.page,
      page_size: response.data.pagination.page_size,
      total_pages: response.data.pagination.total_pages,
    }
  },

  async getAllRoles(): Promise<Role[]> {
    // Get all roles by requesting a large page size
    const response = await this.getRoles(1, 1000)
    return response.data
  },

  async getRole(id: string): Promise<Role> {
    const response = await api.get<ApiResponse<Role>>(`/admin/roles/${id}`)
    return response.data.data
  },

  async createRole(data: CreateRoleRequest): Promise<Role> {
    const response = await api.post<ApiResponse<Role>>('/admin/roles', data)
    return response.data.data
  },

  async updateRole(id: string, data: UpdateRoleRequest): Promise<Role> {
    const response = await api.put<ApiResponse<Role>>(`/admin/roles/${id}`, data)
    return response.data.data
  },

  async deleteRole(id: string): Promise<void> {
    await api.delete(`/admin/roles/${id}`)
  },

  // Permissions
  async getPermissions(
    page = 1,
    pageSize = 100,
    resource?: string
  ): Promise<PaginatedResponse<Permission>> {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
    })

    if (resource) params.append('resource', resource)

    const response = await api.get<PaginatedApiResponse<Permission>>(`/admin/permissions?${params}`)
    return {
      data: response.data.data,
      total_count: response.data.pagination.total_count,
      page: response.data.pagination.page,
      page_size: response.data.pagination.page_size,
      total_pages: response.data.pagination.total_pages,
    }
  },

  async getAllPermissions(): Promise<Permission[]> {
    const response = await this.getPermissions(1, 1000)
    return response.data
  },

  // Role Assignments
  async assignRole(data: RoleAssignmentRequest): Promise<void> {
    await api.post('/admin/role-assignments', data)
  },

  async removeRole(data: RoleAssignmentRequest): Promise<void> {
    await api.delete('/admin/role-assignments', { data })
  },

  // Audit Logs
  async getAuditLogs(
    page = 1,
    pageSize = 20,
    filters: {
      user_id?: string
      action?: string
      resource?: string
      start_date?: string
      end_date?: string
    } = {}
  ): Promise<PaginatedResponse<AuditLog>> {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
    })

    if (filters.user_id) params.append('user_id', filters.user_id)
    if (filters.action) params.append('action', filters.action)
    if (filters.resource) params.append('resource', filters.resource)
    if (filters.start_date) params.append('start_date', filters.start_date)
    if (filters.end_date) params.append('end_date', filters.end_date)

    const response = await api.get<PaginatedApiResponse<AuditLog>>(`/admin/audit-logs?${params}`)
    return {
      data: response.data.data,
      total_count: response.data.pagination.total_count,
      page: response.data.pagination.page,
      page_size: response.data.pagination.page_size,
      total_pages: response.data.pagination.total_pages,
    }
  },
}
