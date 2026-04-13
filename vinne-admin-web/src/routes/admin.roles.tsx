import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import AdminRoles from '@/pages/AdminRoles'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/roles')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <AdminRoles />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
