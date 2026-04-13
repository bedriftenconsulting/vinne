import { RouterProvider, createRouter } from '@tanstack/react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth'
import { routeTree } from './routeTree.gen'
import { Toaster } from '@/components/ui/toaster'
import { useEffect } from 'react'

// Create a new router instance
const router = createRouter({
  routeTree,
  context: {
    auth: undefined!,
  },
})

// Register the router instance for type safety
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

// Create a query client
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5, // 5 minutes
      retry: 1,
    },
  },
})

function App() {
  const auth = useAuthStore()
  const initializeAuth = useAuthStore(state => state.initializeAuth)

  // Initialize authentication once on app load
  useEffect(() => {
    initializeAuth()
  }, [initializeAuth])

  // Show loading state while auth is being validated
  if (!auth.isInitialized) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <img
            src="/spiel_logo.png"
            alt="Spiel"
            className="h-16 w-16 mx-auto mb-4 animate-pulse"
          />
          <p className="text-gray-600">Validating authentication...</p>
        </div>
      </div>
    )
  }

  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} context={{ auth }} />
      <Toaster />
    </QueryClientProvider>
  )
}

export default App
