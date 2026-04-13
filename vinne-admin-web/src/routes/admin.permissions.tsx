import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import AdminPermissions from '@/pages/AdminPermissions'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/permissions')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <AdminPermissions />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
