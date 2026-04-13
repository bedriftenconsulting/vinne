import { useEffect, useState } from 'react'
import { Link, useLocation } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import {
  Home,
  Gamepad2,
  Trophy,
  DollarSign,
  FileText,
  Settings,
  LogOut,
  Menu,
  X,
  Bell,
  ChevronDown,
  Shield,
  UserCog,
  Key,
  Activity,
  Building2,
  Store,
  Wallet,
  Monitor,
  Users,
} from 'lucide-react'

interface AdminLayoutProps {
  children: React.ReactNode
}

const navigation = [
  { name: 'Dashboard', href: '/dashboard', icon: Home, disabled: false },
  { name: 'Games', href: '/games', icon: Gamepad2, disabled: false },
  { name: 'Draws', href: '/draws', icon: Trophy, disabled: false },
  { name: 'Agents', href: '/admin/agents', icon: Building2, disabled: false },
  { name: 'Players', href: '/admin/players', icon: Users, disabled: false },
  { name: 'Retailers', href: '/admin/retailers', icon: Store, disabled: false },
  { name: 'Wallet Credits', href: '/admin/wallet-credits', icon: Wallet, disabled: false },
  { name: 'Transactions', href: '/admin/transactions', icon: DollarSign, disabled: false },
  { name: 'POS Terminals', href: '/admin/pos-terminals', icon: Monitor, disabled: true },
  { name: 'Reports', href: '/reports', icon: FileText, disabled: true },
]

const adminNavigation = [
  { name: 'Admin Users', href: '/admin/users', icon: UserCog },
  { name: 'Roles', href: '/admin/roles', icon: Shield },
  { name: 'Permissions', href: '/admin/permissions', icon: Key },
  { name: 'Audit Logs', href: '/admin/audit-logs', icon: Activity },
]

const settingsNavigation = [{ name: 'Settings', href: '/settings', icon: Settings }]

export default function AdminLayout({ children }: AdminLayoutProps) {
  const { user, adminLogout } = useAuthStore()
  const location = useLocation()
  const [sidebarOpen, setSidebarOpen] = useState(false)

  // Ensure mobile sidebar/backdrop cannot remain stuck across route transitions.
  useEffect(() => {
    setSidebarOpen(false)
  }, [location.pathname])

  const handleLogout = async () => {
    await adminLogout()
  }

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Mobile sidebar backdrop */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black bg-opacity-50 pointer-events-auto lg:hidden lg:pointer-events-none"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <div
        className={`fixed inset-y-0 left-0 z-50 w-72 sm:w-64 bg-white shadow-lg transform transition-transform duration-300 ease-in-out lg:translate-x-0 lg:static lg:inset-0 ${
          sidebarOpen
            ? 'translate-x-0 pointer-events-auto'
            : '-translate-x-full pointer-events-none lg:pointer-events-auto'
        }`}
      >
        <div className="flex h-full flex-col overflow-y-auto">
          {/* Logo */}
          <div className="flex h-16 items-center justify-between px-4 border-b shrink-0">
            <div className="flex items-center min-w-0">
              <img
                src="/spiel_logo.png"
                alt="Spiel"
                className="h-8 w-8 mr-2 sm:mr-3 shrink-0"
              />
              <span className="text-base sm:text-xl font-semibold text-gray-900 truncate">
                Spiel
              </span>
            </div>
            <button
              onClick={() => setSidebarOpen(false)}
              className="lg:hidden text-gray-500 hover:text-gray-700 shrink-0"
              aria-label="Close sidebar"
            >
              <X className="h-6 w-6" />
            </button>
          </div>

          {/* Navigation */}
          <nav className="flex-1 space-y-1 px-2 py-4">
            {/* Main Navigation */}
            {navigation.map(item => {
              const isActive =
                location.pathname === item.href || location.pathname.startsWith(item.href + '/')

              if (item.disabled) {
                return (
                  <div
                    key={item.name}
                    className="flex items-center px-3 py-2 text-sm font-medium rounded-md cursor-not-allowed opacity-50 text-gray-400"
                    title="Coming soon"
                  >
                    <item.icon className="mr-3 h-5 w-5" />
                    {item.name}
                  </div>
                )
              }

              return (
                <Link
                  key={item.name}
                  to={item.href}
                  className={`flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors ${
                    isActive
                      ? 'bg-blue-50 text-blue-700'
                      : 'text-gray-700 hover:text-gray-900 hover:bg-gray-50'
                  }`}
                  onClick={() => setSidebarOpen(false)}
                >
                  <item.icon className="mr-3 h-5 w-5" />
                  {item.name}
                </Link>
              )
            })}

            {/* Admin Section */}
            <div className="pt-6">
              <p className="px-3 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                Administration
              </p>
              <div className="mt-2 space-y-1">
                {adminNavigation.map(item => {
                  const isActive =
                    location.pathname === item.href || location.pathname.startsWith(item.href + '/')
                  return (
                    <Link
                      key={item.name}
                      to={item.href}
                      className={`flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors ${
                        isActive
                          ? 'bg-blue-50 text-blue-700'
                          : 'text-gray-700 hover:text-gray-900 hover:bg-gray-50'
                      }`}
                      onClick={() => setSidebarOpen(false)}
                    >
                      <item.icon className="mr-3 h-5 w-5" />
                      {item.name}
                    </Link>
                  )
                })}
              </div>
            </div>

            {/* Settings Section */}
            <div className="pt-6">
              {settingsNavigation.map(item => {
                const isActive =
                  location.pathname === item.href || location.pathname.startsWith(item.href + '/')
                return (
                  <Link
                    key={item.name}
                    to={item.href}
                    className={`flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors ${
                      isActive
                        ? 'bg-blue-50 text-blue-700'
                        : 'text-gray-700 hover:text-gray-900 hover:bg-gray-50'
                    }`}
                    onClick={() => setSidebarOpen(false)}
                  >
                    <item.icon className="mr-3 h-5 w-5" />
                    {item.name}
                  </Link>
                )
              })}
            </div>
          </nav>

          {/* User info */}
          <div className="border-t p-4 shrink-0">
            <div className="flex items-center gap-3">
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-gray-900 truncate">{user?.username}</p>
                <p className="text-xs text-gray-500 capitalize truncate">
                  {user?.roles?.[0]?.name?.replace('_', ' ') || 'Admin'}
                </p>
              </div>
              <Button variant="ghost" size="sm" onClick={handleLogout} aria-label="Logout">
                <LogOut className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Top header */}
        <header className="bg-white shadow-sm border-b shrink-0">
          <div className="flex items-center justify-between px-3 sm:px-4 md:px-6 py-3">
            <button
              onClick={() => setSidebarOpen(true)}
              className="text-gray-500 hover:text-gray-700 lg:hidden p-2 -ml-2"
              aria-label="Open sidebar"
            >
              <Menu className="h-6 w-6" />
            </button>

            <div className="flex-1 flex items-center justify-end">
              <div className="flex items-center space-x-2 sm:space-x-4">
                <Button variant="ghost" size="sm" aria-label="Notifications">
                  <Bell className="h-5 w-5" />
                </Button>

                <div className="hidden sm:flex items-center space-x-2">
                  <span className="text-sm text-gray-700 truncate max-w-32">{user?.username}</span>
                  <ChevronDown className="h-4 w-4 text-gray-500" />
                </div>
              </div>
            </div>
          </div>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto">{children}</main>
      </div>
    </div>
  )
}
