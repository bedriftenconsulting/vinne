import { redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth'

interface LocationContext {
  href: string
}

export async function requireAuth({ location }: { context: unknown; location: LocationContext }) {
  // Read directly from Zustand store — always has the latest state
  const { isInitialized, initializeAuth } = useAuthStore.getState()

  // If not yet initialized, trigger initialization and wait for it
  if (!isInitialized) {
    await initializeAuth()
  }

  // Re-read after potential initialization
  const authState = useAuthStore.getState()
  if (!authState.isAuthenticated) {
    throw redirect({
      to: '/login',
      search: {
        redirect: location.href,
      },
    })
  }
}
