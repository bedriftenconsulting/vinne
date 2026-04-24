import { useEffect } from 'react'
import { Link, useLocation } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth'
import {
  Home,
  Gamepad2,
  Trophy,
  DollarSign,
  FileText,
  Settings,
  LogOut,
  Shield,
  UserCog,
  Key,
  Activity,
  Monitor,
  Users,
  CreditCard,
  Phone,
} from 'lucide-react'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
  useSidebar,
} from '@/components/ui/sidebar'

interface AdminLayoutProps {
  children: React.ReactNode
}

const operationsNav = [
  { name: 'Dashboard', href: '/dashboard', icon: Home },
  { name: 'Games', href: '/games', icon: Gamepad2 },
  { name: 'Draws', href: '/draws', icon: Trophy },
  { name: 'Wins', href: '/wins', icon: DollarSign },
]

const commerceNav = [
  { name: 'Players', href: '/players', icon: Users },
  { name: 'Transactions', href: '/transactions', icon: CreditCard },
  { name: 'USSD Sessions', href: '/ussd-sessions', icon: Phone },
]

const configNav = [
]

const comingSoonNav = [
  { name: 'POS Terminals', href: '/admin/pos-terminals', icon: Monitor },
  { name: 'Reports', href: '/reports', icon: FileText },
]

const adminNav = [
  { name: 'Admin Users', href: '/admin/users', icon: UserCog },
  { name: 'Roles', href: '/admin/roles', icon: Shield },
  { name: 'Permissions', href: '/admin/permissions', icon: Key },
  { name: 'Audit Logs', href: '/admin/audit-logs', icon: Activity },
  { name: 'Winner Config', href: '/config/winner-selection', icon: Trophy },
  { name: 'Settings', href: '/settings', icon: Settings },
]

