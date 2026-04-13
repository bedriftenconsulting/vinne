import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import Retailers from '@/pages/Retailers'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/retailers')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <Retailers />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
