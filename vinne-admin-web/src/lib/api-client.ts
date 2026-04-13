// API client configuration
import { config } from '@/config'

export class ApiClient {
  private baseURL: string

  constructor() {
    // API URL from config already includes /api/v1
    this.baseURL = config.api.baseUrl
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const url = `${this.baseURL}${endpoint}`

    // Add auth token if available
    const token = localStorage.getItem('access_token')
    if (token) {
      options.headers = {
        ...options.headers,
        Authorization: `Bearer ${token}`,
      }
    }

    // Add content type for JSON
    if (options.body && typeof options.body === 'string') {
      options.headers = {
        ...options.headers,
        'Content-Type': 'application/json',
      }
    }

    // Ensure credentials are included for cookies
    options.credentials = 'include'

    if (config.debug.enabled) {
      console.log(`[API] ${options.method || 'GET'} ${url}`)
    }

    const response = await fetch(url, options)

    if (!response.ok) {
      throw new Error(`API Error: ${response.status} ${response.statusText}`)
    }

    return response.json()
  }

  public get<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'GET' })
  }

  public post<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    })
  }

  public put<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'PUT',
      body: data ? JSON.stringify(data) : undefined,
    })
  }

  public delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'DELETE' })
  }
}

export const apiClient = new ApiClient()
