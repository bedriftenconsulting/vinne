import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import PosTerminals from '@/pages/PosTerminals'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/pos-terminals')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <PosTerminals />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
