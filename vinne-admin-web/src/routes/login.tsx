import { createFileRoute, redirect } from '@tanstack/react-router'
import Login from '@/pages/Login'

export const Route = createFileRoute('/login')({
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  beforeLoad: ({ context }: any) => {
    // If user is already authenticated, redirect to dashboard
    if (context.auth?.isAuthenticated) {
      throw redirect({
        to: '/dashboard',
      })
    }
  },
  component: Login,
})
