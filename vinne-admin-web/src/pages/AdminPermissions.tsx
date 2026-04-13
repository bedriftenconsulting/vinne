import { useQuery } from '@tanstack/react-query'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { adminService, type Permission } from '@/services/admin'

export default function AdminPermissions() {
  const { data: permissionsData, isLoading } = useQuery({
    queryKey: ['admin-permissions-all'],
    queryFn: () => adminService.getAllPermissions(),
  })

  const groupPermissionsByResource = (permissions: Permission[]) => {
    const groups: Record<string, Permission[]> = {}
    if (Array.isArray(permissions)) {
      permissions.forEach(permission => {
        if (!groups[permission.resource]) {
          groups[permission.resource] = []
        }
        groups[permission.resource].push(permission)
      })
    }
    return groups
  }

  if (isLoading) {
    return <div className="p-6">Loading...</div>
  }

  if (!permissionsData) {
    return <div className="p-6">No permissions data available.</div>
  }

  const groupedPermissions = groupPermissionsByResource(permissionsData || [])

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Permissions</h1>
        <Badge variant="secondary">
          {Array.isArray(permissionsData) ? permissionsData.length : 0} Total Permissions
        </Badge>
      </div>

      <div className="grid gap-6">
        {Object.entries(groupedPermissions).map(([resource, permissions]) => (
          <Card key={resource}>
            <CardHeader>
              <CardTitle className="capitalize">{resource} Permissions</CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Action</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead>Permission Key</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {permissions.map(permission => (
                    <TableRow key={permission.id}>
                      <TableCell>
                        <Badge variant="outline" className="capitalize">
                          {permission.action}
                        </Badge>
                      </TableCell>
                      <TableCell>{permission.description || 'No description available'}</TableCell>
                      <TableCell>
                        <code className="bg-muted px-2 py-1 rounded text-sm">
                          {permission.resource}:{permission.action}
                        </code>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>All Permissions Overview</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Resource</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Permission Key</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {Array.isArray(permissionsData) &&
                permissionsData.map(permission => (
                  <TableRow key={permission.id}>
                    <TableCell>
                      <Badge variant="secondary" className="capitalize">
                        {permission.resource}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="capitalize">
                        {permission.action}
                      </Badge>
                    </TableCell>
                    <TableCell>{permission.description || 'No description available'}</TableCell>
                    <TableCell>
                      <code className="bg-muted px-2 py-1 rounded text-sm">
                        {permission.resource}:{permission.action}
                      </code>
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
