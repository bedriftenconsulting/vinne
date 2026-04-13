import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import Agents from '@/pages/Agents'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/agents')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <Agents />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
