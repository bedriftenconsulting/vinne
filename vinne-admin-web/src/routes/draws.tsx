import { createFileRoute, redirect } from '@tanstack/react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import Draws from '@/pages/Draws'

const queryClient = new QueryClient()

// Define search params schema
type DrawsSearch = {
  page?: number
  status?: string
  game?: string
  startDate?: string
  endDate?: string
}

export const Route = createFileRoute('/draws')({
  validateSearch: (search: Record<string, unknown>): DrawsSearch => {
    return {
      page: Number(search?.page) || undefined,
      status: (search?.status as string) || undefined,
      game: (search?.game as string) || undefined,
      startDate: (search?.startDate as string) || undefined,
      endDate: (search?.endDate as string) || undefined,
    }
  },
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  beforeLoad: ({ context }: any) => {
    // Check if user is authenticated
    if (!context.auth?.isAuthenticated) {
      throw redirect({
        to: '/login',
        search: {
          redirect: '/draws',
        },
      })
    }
  },
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <Draws />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
