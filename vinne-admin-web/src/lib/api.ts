import axios from 'axios'

// Create an axios instance with default configuration
export const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:4000/api/v1',
  timeout: 30000, // 30 seconds - increased to handle file uploads
  headers: {
    'Content-Type': 'application/json',
  },
  withCredentials: true, // Important for CORS with credentials
})

// Request interceptor to add auth token
api.interceptors.request.use(
  config => {
    const token = localStorage.getItem('access_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  error => {
    return Promise.reject(error)
  }
)

// Response interceptor to handle token refresh and invalid tokens
api.interceptors.response.use(
  response => response,
  async error => {
    const originalRequest = error.config

    // Check for 401 Unauthorized errors
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true

      // Don't try to refresh for login/refresh endpoints
      if (
        originalRequest.url?.includes('/auth/login') ||
        originalRequest.url?.includes('/auth/refresh')
      ) {
        return Promise.reject(error)
      }

      try {
        const refreshToken = localStorage.getItem('refresh_token')
        if (refreshToken) {
          const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:4000/api/v1'
          const response = await axios.post(
            `${apiUrl}/admin/auth/refresh`,
            { refresh_token: refreshToken },
            { withCredentials: true }
          )

          if (response.data.success) {
            const { access_token, refresh_token: newRefreshToken } = response.data.data
            localStorage.setItem('access_token', access_token)
            localStorage.setItem('refresh_token', newRefreshToken)

            // Retry original request with new token
            originalRequest.headers.Authorization = `Bearer ${access_token}`
            return api(originalRequest)
          }
        } else {
          // No refresh token available, clear auth and redirect
          localStorage.removeItem('access_token')
          localStorage.removeItem('refresh_token')

          // Clear auth state if available
          const authState = localStorage.getItem('auth-storage')
          if (authState) {
            const parsedState = JSON.parse(authState)
            parsedState.state.isAuthenticated = false
            parsedState.state.user = null
            localStorage.setItem('auth-storage', JSON.stringify(parsedState))
          }

          window.location.href = '/login'
        }
      } catch {
        // Refresh failed, clear everything and redirect to login
        localStorage.removeItem('access_token')
        localStorage.removeItem('refresh_token')

        // Clear auth state if available
        const authState = localStorage.getItem('auth-storage')
        if (authState) {
          const parsedState = JSON.parse(authState)
          parsedState.state.isAuthenticated = false
          parsedState.state.user = null
          localStorage.setItem('auth-storage', JSON.stringify(parsedState))
        }

        window.location.href = '/login'
      }
    }

    // Check for 403 Forbidden errors (but not CORS errors)
    if (error.response?.status === 403) {
      // Check if this is a CORS error or actual forbidden
      const errorMessage = error.response?.data?.error || ''

      // If it's an actual forbidden error (not CORS)
      if (errorMessage && !errorMessage.includes('Origin not allowed')) {
        // Token is invalid, clear auth and redirect
        localStorage.removeItem('access_token')
        localStorage.removeItem('refresh_token')

        // Clear auth state if available
        const authState = localStorage.getItem('auth-storage')
        if (authState) {
          const parsedState = JSON.parse(authState)
          parsedState.state.isAuthenticated = false
          parsedState.state.user = null
          localStorage.setItem('auth-storage', JSON.stringify(parsedState))
        }

        window.location.href = '/login'
      }
      // If it's a CORS error, let it through for proper error handling
    }

    return Promise.reject(error)
  }
)

export default api
