import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth'

export const Route = createFileRoute('/')({
  beforeLoad: async () => {
    const { isInitialized, initializeAuth } = useAuthStore.getState()

    if (!isInitialized) {
      await initializeAuth()
    }

    const authState = useAuthStore.getState()
    if (authState.isAuthenticated) {
      throw redirect({ to: '/dashboard' })
    } else {
      throw redirect({ to: '/login' })
    }
  },
  component: () => <div>Redirecting...</div>,
})