function AppSidebar() {
  const { user, adminLogout } = useAuthStore()
  const location = useLocation()
  const { state } = useSidebar()
  const collapsed = state === 'collapsed'

  // Role-based access
  // commerce_manager or manager role = Commerce only
  // everyone else (super_admin, admin, support, viewer) = sees everything
  const userRoles = user?.roles?.map(r => r.name.toLowerCase()) || []
  console.log('[AdminLayout] user roles:', userRoles)
  const isCommerceOnly = userRoles.length > 0 &&
    userRoles.every(r => r === 'commerce_manager' || r === 'manager')
  console.log('[AdminLayout] isCommerceOnly:', isCommerceOnly)
  const showOperations = !isCommerceOnly
  const showCommerce = true
  const showAdmin = !isCommerceOnly
  const showComingSoon = !isCommerceOnly

  const isActive = (href: string) =>
    location.pathname === href || location.pathname.startsWith(href + '/')

  const NavItem = ({ item }: { item: { name: string; href: string; icon: React.ElementType } }) => {
    const active = isActive(item.href)
    return (
      <SidebarMenuItem>
        <SidebarMenuButton asChild tooltip={item.name}>
          <Link
            to={item.href}
            className={[
              'flex items-center gap-2.5 px-3 py-2 rounded-md text-sm transition-colors',
              active
                ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium'
                : 'text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
            ].join(' ')}
          >
            <item.icon className="h-4 w-4 shrink-0" />
            {!collapsed && <span>{item.name}</span>}
          </Link>
        </SidebarMenuButton>
      </SidebarMenuItem>
    )
  }

  return (
    <Sidebar collapsible="icon">
      {/* Logo / Brand */}
      <SidebarHeader className="px-4 py-5 border-b border-sidebar-border">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-md overflow-hidden shrink-0 flex items-center justify-center">
            <img 
              src="/winbig-logo.png" 
              alt="WinBig Africa" 
              className="h-full w-full object-contain"
            />
          </div>
          {!collapsed && (
            <div className="min-w-0">
              <p className="text-sm font-semibold text-sidebar-primary tracking-[-0.02em] truncate">
                WinBig Africa
              </p>
              <p className="text-[10px] text-sidebar-muted">Admin Console</p>
            </div>
          )}
        </div>
      </SidebarHeader>

      <SidebarContent className="px-2 py-3">
        {/* Operations */}
        {showOperations && (
          <SidebarGroup>
            <SidebarGroupLabel className="text-[10px] font-semibold uppercase tracking-[0.1em] text-sidebar-muted px-3 mb-1">
              Operations
            </SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {operationsNav.map(item => (
                  <NavItem key={item.name} item={item} />
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        )}

        {/* Commerce */}
        {showCommerce && (
          <SidebarGroup className={showOperations ? "pt-4" : ""}>
            <SidebarGroupLabel className="text-[10px] font-semibold uppercase tracking-[0.1em] text-sidebar-muted px-3 mb-1">
              Commerce
            </SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {commerceNav.map(item => (
                  <NavItem key={item.name} item={item} />
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        )}

        {/* Administration */}
        {showAdmin && (
          <SidebarGroup className="pt-4">
            <SidebarGroupLabel className="text-[10px] font-semibold uppercase tracking-[0.1em] text-sidebar-muted px-3 mb-1">
              Administration
            </SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {adminNav.map(item => (
                  <NavItem key={item.name} item={item} />
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        )}

        {/* Coming Soon */}
        {showComingSoon && (
          <SidebarGroup className="pt-4">
            <SidebarGroupLabel className="text-[10px] font-semibold uppercase tracking-[0.1em] text-sidebar-muted px-3 mb-1">
              Coming Soon
            </SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {comingSoonNav.map(item => (
                  <SidebarMenuItem key={item.name}>
                    <SidebarMenuButton disabled tooltip={item.name} className="opacity-40 cursor-not-allowed">
                      <item.icon className="h-4 w-4 shrink-0" />
                      <span>{item.name}</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        )}
      </SidebarContent>

      {/* User footer */}
      <SidebarFooter className="border-t border-sidebar-border px-3 py-3">
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="h-7 w-7 rounded-md bg-sidebar-accent flex items-center justify-center shrink-0">
            <span className="text-xs font-medium text-sidebar-accent-foreground">
              {user?.username?.[0]?.toUpperCase() ?? 'A'}
            </span>
          </div>
          {!collapsed && (
            <div className="flex-1 min-w-0">
              <p className="text-xs font-medium text-sidebar-primary truncate">{user?.username}</p>
              <p className="text-[10px] text-sidebar-muted capitalize truncate">
                {user?.roles?.[0]?.name?.replace(/_/g, ' ') ?? 'Admin'}
              </p>
            </div>
          )}
          {!collapsed && (
            <button
              onClick={() => adminLogout()}
              className="shrink-0 text-sidebar-muted hover:text-sidebar-primary transition-colors"
              title="Logout"
            >
              <LogOut className="h-4 w-4" />
            </button>
          )}
        </div>
      </SidebarFooter>
    </Sidebar>
  )
}

export default function AdminLayout({ children }: AdminLayoutProps) {
  const location = useLocation()

  // Close mobile sidebar on route change is handled by SidebarProvider internally
  useEffect(() => {}, [location.pathname])

  return (
    <SidebarProvider>
      <div className="min-h-screen flex w-full">
        <AppSidebar />
        <div className="flex-1 flex flex-col min-w-0">
          {/* Top header */}
          <header className="h-12 flex items-center border-b border-border bg-card px-4 shrink-0">
            <SidebarTrigger className="mr-3" />
            <span className="text-xs text-muted-foreground font-mono">
              {new Date().toLocaleDateString('en-GH', {
                weekday: 'long',
                year: 'numeric',
                month: 'long',
                day: 'numeric',
              })}
            </span>
          </header>

          {/* Page content */}
          <main className="flex-1 overflow-auto">
            <div className="max-w-[1600px] mx-auto p-6">{children}</div>
          </main>
        </div>
      </div>
    </SidebarProvider>
  )
}
