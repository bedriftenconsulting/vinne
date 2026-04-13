import { createFileRoute, redirect } from '@tanstack/react-router'
import Games from '@/pages/Games'

export const Route = createFileRoute('/games')({
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  beforeLoad: ({ context }: any) => {
    // Check if user is authenticated
    if (!context.auth?.isAuthenticated) {
      throw redirect({
        to: '/login',
        search: {
          redirect: '/games',
        },
      })
    }
    // Remove role check for now since the user object doesn't have roles property
    // TODO: Implement proper role checking when roles are available
  },
  component: Games,
})
