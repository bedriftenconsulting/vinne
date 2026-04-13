import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import AdminUsers from '@/pages/AdminUsers'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/users')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <AdminUsers />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
