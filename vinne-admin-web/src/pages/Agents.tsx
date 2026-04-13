import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  agentService,
  type Agent,
  type CreateAgentRequest,
  type UpdateAgentRequest,
  type UpdateStatusRequest,
} from '@/services/agents'
import { useToast } from '@/hooks/use-toast'
import { getErrorMessage } from '@/lib/utils'
import { Edit, Plus, Search, MoreHorizontal } from 'lucide-react'

export default function Agents() {
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [isEditOpen, setIsEditOpen] = useState(false)
  const [isStatusOpen, setIsStatusOpen] = useState(false)
  const [isPasswordOpen, setIsPasswordOpen] = useState(false)
  const [createdAgentPassword, setCreatedAgentPassword] = useState<{ code: string; password: string } | null>(null)
  const [editingAgent, setEditingAgent] = useState<Agent | null>(null)
  const [statusAgent, setStatusAgent] = useState<Agent | null>(null)
  const [newStatus, setNewStatus] = useState<string>('')
  const [formData, setFormData] = useState<CreateAgentRequest>({
    name: '',
    email: '',
    phone_number: '',
    address: '',
    commission_percentage: 5,
    created_by: 'admin',
  })

  const { toast } = useToast()
  const queryClient = useQueryClient()

  const { data: agentsData } = useQuery({
    queryKey: ['agents', page, search, statusFilter],
    queryFn: () =>
      agentService.getAgents(page, 20, {
        name: search,
        status: statusFilter === 'all' ? undefined : statusFilter,
      }),
    placeholderData: {
      data: [],
      pagination: {
        page: 1,
        page_size: 20,
        total_count: 0,
        total_pages: 1,
      },
    },
    retry: 0,
    refetchOnWindowFocus: false,
  })

  // Removed commission tiers query - now using simple percentage on agents

  const createMutation = useMutation({
    mutationFn: (data: CreateAgentRequest) => agentService.createAgent(data),
    onSuccess: (response) => {
      queryClient.invalidateQueries({ queryKey: ['agents'] })
      setIsCreateOpen(false)
      resetForm()
      if (response?.initial_password) {
        setCreatedAgentPassword({ code: response.agent_code, password: response.initial_password })
        setIsPasswordOpen(true)
      } else {
        toast({ title: 'Agent created successfully' })
      }
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error creating agent',
        description: getErrorMessage(error),
        variant: 'destructive',
      })
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateAgentRequest }) =>
      agentService.updateAgent(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agents'] })
      setIsEditOpen(false)
      setEditingAgent(null)
      resetForm()
      toast({ title: 'Agent updated successfully' })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error updating agent',
        description: getErrorMessage(error),
        variant: 'destructive',
      })
    },
  })

  const statusMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateStatusRequest }) =>
      agentService.updateAgentStatus(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agents'] })
      setIsStatusOpen(false)
      setStatusAgent(null)
      setNewStatus('')
      toast({ title: 'Agent status updated successfully' })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error updating agent status',
        description: getErrorMessage(error),
        variant: 'destructive',
      })
    },
  })

  const resetForm = () => {
    setFormData({
      name: '',
      email: '',
      phone_number: '',
      address: '',
      commission_percentage: 5,
      created_by: 'admin',
    })
  }

  const handleEdit = (agent: Agent) => {
    setEditingAgent(agent)
    setFormData({
      name: agent.name,
      email: agent.email,
      phone_number: agent.phone_number,
      address: agent.address,
      commission_percentage: agent.commission_percentage || 5,
      created_by: 'admin',
    })
    setIsEditOpen(true)
  }

  const handleStatusChange = (agent: Agent) => {
    setStatusAgent(agent)
    setNewStatus(agent.status) // Initialize with current status
    setIsStatusOpen(true)
  }

  const handleSubmit = () => {
    if (editingAgent) {
      const updateData: UpdateAgentRequest = {
        name: formData.name,
        email: formData.email,
        phone_number: formData.phone_number,
        address: formData.address,
        commission_percentage: formData.commission_percentage,
        updated_by: 'admin',
      }
      updateMutation.mutate({ id: editingAgent.id, data: updateData })
    } else {
      createMutation.mutate(formData)
    }
  }

  const getStatusBadgeVariant = (status: string) => {
    switch (status) {
      case 'ACTIVE':
        return 'default'
      case 'SUSPENDED':
        return 'destructive'
      case 'UNDER_REVIEW':
        return 'secondary'
      case 'INACTIVE':
        return 'outline'
      case 'TERMINATED':
        return 'destructive'
      default:
        return 'secondary'
    }
  }

  const getStatusLabel = (status: string) => {
    switch (status) {
      case 'ACTIVE':
        return 'Active'
      case 'SUSPENDED':
        return 'Suspended'
      case 'UNDER_REVIEW':
        return 'Under Review'
      case 'INACTIVE':
        return 'Inactive'
      case 'TERMINATED':
        return 'Terminated'
      default:
        return status
    }
  }

  return (
    <div className="p-3 sm:p-4 md:p-6 space-y-3 sm:space-y-4 md:space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <h1 className="text-2xl sm:text-3xl font-bold">Agents</h1>
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              Add Agent
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Create New Agent</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              <div>
                <Label htmlFor="name">
                  Name <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="name"
                  value={formData.name}
                  onChange={e => setFormData(prev => ({ ...prev, name: e.target.value }))}
                  placeholder="Agent name"
                />
              </div>
              <div>
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  value={formData.email}
                  onChange={e => setFormData(prev => ({ ...prev, email: e.target.value }))}
                  placeholder="contact@business.com"
                />
              </div>
              <div>
                <Label htmlFor="phone_number">
                  Phone Number <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="phone_number"
                  value={formData.phone_number}
                  onChange={e => setFormData(prev => ({ ...prev, phone_number: e.target.value }))}
                  placeholder="+233123456789"
                />
              </div>
              <div>
                <Label htmlFor="address">
                  Address <span className="text-red-500">*</span>
                </Label>
                <Textarea
                  id="address"
                  value={formData.address}
                  onChange={e => setFormData(prev => ({ ...prev, address: e.target.value }))}
                  placeholder="Agent address"
                />
              </div>
              <div>
                <Label htmlFor="commission_percentage">Commission Percentage (%)</Label>
                <Input
                  id="commission_percentage"
                  type="number"
                  min="0"
                  max="100"
                  step="0.1"
                  value={formData.commission_percentage}
                  onChange={e =>
                    setFormData(prev => ({
                      ...prev,
                      commission_percentage: parseFloat(e.target.value) || 0,
                    }))
                  }
                  placeholder="5"
                />
                <p className="text-xs text-muted-foreground mt-1">
                  Enter percentage (e.g., 5 for 5%)
                </p>
              </div>
              <div className="flex gap-2 pt-4">
                <Button
                  onClick={handleSubmit}
                  disabled={createMutation.isPending}
                  className="flex-1"
                >
                  {createMutation.isPending ? 'Creating...' : 'Create Agent'}
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

      {/* Stats Cards */}
      <div className="grid gap-3 sm:gap-4 grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 p-3 sm:p-6 sm:pb-2">
            <h3 className="text-xs sm:text-sm font-medium">Total Agents</h3>
          </CardHeader>
          <CardContent className="p-3 sm:p-6 pt-0">
            <div className="text-lg sm:text-2xl font-bold">
              {agentsData?.pagination?.total_count || 0}
            </div>
            <p className="text-xs text-muted-foreground">Registered</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 p-3 sm:p-6 sm:pb-2">
            <h3 className="text-xs sm:text-sm font-medium">Active Agents</h3>
          </CardHeader>
          <CardContent className="p-3 sm:p-6 pt-0">
            <div className="text-lg sm:text-2xl font-bold">
              {Array.isArray(agentsData?.data)
                ? agentsData.data.filter(a => a.status === 'ACTIVE').length
                : 0}
            </div>
            <p className="text-xs text-muted-foreground">Active</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 p-3 sm:p-6 sm:pb-2">
            <h3 className="text-xs sm:text-sm font-medium">Suspended</h3>
          </CardHeader>
          <CardContent className="p-3 sm:p-6 pt-0">
            <div className="text-lg sm:text-2xl font-bold">
              {Array.isArray(agentsData?.data)
                ? agentsData.data.filter(a => a.status === 'SUSPENDED').length
                : 0}
            </div>
            <p className="text-xs text-muted-foreground">Suspended</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 p-3 sm:p-6 sm:pb-2">
            <h3 className="text-xs sm:text-sm font-medium">Under Review</h3>
          </CardHeader>
          <CardContent className="p-3 sm:p-6 pt-0">
            <div className="text-lg sm:text-2xl font-bold">
              {Array.isArray(agentsData?.data)
                ? agentsData.data.filter(a => a.status === 'UNDER_REVIEW').length
                : 0}
            </div>
            <p className="text-xs text-muted-foreground">Reviewing</p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="p-3 sm:p-6">
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2">
            <div className="flex items-center space-x-2 flex-1">
              <Search className="h-4 w-4 shrink-0" />
              <Input
                placeholder="Search agents..."
                value={search}
                onChange={e => setSearch(e.target.value)}
                className="flex-1 sm:max-w-sm"
              />
            </div>
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="w-full sm:w-48">
                <SelectValue placeholder="Filter by status" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All statuses</SelectItem>
                <SelectItem value="active">Active</SelectItem>
                <SelectItem value="suspended">Suspended</SelectItem>
                <SelectItem value="under_review">Under Review</SelectItem>
                <SelectItem value="inactive">Inactive</SelectItem>
                <SelectItem value="terminated">Terminated</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent className="p-0 sm:p-6 sm:pt-0">
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-xs sm:text-sm">Agent Code</TableHead>
                  <TableHead className="text-xs sm:text-sm">Business Name</TableHead>
                  <TableHead className="text-xs sm:text-sm hidden md:table-cell">Email</TableHead>
                  <TableHead className="text-xs sm:text-sm hidden lg:table-cell">Phone</TableHead>
                  <TableHead className="text-xs sm:text-sm">Status</TableHead>
                  <TableHead className="text-xs sm:text-sm hidden sm:table-cell">Created</TableHead>
                  <TableHead className="text-xs sm:text-sm">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {Array.isArray(agentsData?.data) && agentsData.data.length > 0 ? (
                  agentsData.data.map(agent => (
                    <TableRow key={agent.id}>
                      <TableCell className="font-medium text-xs sm:text-sm py-2 sm:py-4">
                        {agent.agent_code}
                      </TableCell>
                      <TableCell className="py-2 sm:py-4">
                        <Link
                          to="/admin/agent/$agentId"
                          params={{ agentId: agent.id }}
                          className="text-blue-600 hover:text-blue-800 hover:underline font-medium text-xs sm:text-sm truncate max-w-32 sm:max-w-none inline-block"
                        >
                          {agent.name}
                        </Link>
                      </TableCell>
                      <TableCell className="text-xs sm:text-sm hidden md:table-cell py-2 sm:py-4">
                        {agent.email}
                      </TableCell>
                      <TableCell className="text-xs sm:text-sm hidden lg:table-cell py-2 sm:py-4">
                        {agent.phone_number}
                      </TableCell>
                      <TableCell className="py-2 sm:py-4">
                        <Badge variant={getStatusBadgeVariant(agent.status)} className="text-xs">
                          {getStatusLabel(agent.status)}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs sm:text-sm hidden sm:table-cell py-2 sm:py-4">
                        {new Date(agent.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell className="py-2 sm:py-4">
                        <div className="flex gap-1 sm:gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleEdit(agent)}
                            className="h-7 w-7 sm:h-8 sm:w-8 p-0"
                          >
                            <Edit className="h-3 w-3 sm:h-4 sm:w-4" />
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleStatusChange(agent)}
                            className="h-7 w-7 sm:h-8 sm:w-8 p-0"
                          >
                            <MoreHorizontal className="h-3 w-3 sm:h-4 sm:w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                      {search || statusFilter !== 'all'
                        ? 'No agents found matching your filters.'
                        : 'No agents available yet.'}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>

          {agentsData?.pagination && (
            <div className="flex flex-col sm:flex-row items-center justify-between gap-3 sm:gap-2 py-3 sm:py-4 px-3 sm:px-0">
              <div className="text-xs sm:text-sm text-muted-foreground text-center sm:text-left">
                <span className="hidden sm:inline">
                  Showing {(page - 1) * agentsData.pagination.page_size + 1} to{' '}
                </span>
                <span className="hidden sm:inline">
                  {Math.min(
                    page * agentsData.pagination.page_size,
                    agentsData.pagination.total_count
                  )}{' '}
                  of{' '}
                </span>
                <span className="sm:hidden">
                  Page {page} of {agentsData.pagination.total_pages || 1} (
                  {agentsData.pagination.total_count} total)
                </span>
                <span className="hidden sm:inline">{agentsData.pagination.total_count} agents</span>
              </div>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage(p => Math.max(1, p - 1))}
                  disabled={page <= 1}
                  className="text-xs sm:text-sm h-8 sm:h-9"
                >
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage(p => p + 1)}
                  disabled={page >= (agentsData.pagination.total_pages || 1)}
                  className="text-xs sm:text-sm h-8 sm:h-9"
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
            <DialogTitle>Edit Agent</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label htmlFor="edit-name">Business Name</Label>
              <Input
                id="edit-name"
                value={formData.name}
                onChange={e => setFormData(prev => ({ ...prev, name: e.target.value }))}
              />
            </div>
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
              <Label htmlFor="edit-phone">Phone Number</Label>
              <Input
                id="edit-phone"
                value={formData.phone_number}
                onChange={e => setFormData(prev => ({ ...prev, phone_number: e.target.value }))}
              />
            </div>
            <div>
              <Label htmlFor="edit-address">Address</Label>
              <Textarea
                id="edit-address"
                value={formData.address}
                onChange={e => setFormData(prev => ({ ...prev, address: e.target.value }))}
              />
            </div>
            <div>
              <Label htmlFor="edit-commission-percentage">Commission Percentage (%)</Label>
              <Input
                id="edit-commission-percentage"
                type="number"
                min="0"
                max="100"
                step="0.1"
                value={formData.commission_percentage}
                onChange={e =>
                  setFormData(prev => ({
                    ...prev,
                    commission_percentage: parseFloat(e.target.value) || 0,
                  }))
                }
                placeholder="5"
              />
              <p className="text-xs text-muted-foreground mt-1">
                Enter percentage (e.g., 5 for 5%)
              </p>
            </div>
            <div className="flex gap-2 pt-4">
              <Button onClick={handleSubmit} disabled={updateMutation.isPending} className="flex-1">
                {updateMutation.isPending ? 'Updating...' : 'Update Agent'}
              </Button>
              <Button
                variant="outline"
                onClick={() => {
                  setIsEditOpen(false)
                  setEditingAgent(null)
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

      <Dialog open={isStatusOpen} onOpenChange={setIsStatusOpen}>        <DialogContent>
          <DialogHeader>
            <DialogTitle>Update Agent Status</DialogTitle>
          </DialogHeader>
          <div className="py-4">
            <p className="mb-4">
              Update status for agent: <span className="font-semibold">{statusAgent?.name}</span>
            </p>
            <div className="space-y-4">
              <div>
                <Label>Current Status</Label>
                <div className="mt-1">
                  <Badge variant={getStatusBadgeVariant(statusAgent?.status || '')}>
                    {getStatusLabel(statusAgent?.status || '')}
                  </Badge>
                </div>
              </div>
              <div>
                <Label htmlFor="new-status">New Status</Label>
                <Select value={newStatus} onValueChange={setNewStatus}>
                  <SelectTrigger className="mt-1">
                    <SelectValue placeholder="Select new status" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="ACTIVE">Active</SelectItem>
                    <SelectItem value="SUSPENDED">Suspended</SelectItem>
                    <SelectItem value="UNDER_REVIEW">Under Review</SelectItem>
                    <SelectItem value="INACTIVE">Inactive</SelectItem>
                    <SelectItem value="TERMINATED">Terminated</SelectItem>
                  </SelectContent>
                </Select>
                {newStatus && newStatus !== statusAgent?.status && (
                  <p className="text-sm text-muted-foreground mt-2">
                    Changing from <strong>{getStatusLabel(statusAgent?.status || '')}</strong> to{' '}
                    <strong>{getStatusLabel(newStatus)}</strong>
                  </p>
                )}
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setIsStatusOpen(false)
                setStatusAgent(null)
                setNewStatus('')
              }}
            >
              Cancel
            </Button>
            <Button
              onClick={() => {
                if (statusAgent && newStatus) {
                  statusMutation.mutate({
                    id: statusAgent.id,
                    data: {
                      status: newStatus as
                        | 'ACTIVE'
                        | 'SUSPENDED'
                        | 'UNDER_REVIEW'
                        | 'INACTIVE'
                        | 'TERMINATED',
                    },
                  })
                }
              }}
              disabled={statusMutation.isPending || !newStatus || newStatus === statusAgent?.status}
            >
              {statusMutation.isPending ? 'Updating...' : 'Update Status'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Initial password dialog - shown once after agent creation */}
      <Dialog open={isPasswordOpen} onOpenChange={setIsPasswordOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Agent Created Successfully</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <p className="text-sm text-muted-foreground">
              Share these login credentials with the agent. The password cannot be retrieved again.
            </p>
            <div className="rounded-md bg-muted p-4 space-y-2 font-mono text-sm">
              <div>Agent Code: <span className="font-bold">{createdAgentPassword?.code}</span></div>
              <div>Password: <span className="font-bold">{createdAgentPassword?.password}</span></div>
            </div>
            <p className="text-xs text-muted-foreground">
              The agent can log in using their phone number or agent code with this password.
            </p>
          </div>
          <DialogFooter>
            <Button onClick={() => {
              navigator.clipboard?.writeText(`Agent Code: ${createdAgentPassword?.code}\nPassword: ${createdAgentPassword?.password}`)
              toast({ title: 'Credentials copied to clipboard' })
            }} variant="outline">Copy</Button>
            <Button onClick={() => { setIsPasswordOpen(false); setCreatedAgentPassword(null) }}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
