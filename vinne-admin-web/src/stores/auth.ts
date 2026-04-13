import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'
import api from '@/lib/api'

interface Role {
  id: string
  name: string
  description: string
  created_at: string
  updated_at: string
}

interface User {
  id: string
  email: string
  username: string
  roles: Role[]
  created_at: string
  updated_at: string
}

interface AuthState {
  user: User | null
  isAuthenticated: boolean
  isLoading: boolean
  isInitialized: boolean
  login: (credentials: { username: string; password: string }) => Promise<void>
  adminLogin: (credentials: { email: string; password: string }) => Promise<void>
  logout: () => void
  adminLogout: () => Promise<void>
  refreshToken: () => Promise<void>
  setUser: (user: User | null) => void
  validateAuth: () => Promise<void>
  initializeAuth: () => Promise<void>
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      isAuthenticated: false,
      isLoading: false,
      isInitialized: false,

      login: async credentials => {
        set({ isLoading: true })
        try {
          const response = await api.post('/auth/login', credentials)

          if (response.data.success) {
            const { access_token, refresh_token, user } = response.data.data

            // Store tokens
            localStorage.setItem('access_token', access_token)
            localStorage.setItem('refresh_token', refresh_token)

            // Update state
            set({
              user,
              isAuthenticated: true,
              isLoading: false,
            })
          }
        } catch (error) {
          set({ isLoading: false })
          throw error
        }
      },

      adminLogin: async credentials => {
        set({ isLoading: true })
        try {
          const response = await api.post('/admin/auth/login', credentials)

          if (response.data.success) {
            const { access_token, refresh_token, user } = response.data.data

            // Store tokens
            localStorage.setItem('access_token', access_token)
            localStorage.setItem('refresh_token', refresh_token)

            // Update state and immediately persist it
            set({
              user,
              isAuthenticated: true,
              isLoading: false,
              isInitialized: true,
            })

            // Manually persist the state to localStorage to ensure it's saved
            const authState = {
              state: {
                user,
                isAuthenticated: true,
              },
              version: 0,
            }
            localStorage.setItem('auth-storage', JSON.stringify(authState))
          }
        } catch (error) {
          set({ isLoading: false })
          throw error
        }
      },

      logout: async () => {
        try {
          await api.post('/admin/auth/logout')
        } catch (error) {
          console.error('Logout error:', error)
        } finally {
          // Clear tokens and state
          localStorage.removeItem('access_token')
          localStorage.removeItem('refresh_token')
          set({
            user: null,
            isAuthenticated: false,
            isLoading: false,
          })
        }
      },

      adminLogout: async () => {
        try {
          await api.post('/admin/auth/logout')
        } catch (error) {
          console.error('Admin logout error:', error)
        } finally {
          // Clear tokens and state
          localStorage.removeItem('access_token')
          localStorage.removeItem('refresh_token')
          set({
            user: null,
            isAuthenticated: false,
            isLoading: false,
          })
          // Redirect to login page after logout
          window.location.href = '/login'
        }
      },

      refreshToken: async () => {
        const refreshToken = localStorage.getItem('refresh_token')
        if (!refreshToken) {
          throw new Error('No refresh token available')
        }

        try {
          const response = await api.post('/admin/auth/refresh', {
            refresh_token: refreshToken,
          })

          if (response.data.success) {
            const { access_token, refresh_token: newRefreshToken } = response.data.data
            localStorage.setItem('access_token', access_token)
            localStorage.setItem('refresh_token', newRefreshToken)
          }
        } catch (error) {
          // If refresh fails, logout user
          get().logout()
          throw error
        }
      },

      setUser: user => {
        set({
          user,
          isAuthenticated: !!user,
        })
      },

      validateAuth: async () => {
        const accessToken = localStorage.getItem('access_token')
        const refreshToken = localStorage.getItem('refresh_token')
        const currentState = get()

        if (!accessToken && !refreshToken) {
          set({ user: null, isAuthenticated: false, isInitialized: true })
          return
        }

        // If we already have user data from localStorage, keep it while validating
        if (currentState.user && currentState.isAuthenticated) {
          set({ isInitialized: true })
        }

        try {
          const response = await api.get('/admin/profile')

          if (response.data.success) {
            const profileUser = response.data.data.user
            // Merge with existing user to preserve fields like roles that profile may not return
            const existingUser = get().user
            set({
              user: { ...existingUser, ...profileUser },
              isAuthenticated: true,
              isInitialized: true,
            })
          }
        } catch (error) {
          // Token might be expired, try to refresh
          if (
            (error as unknown as { response?: { status?: number } })?.response?.status === 401 &&
            refreshToken
          ) {
            try {
              await get().refreshToken()
              // After successful refresh, try to get user info again
              const response = await api.get('/admin/profile')
              if (response.data.success) {
                set({
                  user: response.data.data.user,
                  isAuthenticated: true,
                  isInitialized: true,
                })
              }
            } catch {
              // Refresh failed, clear everything and redirect to login
              localStorage.removeItem('access_token')
              localStorage.removeItem('refresh_token')
              set({
                user: null,
                isAuthenticated: false,
                isInitialized: true,
              })
              window.location.href = '/login'
            }
          } else {
            // Other error or no refresh token, clear and redirect
            localStorage.removeItem('access_token')
            localStorage.removeItem('refresh_token')
            set({
              user: null,
              isAuthenticated: false,
              isInitialized: true,
            })
            window.location.href = '/login'
          }
        }
      },

      initializeAuth: async () => {
        const state = get()
        // Skip if already initialized to prevent re-running on every render
        if (state.isInitialized) {
          return
        }

        // First check if we have stored auth state from localStorage (Zustand persist)
        const storedState = localStorage.getItem('auth-storage')
        if (storedState) {
          try {
            const parsedState = JSON.parse(storedState)
            if (parsedState?.state?.user && parsedState?.state?.isAuthenticated) {
              // We have persisted auth state, restore it immediately
              set({
                user: parsedState.state.user,
                isAuthenticated: parsedState.state.isAuthenticated,
                isInitialized: false, // Keep false until we validate
              })
            }
          } catch (error) {
            console.error('Error parsing stored auth state:', error)
          }
        }

        // Then validate with the server
        await state.validateAuth()
      },
    }),
    {
      name: 'auth-storage',
      storage: createJSONStorage(() => localStorage),
      partialize: state => ({
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
)
