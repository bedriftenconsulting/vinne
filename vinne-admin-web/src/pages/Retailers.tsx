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
  type Retailer,
  type CreateRetailerRequest,
  type UpdateRetailerRequest,
  type UpdateStatusRequest,
} from '@/services/agents'
import { useToast } from '@/hooks/use-toast'
import { getErrorMessage } from '@/lib/utils'
import { Edit, Plus, Search, MoreHorizontal } from 'lucide-react'
import { Link } from '@tanstack/react-router'

export default function Retailers() {
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [agentFilter, setAgentFilter] = useState<string>('all')
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [isEditOpen, setIsEditOpen] = useState(false)
  const [isStatusOpen, setIsStatusOpen] = useState(false)
  const [isPinOpen, setIsPinOpen] = useState(false)
  const [createdRetailerPin, setCreatedRetailerPin] = useState<{ code: string; pin: string } | null>(null)
  const [editingRetailer, setEditingRetailer] = useState<Retailer | null>(null)
  const [statusRetailer, setStatusRetailer] = useState<Retailer | null>(null)
  const [formData, setFormData] = useState<CreateRetailerRequest>({
    name: '',
    email: '',
    phone_number: '',
    address: '',
    agent_id: 'none',
    created_by: 'admin',
  })

  const { toast } = useToast()
  const queryClient = useQueryClient()

  const { data: retailersData } = useQuery({
    queryKey: ['retailers', page, search, statusFilter, agentFilter],
    queryFn: () =>
      agentService.getRetailers(page, 20, {
        name: search,
        status: statusFilter === 'all' ? undefined : statusFilter,
        agent_id: agentFilter === 'all' ? undefined : agentFilter,
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

  const { data: agentsData } = useQuery({
    queryKey: ['agents-list'],
    queryFn: () => agentService.getAgents(1, 1000, {}), // Load all agents, not just active ones
  })

  const createMutation = useMutation({
    mutationFn: (data: CreateRetailerRequest) => agentService.createRetailer(data),
    onSuccess: (response: any) => {
      queryClient.invalidateQueries({ queryKey: ['retailers'] })
      setIsCreateOpen(false)
      resetForm()
      if (response?.initial_pin) {
        setCreatedRetailerPin({ code: response.retailer?.retailer_code || '', pin: response.initial_pin })
        setIsPinOpen(true)
      } else {
        toast({ title: 'Retailer created successfully' })
      }
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error creating retailer',
        description: getErrorMessage(error),
        variant: 'destructive',
      })
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateRetailerRequest }) =>
      agentService.updateRetailer(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['retailers'] })
      setIsEditOpen(false)
      setEditingRetailer(null)
      resetForm()
      toast({ title: 'Retailer updated successfully' })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error updating retailer',
        description: getErrorMessage(error),
        variant: 'destructive',
      })
    },
  })

  const statusMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateStatusRequest }) =>
      agentService.updateRetailerStatus(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['retailers'] })
      setIsStatusOpen(false)
      setStatusRetailer(null)
      toast({ title: 'Retailer status updated successfully' })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error updating retailer status',
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
      agent_id: 'none',
      created_by: 'admin',
    })
  }

  const handleEdit = (retailer: Retailer) => {
    setEditingRetailer(retailer)
    setFormData({
      name: retailer.name,
      email: retailer.email,
      phone_number: retailer.phone_number,
      address: retailer.address,
      agent_id: retailer.agent_id || 'none',
      created_by: 'admin',
    })
    setIsEditOpen(true)
  }

  const handleStatusChange = (retailer: Retailer) => {
    setStatusRetailer(retailer)
    setIsStatusOpen(true)
  }

  const handleSubmit = () => {
    if (editingRetailer) {
      const updateData: UpdateRetailerRequest = {
        name: formData.name,
        email: formData.email,
        phone_number: formData.phone_number,
        address: formData.address,
        updated_by: 'admin',
      }
      updateMutation.mutate({ id: editingRetailer.id, data: updateData })
    } else {
      const createData = {
        ...formData,
        agent_id: formData.agent_id === 'none' ? '' : formData.agent_id,
      }
      createMutation.mutate(createData)
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

  const getAgentName = (agentId?: string) => {
    if (!agentId || agentId === '' || agentId === null) return 'Independent'
    const agent = agentsData?.data?.find(a => a.id === agentId)
    return agent ? `${agent.name} (${agent.agent_code})` : 'Unknown Agent'
  }

  return (
    <div className="p-3 sm:p-4 md:p-6 space-y-3 sm:space-y-4 md:space-y-6">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <h1 className="text-xl sm:text-2xl md:text-3xl font-bold">Retailers</h1>
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogTrigger asChild>
            <Button className="w-full sm:w-auto">
              <Plus className="mr-2 h-3 sm:h-4 w-3 sm:w-4" />
              <span className="hidden sm:inline">Add Retailer</span>
              <span className="sm:hidden">Add</span>
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Create New Retailer</DialogTitle>
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
                  placeholder="Retailer name"
                />
              </div>
              <div>
                <Label htmlFor="email">Email (Optional)</Label>
                <Input
                  id="email"
                  type="email"
                  value={formData.email}
                  onChange={e => setFormData(prev => ({ ...prev, email: e.target.value }))}
                  placeholder="retailer@business.com (optional)"
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
                <Label htmlFor="address">Address (Optional)</Label>
                <Textarea
                  id="address"
                  value={formData.address}
                  onChange={e => setFormData(prev => ({ ...prev, address: e.target.value }))}
                  placeholder="Business address (optional)"
                />
              </div>
              <div>
                <Label htmlFor="agent">Managing Agent</Label>
                <Select
                  value={formData.agent_id}
                  onValueChange={value => setFormData(prev => ({ ...prev, agent_id: value }))}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Select managing agent" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">Independent (No agent)</SelectItem>
                    {agentsData?.data?.map(agent => (
                      <SelectItem key={agent.id} value={agent.id}>
                        {agent.name} ({agent.agent_code})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex gap-2 pt-4">
                <Button
                  onClick={handleSubmit}
                  disabled={createMutation.isPending}
                  className="flex-1"
                >
                  {createMutation.isPending ? 'Creating...' : 'Create Retailer'}
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
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <h3 className="text-xs sm:text-sm font-medium">Total Retailers</h3>
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">
              {retailersData?.pagination?.total_count || 0}
            </div>
            <p className="text-xs text-muted-foreground">Registered retailers</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <h3 className="text-xs sm:text-sm font-medium">Active Retailers</h3>
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">
              {Array.isArray(retailersData?.data)
                ? retailersData.data.filter(r => r.status === 'ACTIVE' || String(r.status) === '1')
                    .length
                : 0}
            </div>
            <p className="text-xs text-muted-foreground">Currently active</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <h3 className="text-xs sm:text-sm font-medium">Independent</h3>
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">
              {Array.isArray(retailersData?.data)
                ? retailersData.data.filter(r => !r.agent_id || r.agent_id === '').length
                : 0}
            </div>
            <p className="text-xs text-muted-foreground">No managing agent</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <h3 className="text-xs sm:text-sm font-medium">Agent-Managed</h3>
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">
              {Array.isArray(retailersData?.data)
                ? retailersData.data.filter(r => r.agent_id && r.agent_id !== '').length
                : 0}
            </div>
            <p className="text-xs text-muted-foreground">Under agent management</p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 sm:gap-3 md:gap-4">
            <div className="flex items-center gap-2 flex-1 min-w-0">
              <Search className="h-3 sm:h-4 w-3 sm:w-4 shrink-0" />
              <Input
                placeholder="Search retailers..."
                value={search}
                onChange={e => setSearch(e.target.value)}
                className="w-full sm:max-w-sm text-xs sm:text-sm"
              />
            </div>
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="w-full sm:w-40 md:w-48 text-xs sm:text-sm">
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
            <Select value={agentFilter} onValueChange={setAgentFilter}>
              <SelectTrigger className="w-full sm:w-40 md:w-48 text-xs sm:text-sm">
                <SelectValue placeholder="Filter by agent" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All agents</SelectItem>
                <SelectItem value="independent">Independent</SelectItem>
                {agentsData?.data?.map(agent => (
                  <SelectItem key={agent.id} value={agent.id}>
                    {agent.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-xs sm:text-sm">Retailer Code</TableHead>
                  <TableHead className="text-xs sm:text-sm">Retailer Name</TableHead>
                  <TableHead className="text-xs sm:text-sm">Email</TableHead>
                  <TableHead className="text-xs sm:text-sm">Phone</TableHead>
                  <TableHead className="text-xs sm:text-sm">Managing Agent</TableHead>
                  <TableHead className="text-xs sm:text-sm">Status</TableHead>
                  <TableHead className="text-xs sm:text-sm">Created</TableHead>
                  <TableHead className="text-xs sm:text-sm">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {Array.isArray(retailersData?.data) && retailersData.data.length > 0 ? (
                  retailersData.data.map(retailer => (
                    <TableRow key={retailer.id}>
                      <TableCell className="font-medium text-xs sm:text-sm">
                        {retailer.retailer_code}
                      </TableCell>
                      <TableCell className="text-xs sm:text-sm">
                        <Link
                          to="/admin/retailer/$retailerId"
                          params={{ retailerId: retailer.id }}
                          className="text-blue-600 hover:text-blue-800 hover:underline font-medium"
                        >
                          {retailer.name}
                        </Link>
                      </TableCell>
                      <TableCell className="text-xs sm:text-sm">{retailer.email}</TableCell>
                      <TableCell className="text-xs sm:text-sm">{retailer.phone_number}</TableCell>
                      <TableCell className="text-xs sm:text-sm">
                        {getAgentName(retailer.agent_id)}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={getStatusBadgeVariant(String(retailer.status))}
                          className="text-xs"
                        >
                          {String(retailer.status) === '1' || retailer.status === 'ACTIVE'
                            ? 'Active'
                            : getStatusLabel(String(retailer.status))}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs sm:text-sm">
                        {retailer.created_at
                          ? (() => {
                              try {
                                const date = new Date(retailer.created_at)
                                return isNaN(date.getTime())
                                  ? 'N/A'
                                  : date.toLocaleDateString('en-GB', {
                                      day: '2-digit',
                                      month: 'short',
                                      year: 'numeric',
                                    })
                              } catch {
                                return 'N/A'
                              }
                            })()
                          : 'N/A'}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-2">
                          <Button variant="outline" size="sm" onClick={() => handleEdit(retailer)}>
                            <Edit className="h-3 sm:h-4 w-3 sm:w-4" />
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleStatusChange(retailer)}
                          >
                            <MoreHorizontal className="h-3 sm:h-4 w-3 sm:w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={8} className="text-center py-8 text-muted-foreground">
                      {search || statusFilter !== 'all' || agentFilter !== 'all'
                        ? 'No retailers found matching your filters.'
                        : 'No retailers available yet.'}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>

          {retailersData?.pagination && (
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 py-4">
              <div className="text-xs sm:text-sm text-muted-foreground">
                Showing {(page - 1) * retailersData.pagination.page_size + 1} to{' '}
                {Math.min(
                  page * retailersData.pagination.page_size,
                  retailersData.pagination.total_count
                )}{' '}
                of {retailersData.pagination.total_count} retailers
              </div>
              <div className="flex gap-2 w-full sm:w-auto justify-between sm:justify-end">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage(p => Math.max(1, p - 1))}
                  disabled={page <= 1}
                  className="text-xs sm:text-sm"
                >
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage(p => p + 1)}
                  disabled={page >= (retailersData.pagination.total_pages || 1)}
                  className="text-xs sm:text-sm"
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
            <DialogTitle>Edit Retailer</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label htmlFor="edit-name">Retailer Name</Label>
              <Input
                id="edit-name"
                value={formData.name}
                onChange={e => setFormData(prev => ({ ...prev, name: e.target.value }))}
              />
            </div>
            <div>
              <Label htmlFor="edit-email">Email (Optional)</Label>
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
              <Label htmlFor="edit-address">Address (Optional)</Label>
              <Textarea
                id="edit-address"
                value={formData.address}
                onChange={e => setFormData(prev => ({ ...prev, address: e.target.value }))}
              />
            </div>
            <div className="flex gap-2 pt-4">
              <Button onClick={handleSubmit} disabled={updateMutation.isPending} className="flex-1">
                {updateMutation.isPending ? 'Updating...' : 'Update Retailer'}
              </Button>
              <Button
                variant="outline"
                onClick={() => {
                  setIsEditOpen(false)
                  setEditingRetailer(null)
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

      <Dialog open={isStatusOpen} onOpenChange={setIsStatusOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Update Retailer Status</DialogTitle>
          </DialogHeader>
          <div className="py-4">
            <p className="mb-4">
              Update status for retailer:{' '}
              <span className="font-semibold">{statusRetailer?.name}</span>
            </p>
            <div className="space-y-4">
              <div>
                <Label>Current Status</Label>
                <Badge variant={getStatusBadgeVariant(statusRetailer?.status || '')}>
                  {getStatusLabel(statusRetailer?.status || '')}
                </Badge>
              </div>
              <div>
                <Label htmlFor="new-status">New Status</Label>
                <Select>
                  <SelectTrigger>
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
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setIsStatusOpen(false)
                setStatusRetailer(null)
              }}
            >
              Cancel
            </Button>
            <Button
              onClick={() => {
                if (statusRetailer) {
                  statusMutation.mutate({
                    id: statusRetailer.id,
                    data: { status: 'ACTIVE' },
                  })
                }
              }}
              disabled={statusMutation.isPending}
            >
              {statusMutation.isPending ? 'Updating...' : 'Update Status'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* PIN dialog - shown once after retailer creation */}
      <Dialog open={isPinOpen} onOpenChange={setIsPinOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Retailer Created Successfully</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <p className="text-sm text-muted-foreground">
              Share these POS login credentials with the retailer. The PIN cannot be retrieved again.
            </p>
            <div className="rounded-md bg-muted p-4 space-y-2 font-mono text-sm">
              <div>Retailer Code: <span className="font-bold">{createdRetailerPin?.code}</span></div>
              <div>PIN: <span className="font-bold">{createdRetailerPin?.pin}</span></div>
            </div>
            <p className="text-xs text-muted-foreground">
              The retailer uses their retailer code + PIN to log into the POS terminal.
            </p>
          </div>
          <DialogFooter>
            <Button onClick={() => {
              navigator.clipboard?.writeText(`Retailer Code: ${createdRetailerPin?.code}\nPIN: ${createdRetailerPin?.pin}`)
              toast({ title: 'Credentials copied to clipboard' })
            }} variant="outline">Copy</Button>
            <Button onClick={() => { setIsPinOpen(false); setCreatedRetailerPin(null) }}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
