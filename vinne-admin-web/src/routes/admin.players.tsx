import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import Players from '@/pages/Players'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/players')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <Players />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
