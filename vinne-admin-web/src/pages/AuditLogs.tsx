import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { adminService } from '@/services/admin'
import { User, Activity } from 'lucide-react'

export default function AuditLogs() {
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState({
    user_id: '',
    action: '',
    resource: '',
    start_date: '',
    end_date: '',
  })

  const { data: auditData, isLoading } = useQuery({
    queryKey: ['admin-audit-logs', page, filters],
    queryFn: () => adminService.getAuditLogs(page, 20, filters),
  })

  // Removed usersData as it was not being used in the component

  const handleFilterChange = (key: string, value: string) => {
    setFilters(prev => ({ ...prev, [key]: value }))
    setPage(1)
  }

  const clearFilters = () => {
    setFilters({
      user_id: '',
      action: '',
      resource: '',
      start_date: '',
      end_date: '',
    })
    setPage(1)
  }

  const getActionBadgeVariant = (action: string) => {
    switch (action.toLowerCase()) {
      case 'create':
        return 'default'
      case 'update':
        return 'secondary'
      case 'delete':
        return 'destructive'
      case 'login':
        return 'outline'
      default:
        return 'outline'
    }
  }

  const getResourceBadgeVariant = (resource: string) => {
    switch (resource?.toLowerCase()) {
      case 'user':
        return 'default'
      case 'role':
        return 'secondary'
      case 'permission':
        return 'outline'
      default:
        return 'outline'
    }
  }

  const formatRequestData = (data: Record<string, unknown>) => {
    if (!data || Object.keys(data).length === 0) return 'None'

    const filteredData = Object.fromEntries(
      Object.entries(data).filter(
        ([key, value]) => key !== 'password' && value !== null && value !== undefined
      )
    )

    return JSON.stringify(filteredData, null, 2)
  }

  if (isLoading) {
    return <div className="p-6">Loading...</div>
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Audit Logs</h1>
        <Badge variant="secondary">
          <Activity className="mr-1 h-3 w-3" />
          {auditData?.total_count || 0} Total Events
        </Badge>
      </div>

      <Card>
        <CardHeader>
          <div className="grid grid-cols-1 md:grid-cols-6 gap-4">
            <div className="md:col-span-2">
              <Label htmlFor="user-filter">User</Label>
              <Input
                placeholder="Search by username..."
                value={filters.user_id}
                onChange={e => handleFilterChange('user_id', e.target.value)}
              />
            </div>
            <div>
              <Label htmlFor="action-filter">Action</Label>
              <Input
                placeholder="Action (create, update, delete...)"
                value={filters.action}
                onChange={e => handleFilterChange('action', e.target.value)}
              />
            </div>
            <div>
              <Label htmlFor="resource-filter">Resource</Label>
              <Input
                placeholder="Resource (user, role, permission...)"
                value={filters.resource}
                onChange={e => handleFilterChange('resource', e.target.value)}
              />
            </div>
            <div>
              <Label htmlFor="start-date">Start Date</Label>
              <Input
                id="start-date"
                type="date"
                value={filters.start_date}
                onChange={e => handleFilterChange('start_date', e.target.value)}
              />
            </div>
            <div className="flex items-end gap-2">
              <div className="flex-1">
                <Label htmlFor="end-date">End Date</Label>
                <Input
                  id="end-date"
                  type="date"
                  value={filters.end_date}
                  onChange={e => handleFilterChange('end_date', e.target.value)}
                />
              </div>
              <Button variant="outline" onClick={clearFilters}>
                Clear
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Timestamp</TableHead>
                <TableHead>User</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Resource</TableHead>
                <TableHead>IP Address</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Details</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {Array.isArray(auditData?.data) &&
                auditData.data.map(log => (
                  <TableRow key={log.id}>
                    <TableCell className="font-mono text-sm">
                      {new Date(log.created_at).toLocaleString()}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center space-x-2">
                        <User className="h-4 w-4" />
                        <div>
                          <div className="font-medium">{log.admin_user?.username || 'Unknown'}</div>
                          <div className="text-sm text-muted-foreground">
                            {log.admin_user?.email || 'N/A'}
                          </div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={getActionBadgeVariant(log.action)} className="capitalize">
                        {log.action}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {log.resource ? (
                        <Badge
                          variant={getResourceBadgeVariant(log.resource)}
                          className="capitalize"
                        >
                          {log.resource}
                          {log.resource_id && (
                            <span className="ml-1 text-xs opacity-75">
                              #{log.resource_id.slice(0, 8)}
                            </span>
                          )}
                        </Badge>
                      ) : (
                        <span className="text-muted-foreground">-</span>
                      )}
                    </TableCell>
                    <TableCell className="font-mono text-sm">{log.ip_address}</TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          log.response_status >= 200 && log.response_status < 300
                            ? 'default'
                            : 'destructive'
                        }
                      >
                        {log.response_status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <details className="cursor-pointer">
                        <summary className="text-sm font-medium hover:text-primary">
                          View Details
                        </summary>
                        <div className="mt-2 p-2 bg-muted rounded text-xs">
                          <div className="mb-2">
                            <strong>User Agent:</strong>
                            <pre className="whitespace-pre-wrap break-all">{log.user_agent}</pre>
                          </div>
                          <div>
                            <strong>Request Data:</strong>
                            <pre className="whitespace-pre-wrap">
                              {formatRequestData(log.request_data)}
                            </pre>
                          </div>
                        </div>
                      </details>
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>

          {auditData && (
            <div className="flex items-center justify-between space-x-2 py-4">
              <div className="text-sm text-muted-foreground">
                Showing {(page - 1) * auditData.page_size + 1} to{' '}
                {Math.min(page * auditData.page_size, auditData.total_count)} of{' '}
                {auditData.total_count} events
              </div>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage(p => Math.max(1, p - 1))}
                  disabled={page <= 1}
                >
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage(p => p + 1)}
                  disabled={page >= auditData.total_pages}
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
