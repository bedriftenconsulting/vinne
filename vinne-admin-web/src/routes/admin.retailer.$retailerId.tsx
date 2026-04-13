import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import RetailerProfile from '@/pages/RetailerProfile'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/retailer/$retailerId')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <RetailerProfile />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
