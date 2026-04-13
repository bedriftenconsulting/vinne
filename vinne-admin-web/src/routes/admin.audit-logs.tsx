import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import AuditLogs from '@/pages/AuditLogs'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/audit-logs')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <AuditLogs />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
