import { createFileRoute, redirect } from '@tanstack/react-router'
import AdminLayout from '@/components/layouts/AdminLayout'
import DrawDetails from '@/pages/DrawDetails'

export const Route = createFileRoute('/draw/$drawId')({
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  beforeLoad: ({ context }: any) => {
    // Check if user is authenticated
    if (!context.auth?.isAuthenticated) {
      throw redirect({
        to: '/login',
        search: {
          redirect: '/draws',
        },
      })
    }
  },
  component: () => (
    <AdminLayout>
      <DrawDetails />
    </AdminLayout>
  ),
})
