import { createFileRoute } from '@tanstack/react-router'
import Dashboard from '@/pages/Dashboard'
import { requireAuth } from '@/utils/authCheck'

export const Route = createFileRoute('/dashboard')({
  beforeLoad: requireAuth,
  component: Dashboard,
})
