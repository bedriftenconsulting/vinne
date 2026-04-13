import { useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth'

interface ProtectedRouteProps {
  children: React.ReactNode
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const navigate = useNavigate()
  const { isAuthenticated, isInitialized } = useAuthStore()

  useEffect(() => {
    // Only redirect if auth is initialized and user is not authenticated
    if (isInitialized && !isAuthenticated) {
      navigate({ to: '/login' })
    }
  }, [isAuthenticated, isInitialized, navigate])

  // Show loading while auth is being initialized
  if (!isInitialized) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-gray-900 mx-auto"></div>
          <p className="mt-4 text-gray-600">Loading...</p>
        </div>
      </div>
    )
  }

  // If not authenticated after initialization, don't render children
  if (!isAuthenticated) {
    return null
  }

  return <>{children}</>
}
