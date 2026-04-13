import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import PlayerProfile from '@/pages/PlayerProfile'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/player/$playerId')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <PlayerProfile />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
