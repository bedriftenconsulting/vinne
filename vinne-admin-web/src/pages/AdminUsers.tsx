import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
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
import {
  adminService,
  type User,
  type CreateUserRequest,
  type UpdateUserRequest,
} from '@/services/admin'
import { useToast } from '@/hooks/use-toast'
import { getErrorMessage } from '@/lib/utils'
import { Trash2, Edit, Plus, Search } from 'lucide-react'

export default function AdminUsers() {
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [isEditOpen, setIsEditOpen] = useState(false)
  const [isDeleteOpen, setIsDeleteOpen] = useState(false)
  const [editingUser, setEditingUser] = useState<User | null>(null)
  const [deletingUser, setDeletingUser] = useState<User | null>(null)
  const [formData, setFormData] = useState<CreateUserRequest>({
    email: '',
    username: '',
    password: '',
    first_name: '',
    last_name: '',
    role_ids: [],
    ip_whitelist: [],
  })

  const { toast } = useToast()
  const queryClient = useQueryClient()

  const { data: usersData, isLoading } = useQuery({
    queryKey: ['admin-users', page, search],
    queryFn: () => adminService.getUsers(page, 20, { username: search }),
  })

  const { data: rolesData } = useQuery({
    queryKey: ['admin-roles-all'],
    queryFn: () => adminService.getAllRoles(),
  })

  const createMutation = useMutation({
    mutationFn: (data: CreateUserRequest) => adminService.createUser(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
      setIsCreateOpen(false)
      resetForm()
      toast({ title: 'User created successfully' })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error creating user',
        description: getErrorMessage(error),
        variant: 'destructive',
      })
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateUserRequest }) =>
      adminService.updateUser(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
      setIsEditOpen(false)
      setEditingUser(null)
      resetForm()
      toast({ title: 'User updated successfully' })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error updating user',
        description: getErrorMessage(error),
        variant: 'destructive',
      })
    },
  })

  // Role assignment mutations - will be used when role assignment UI is implemented
  // const assignRoleMutation = useMutation({
  //   mutationFn: ({ userId, roleId }: { userId: string; roleId: string }) =>
  //     adminService.assignRoleToUser(userId, roleId),
  //   onSuccess: () => {
  //     queryClient.invalidateQueries({ queryKey: ['admin-users'] })
  //     toast({ title: 'Role assigned successfully' })
  //   },
  //   onError: (error: unknown) => {
  //     toast({
  //       title: 'Error assigning role',
  //       description: (error as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Something went wrong',
  //       variant: 'destructive',
  //     })
  //   },
  // })

  // const removeRoleMutation = useMutation({
  //   mutationFn: ({ userId, roleId }: { userId: string; roleId: string }) =>
  //     adminService.removeRoleFromUser(userId, roleId),
  //   onSuccess: () => {
  //     queryClient.invalidateQueries({ queryKey: ['admin-users'] })
  //     toast({ title: 'Role removed successfully' })
  //   },
  //   onError: (error: unknown) => {
  //     toast({
  //       title: 'Error removing role',
  //       description: (error as { response?: { data?: { message?: string } } })?.response?.data?.message || 'Something went wrong',
  //       variant: 'destructive',
  //     })
  //   },
  // })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => adminService.deleteUser(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
      setIsDeleteOpen(false)
      setDeletingUser(null)
      toast({ title: 'User deleted successfully' })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error deleting user',
        description: getErrorMessage(error),
        variant: 'destructive',
      })
    },
  })

  const resetForm = () => {
    setFormData({
      email: '',
      username: '',
      password: '',
      role_ids: [],
    })
  }

  const handleEdit = (user: User) => {
    setEditingUser(user)
    setFormData({
      email: user.email,
      username: user.username,
      password: '',
      first_name: user.first_name,
      last_name: user.last_name,
      role_ids: user.roles?.map(role => role.id) || [],
      ip_whitelist: user.ip_whitelist || [],
    })
    setIsEditOpen(true)
  }

  const handleSubmit = () => {
    if (editingUser) {
      const updateData: UpdateUserRequest = {
        email: formData.email,
        username: formData.username,
        first_name: formData.first_name,
        last_name: formData.last_name,
        ip_whitelist: formData.ip_whitelist,
        role_ids: formData.role_ids,
      }
      updateMutation.mutate({ id: editingUser.id, data: updateData })
    } else {
      createMutation.mutate(formData)
    }
  }

  const handleRoleToggle = (roleId: string) => {
    setFormData(prev => ({
      ...prev,
      role_ids: (prev.role_ids || []).includes(roleId)
        ? (prev.role_ids || []).filter(id => id !== roleId)
        : [...(prev.role_ids || []), roleId],
    }))
  }

  const handleDelete = (user: User) => {
    setDeletingUser(user)
    setIsDeleteOpen(true)
  }

  const confirmDelete = () => {
    if (deletingUser) {
      deleteMutation.mutate(deletingUser.id)
    }
  }

  if (isLoading) {
    return <div className="p-6">Loading...</div>
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Admin Users</h1>
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              Add User
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Create New User</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              <div>
                <Label htmlFor="email">
                  Email <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="email"
                  type="email"
                  value={formData.email}
                  onChange={e => setFormData(prev => ({ ...prev, email: e.target.value }))}
                  placeholder="user@example.com"
                />
              </div>
              <div>
                <Label htmlFor="username">
                  Username <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="username"
                  value={formData.username}
                  onChange={e => setFormData(prev => ({ ...prev, username: e.target.value }))}
                  placeholder="username"
                />
              </div>
              <div>
                <Label htmlFor="password">
                  Password <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="password"
                  type="password"
                  value={formData.password}
                  onChange={e => setFormData(prev => ({ ...prev, password: e.target.value }))}
                  placeholder="Password"
                />
              </div>
              <div>
                <Label htmlFor="first_name">First Name</Label>
                <Input
                  id="first_name"
                  value={formData.first_name || ''}
                  onChange={e => setFormData(prev => ({ ...prev, first_name: e.target.value }))}
                  placeholder="First name"
                />
              </div>
              <div>
                <Label htmlFor="last_name">Last Name</Label>
                <Input
                  id="last_name"
                  value={formData.last_name || ''}
                  onChange={e => setFormData(prev => ({ ...prev, last_name: e.target.value }))}
                  placeholder="Last name"
                />
              </div>
              <div>
                <Label>Roles</Label>
                <div className="space-y-2 mt-2">
                  {Array.isArray(rolesData) &&
                    rolesData.map(role => (
                      <div key={role.id} className="flex items-center space-x-2">
                        <Switch
                          checked={(formData.role_ids || []).includes(role.id)}
                          onCheckedChange={() => handleRoleToggle(role.id)}
                        />
                        <Label>{role.name}</Label>
                        {role.description && (
                          <span className="text-sm text-muted-foreground">
                            - {role.description}
                          </span>
                        )}
                      </div>
                    ))}
                </div>
              </div>
              <div className="flex gap-2 pt-4">
                <Button
                  onClick={handleSubmit}
                  disabled={createMutation.isPending}
                  className="flex-1"
                >
                  {createMutation.isPending ? 'Creating...' : 'Create User'}
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
            </div>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center space-x-2">
            <Search className="h-4 w-4" />
            <Input
              placeholder="Search users..."
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="max-w-sm"
            />
          </div>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Username</TableHead>
                <TableHead>Email</TableHead>
                <TableHead>Name</TableHead>
                <TableHead>Roles</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {Array.isArray(usersData?.data) &&
                usersData.data.map(user => (
                  <TableRow key={user.id}>
                    <TableCell className="font-medium">{user.username}</TableCell>
                    <TableCell>{user.email}</TableCell>
                    <TableCell>
                      {[user.first_name, user.last_name].filter(Boolean).join(' ') || '-'}
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {user.roles?.map(role => (
                          <Badge key={role.id} variant="secondary">
                            {role.name}
                          </Badge>
                        )) || <span className="text-muted-foreground">No roles</span>}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={user.is_active ? 'default' : 'secondary'}>
                        {user.is_active ? 'Active' : 'Inactive'}
                      </Badge>
                    </TableCell>
                    <TableCell>{new Date(user.created_at).toLocaleDateString()}</TableCell>
                    <TableCell>
                      <div className="flex gap-2">
                        <Button variant="outline" size="sm" onClick={() => handleEdit(user)}>
                          <Edit className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleDelete(user)}
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

          {usersData && (
            <div className="flex items-center justify-between space-x-2 py-4">
              <div className="text-sm text-muted-foreground">
                Showing {(page - 1) * usersData.page_size + 1} to{' '}
                {Math.min(page * usersData.page_size, usersData.total_count)} of{' '}
                {usersData.total_count} users
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
                  disabled={page >= usersData.total_pages}
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={isEditOpen} onOpenChange={setIsEditOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Edit User</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label htmlFor="edit-email">Email</Label>
              <Input
                id="edit-email"
                type="email"
                value={formData.email}
                onChange={e => setFormData(prev => ({ ...prev, email: e.target.value }))}
              />
            </div>
            <div>
              <Label htmlFor="edit-username">Username</Label>
              <Input
                id="edit-username"
                value={formData.username}
                onChange={e => setFormData(prev => ({ ...prev, username: e.target.value }))}
              />
            </div>
            <div>
              <Label htmlFor="edit-first-name">First Name</Label>
              <Input
                id="edit-first-name"
                value={formData.first_name || ''}
                onChange={e => setFormData(prev => ({ ...prev, first_name: e.target.value }))}
                placeholder="First name"
              />
            </div>
            <div>
              <Label htmlFor="edit-last-name">Last Name</Label>
              <Input
                id="edit-last-name"
                value={formData.last_name || ''}
                onChange={e => setFormData(prev => ({ ...prev, last_name: e.target.value }))}
                placeholder="Last name"
              />
            </div>
            <div>
              <Label>Roles</Label>
              <div className="space-y-2 mt-2">
                {Array.isArray(rolesData) &&
                  rolesData.map(role => (
                    <div key={role.id} className="flex items-center space-x-2">
                      <Switch
                        checked={(formData.role_ids || []).includes(role.id)}
                        onCheckedChange={() => handleRoleToggle(role.id)}
                      />
                      <Label>{role.name}</Label>
                      {role.description && (
                        <span className="text-sm text-muted-foreground">- {role.description}</span>
                      )}
                    </div>
                  ))}
              </div>
            </div>
            <div className="flex gap-2 pt-4">
              <Button onClick={handleSubmit} disabled={updateMutation.isPending} className="flex-1">
                {updateMutation.isPending ? 'Updating...' : 'Update User'}
              </Button>
              <Button
                variant="outline"
                onClick={() => {
                  setIsEditOpen(false)
                  setEditingUser(null)
                  resetForm()
                }}
                className="flex-1"
              >
                Cancel
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={isDeleteOpen} onOpenChange={setIsDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete User</DialogTitle>
          </DialogHeader>
          <div className="py-4">
            <p>
              Are you sure you want to delete the user{' '}
              <span className="font-semibold">"{deletingUser?.username}"</span>?
            </p>
            <p className="text-sm text-muted-foreground mt-2">
              This action cannot be undone and will permanently remove the user and all their data.
            </p>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setIsDeleteOpen(false)
                setDeletingUser(null)
              }}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={confirmDelete}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? 'Deleting...' : 'Delete User'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
