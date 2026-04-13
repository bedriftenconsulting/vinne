import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import AgentProfile from '@/pages/AgentProfile'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/agent/$agentId')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <AgentProfile />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
