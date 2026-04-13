import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
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
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  adminService,
  type Role,
  type CreateRoleRequest,
  type UpdateRoleRequest,
  type Permission,
} from '@/services/admin'
import { useToast } from '@/hooks/use-toast'
import { getErrorMessage } from '@/lib/utils'
import { Trash2, Edit, Plus } from 'lucide-react'

export default function AdminRoles() {
  const [page, setPage] = useState(1)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [isEditOpen, setIsEditOpen] = useState(false)
  const [isDeleteOpen, setIsDeleteOpen] = useState(false)
  const [editingRole, setEditingRole] = useState<Role | null>(null)
  const [deletingRole, setDeletingRole] = useState<Role | null>(null)
  const [formData, setFormData] = useState<CreateRoleRequest>({
    name: '',
    description: '',
    permission_ids: [],
  })

  const { toast } = useToast()
  const queryClient = useQueryClient()

  const { data: rolesData, isLoading } = useQuery({
    queryKey: ['admin-roles', page],
    queryFn: () => adminService.getRoles(page, 20),
  })

  const { data: permissionsData } = useQuery({
    queryKey: ['admin-permissions-all'],
    queryFn: () => adminService.getAllPermissions(),
  })

  const createMutation = useMutation({
    mutationFn: (data: CreateRoleRequest) => adminService.createRole(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-roles'] })
      setIsCreateOpen(false)
      resetForm()
      toast({ title: 'Role created successfully' })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error creating role',
        description: getErrorMessage(error),
        variant: 'destructive',
      })
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateRoleRequest }) =>
      adminService.updateRole(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-roles'] })
      setIsEditOpen(false)
      setEditingRole(null)
      resetForm()
      toast({ title: 'Role updated successfully' })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error updating role',
        description: getErrorMessage(error),
        variant: 'destructive',
      })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => adminService.deleteRole(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-roles'] })
      setIsDeleteOpen(false)
      setDeletingRole(null)
      toast({ title: 'Role deleted successfully' })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error deleting role',
        description: getErrorMessage(error),
        variant: 'destructive',
      })
    },
  })

  const resetForm = () => {
    setFormData({
      name: '',
      description: '',
      permission_ids: [],
    })
  }

  const handleEdit = (role: Role) => {
    setEditingRole(role)
    setFormData({
      name: role.name,
      description: role.description || '',
      permission_ids: Array.isArray(role.permissions)
        ? role.permissions.map(permission => permission.id)
        : [],
    })
    setIsEditOpen(true)
  }

  const handleSubmit = () => {
    if (editingRole) {
      const updateData: UpdateRoleRequest = {
        name: formData.name,
        description: formData.description,
        permission_ids: formData.permission_ids,
      }
      updateMutation.mutate({ id: editingRole.id, data: updateData })
    } else {
      createMutation.mutate(formData)
    }
  }

  const handlePermissionToggle = (permissionId: string) => {
    setFormData(prev => ({
      ...prev,
      permission_ids: prev.permission_ids.includes(permissionId)
        ? prev.permission_ids.filter(id => id !== permissionId)
        : [...prev.permission_ids, permissionId],
    }))
  }

  const handleDelete = (role: Role) => {
    setDeletingRole(role)
    setIsDeleteOpen(true)
  }

  const confirmDelete = () => {
    if (deletingRole) {
      deleteMutation.mutate(deletingRole.id)
    }
  }

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

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Roles Management</h1>
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              Add Role
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-2xl">
            <DialogHeader>
              <DialogTitle>Create New Role</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 max-h-96 overflow-y-auto">
              <div>
                <Label htmlFor="name">
                  Name <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="name"
                  value={formData.name}
                  onChange={e => setFormData(prev => ({ ...prev, name: e.target.value }))}
                  placeholder="Role name"
                />
              </div>
              <div>
                <Label htmlFor="description">
                  Description <span className="text-red-500">*</span>
                </Label>
                <Textarea
                  id="description"
                  value={formData.description}
                  onChange={e => setFormData(prev => ({ ...prev, description: e.target.value }))}
                  placeholder="Role description"
                />
              </div>
              <div>
                <Label>
                  Permissions <span className="text-red-500">*</span>
                </Label>
                <div className="space-y-4 mt-2">
                  {permissionsData &&
                    Object.entries(groupPermissionsByResource(permissionsData || [])).map(
                      ([resource, permissions]) => (
                        <div key={resource} className="border rounded p-3">
                          <h4 className="font-semibold mb-2 capitalize">{resource}</h4>
                          <div className="space-y-2">
                            {Array.isArray(permissions) &&
                              permissions.map(permission => (
                                <div key={permission.id} className="flex items-center space-x-2">
                                  <Switch
                                    checked={formData.permission_ids.includes(permission.id)}
                                    onCheckedChange={() => handlePermissionToggle(permission.id)}
                                  />
                                  <Label className="capitalize">{permission.action}</Label>
                                  {permission.description && (
                                    <span className="text-sm text-muted-foreground">
                                      - {permission.description}
                                    </span>
                                  )}
                                </div>
                              ))}
                          </div>
                        </div>
                      )
                    )}
                </div>
              </div>
            </div>
            <div className="flex gap-2 pt-4">
              <Button onClick={handleSubmit} disabled={createMutation.isPending} className="flex-1">
                {createMutation.isPending ? 'Creating...' : 'Create Role'}
              </Button>
              <Button
                variant="outline"
                onClick={() => {
                  setIsCreateOpen(false)
                  resetForm()
                }}
                className="flex-1"
              >
                Cancel
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Roles</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Permissions</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {Array.isArray(rolesData?.data) &&
                rolesData.data.map(role => (
                  <TableRow key={role.id}>
                    <TableCell className="font-medium">{role.name}</TableCell>
                    <TableCell>{role.description || 'No description'}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {Array.isArray(role.permissions) &&
                          role.permissions.slice(0, 3).map(permission => (
                            <Badge key={permission.id} variant="outline" className="text-xs">
                              {permission.resource}:{permission.action}
                            </Badge>
                          ))}
                        {role.permissions.length > 3 && (
                          <Badge variant="outline" className="text-xs">
                            +{role.permissions.length - 3} more
                          </Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>{new Date(role.created_at).toLocaleDateString()}</TableCell>
                    <TableCell>
                      <div className="flex gap-2">
                        <Button variant="outline" size="sm" onClick={() => handleEdit(role)}>
                          <Edit className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleDelete(role)}
                          disabled={deleteMutation.isPending}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>

          {rolesData && (
            <div className="flex items-center justify-between space-x-2 py-4">
              <div className="text-sm text-muted-foreground">
                Showing {(page - 1) * rolesData.page_size + 1} to{' '}
                {Math.min(page * rolesData.page_size, rolesData.total_count)} of{' '}
                {rolesData.total_count} roles
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
                  disabled={page >= rolesData.total_pages}
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={isEditOpen} onOpenChange={setIsEditOpen}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Edit Role</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 max-h-96 overflow-y-auto">
            <div>
              <Label htmlFor="edit-name">Name</Label>
              <Input
                id="edit-name"
                value={formData.name}
                onChange={e => setFormData(prev => ({ ...prev, name: e.target.value }))}
              />
            </div>
            <div>
              <Label htmlFor="edit-description">Description</Label>
              <Textarea
                id="edit-description"
                value={formData.description}
                onChange={e => setFormData(prev => ({ ...prev, description: e.target.value }))}
              />
            </div>
            <div>
              <Label>Permissions</Label>
              <div className="space-y-4 mt-2">
                {permissionsData &&
                  Object.entries(groupPermissionsByResource(permissionsData)).map(
                    ([resource, permissions]) => (
                      <div key={resource} className="border rounded p-3">
                        <h4 className="font-semibold mb-2 capitalize">{resource}</h4>
                        <div className="space-y-2">
                          {permissions.map(permission => (
                            <div key={permission.id} className="flex items-center space-x-2">
                              <Switch
                                checked={formData.permission_ids.includes(permission.id)}
                                onCheckedChange={() => handlePermissionToggle(permission.id)}
                              />
                              <Label className="capitalize">{permission.action}</Label>
                              {permission.description && (
                                <span className="text-sm text-muted-foreground">
                                  - {permission.description}
                                </span>
                              )}
                            </div>
                          ))}
                        </div>
                      </div>
                    )
                  )}
              </div>
            </div>
          </div>
          <div className="flex gap-2 pt-4">
            <Button onClick={handleSubmit} disabled={updateMutation.isPending} className="flex-1">
              {updateMutation.isPending ? 'Updating...' : 'Update Role'}
            </Button>
            <Button
              variant="outline"
              onClick={() => {
                setIsEditOpen(false)
                setEditingRole(null)
                resetForm()
              }}
              className="flex-1"
            >
              Cancel
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={isDeleteOpen} onOpenChange={setIsDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Role</DialogTitle>
          </DialogHeader>
          <div className="py-4">
            <p>
              Are you sure you want to delete the role{' '}
              <span className="font-semibold">"{deletingRole?.name}"</span>?
            </p>
            <p className="text-sm text-muted-foreground mt-2">
              This action cannot be undone and will remove all permissions associated with this
              role.
            </p>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setIsDeleteOpen(false)
                setDeletingRole(null)
              }}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={confirmDelete}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? 'Deleting...' : 'Delete Role'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
