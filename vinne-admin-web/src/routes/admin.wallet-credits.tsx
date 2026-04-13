import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/utils/authCheck'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AdminLayout from '@/components/layouts/AdminLayout'
import WalletCredits from '@/pages/WalletCredits'

const queryClient = new QueryClient()

export const Route = createFileRoute('/admin/wallet-credits')({
  beforeLoad: requireAuth,
  component: () => (
    <QueryClientProvider client={queryClient}>
      <AdminLayout>
        <WalletCredits />
      </AdminLayout>
    </QueryClientProvider>
  ),
})
