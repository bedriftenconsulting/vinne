import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import Transactions from '@/pages/Transactions'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/transactions')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <Transactions />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
