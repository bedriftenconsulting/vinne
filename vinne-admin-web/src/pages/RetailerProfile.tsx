import { useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { useToast } from '@/hooks/use-toast'
import { getErrorMessage } from '@/lib/utils'
import { agentService, type UpdateRetailerRequest } from '@/services/agents'
import { walletService } from '@/services/wallet'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  ArrowLeft,
  User,
  Store,
  FileText,
  CreditCard,
  AlertCircle,
  Calendar,
  Mail,
  Phone,
  Building,
  Edit,
  Save,
  X,
  Monitor,
  Plus,
  Power,
  Activity,
  Ban,
  Lock,
  Unlock,
  ShieldOff,
} from 'lucide-react'
import { RetailerWalletCard } from '@/components/wallet/RetailerWalletCard'
import { CreditWalletDialog } from '@/components/wallet/CreditWalletDialog'
import { TransactionHistoryTable } from '@/components/wallet/TransactionHistoryTable'

export default function RetailerProfile() {
  const { retailerId } = useParams({ from: '/admin/retailer/$retailerId' })
  const navigate = useNavigate()
  const { toast } = useToast()
  const queryClient = useQueryClient()

  const [isEditing, setIsEditing] = useState(false)
  const [creditDialogOpen, setCreditDialogOpen] = useState(false)
  const [selectedWalletType, setSelectedWalletType] = useState<'stake' | 'winning'>('stake')
  const [editForm, setEditForm] = useState<UpdateRetailerRequest>({
    name: '',
    email: '',
    phone_number: '',
    address: '',
    updated_by: 'admin',
  })

  // Fetch retailer details
  const { data: retailer, isLoading } = useQuery({
    queryKey: ['retailer', retailerId],
    queryFn: () => agentService.getRetailerById(retailerId),
  })

  // Fetch wallet balances (both stake and winning)
  const { data: stakeBalance } = useQuery({
    queryKey: ['retailer-wallet-balance', retailerId, 'stake'],
    queryFn: () => walletService.getRetailerWalletBalance(retailerId, 'stake'),
    enabled: !!retailerId,
  })

  const { data: winningBalance } = useQuery({
    queryKey: ['retailer-wallet-balance', retailerId, 'winning'],
    queryFn: () => walletService.getRetailerWalletBalance(retailerId, 'winning'),
    enabled: !!retailerId,
  })

  // Fetch transaction history for the selected wallet type
  const { data: transactionHistory } = useQuery({
    queryKey: ['retailer-transactions', retailerId, selectedWalletType],
    queryFn: () =>
      walletService.getTransactionHistory(retailerId, 'retailer', {
        wallet_type: selectedWalletType,
        page: 1,
        page_size: 20,
      }),
    enabled: !!retailerId,
  })

  // Update retailer mutation
  const updateMutation = useMutation({
    mutationFn: (data: UpdateRetailerRequest) => agentService.updateRetailer(retailerId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['retailer', retailerId] })
      setIsEditing(false)
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

  // Credit wallet mutation
  const creditWalletMutation = useMutation({
    mutationFn: (data: {
      wallet_type: 'stake' | 'winning'
      amount: number
      credit_type: 'payment' | 'credit_loan'
      payment_method?: string
      reference?: string
      agent_id?: string
      notes?: string
    }) => walletService.creditRetailerWallet(retailerId, data),
    onSuccess: response => {
      queryClient.invalidateQueries({ queryKey: ['retailer-wallet-balance', retailerId] })
      queryClient.invalidateQueries({ queryKey: ['retailer-transactions', retailerId] })
      setCreditDialogOpen(false)
      toast({
        title: 'Wallet credited successfully',
        description: `Credited ${response.data.gross_amount / 100} GHS to ${selectedWalletType} wallet (Base: ${response.data.base_amount / 100} GHS + Commission: ${response.data.commission_amount / 100} GHS)`,
      })
    },
    onError: (error: unknown) => {
      toast({
        title: 'Error crediting wallet',
        description: getErrorMessage(error),
        variant: 'destructive',
      })
    },
  })

  const handleEdit = () => {
    if (retailer) {
      setEditForm({
        name: retailer.name,
        email: retailer.email,
        phone_number: retailer.phone_number,
        address: retailer.address,
        updated_by: 'admin',
      })
      setIsEditing(true)
    }
  }

  const handleSave = () => {
    updateMutation.mutate(editForm)
  }

  const handleCancel = () => {
    setIsEditing(false)
    if (retailer) {
      setEditForm({
        name: retailer.name,
        email: retailer.email,
        phone_number: retailer.phone_number,
        address: retailer.address,
        updated_by: 'admin',
      })
    }
  }

  const handleCreditWallet = (data: {
    amount: number
    payment_method: string
    reference?: string
    notes?: string
  }) => {
    creditWalletMutation.mutate({
      wallet_type: selectedWalletType,
      amount: data.amount,
      credit_type: 'payment',
      payment_method: data.payment_method || 'cash',
      reference: data.reference,
      agent_id: retailer?.agent_id,
      notes: data.notes,
    })
  }

  const handleSuspendRetailer = () => {
    const action = retailer?.status === 'SUSPENDED' ? 'reactivate' : 'suspend'
    // const newStatus = retailer?.status === 'SUSPENDED' ? 'ACTIVE' : 'SUSPENDED'

    toast({
      title: `${action === 'suspend' ? 'Suspending' : 'Reactivating'} retailer...`,
      description: `This will ${action} the retailer ${action === 'suspend' ? 'from' : 'for'} all operations.`,
    })

    // TODO: Implement API call to update retailer status
    // const newStatus = retailer?.status === 'SUSPENDED' ? 'ACTIVE' : 'SUSPENDED'
    // updateRetailerStatus(retailerId, newStatus)
  }

  const handleBlockWithdrawals = () => {
    toast({
      title: 'Blocking withdrawals...',
      description: 'This will prevent withdrawals from the winning wallet.',
    })

    // TODO: Implement API call to block withdrawals
    // blockRetailerWithdrawals(retailerId)
  }

  const handleUnblockWithdrawals = () => {
    toast({
      title: 'Unblocking withdrawals...',
      description: 'This will allow withdrawals from the winning wallet.',
    })

    // TODO: Implement API call to unblock withdrawals
    // unblockRetailerWithdrawals(retailerId)
  }

  const handleSuspendTerminal = (terminalId: string, currentStatus: string) => {
    const action = currentStatus === 'offline' ? 'reactivate' : 'suspend'

    toast({
      title: `${action === 'suspend' ? 'Suspending' : 'Reactivating'} terminal...`,
      description: `Terminal ${terminalId} will be ${action === 'suspend' ? 'suspended' : 'reactivated'}.`,
    })

    // TODO: Implement API call to suspend/reactivate terminal
    // updateTerminalStatus(terminalId, action)
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

  if (isLoading) {
    return <div className="p-3 sm:p-4 md:p-6 text-sm sm:text-base">Loading retailer details...</div>
  }

  if (!retailer) {
    return (
      <div className="p-3 sm:p-4 md:p-6">
        <div className="text-center">
          <AlertCircle className="mx-auto h-10 w-10 sm:h-12 sm:w-12 text-destructive" />
          <p className="mt-2 text-base sm:text-lg font-semibold">Retailer not found</p>
          <Button
            variant="outline"
            size="sm"
            className="mt-4"
            onClick={() => navigate({ to: '/admin/retailers' })}
          >
            Back to Retailers
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="p-3 sm:p-4 md:p-6 space-y-3 sm:space-y-4 md:space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center gap-3">
        <Button
          variant="outline"
          size="sm"
          onClick={() => navigate({ to: '/admin/retailers' })}
          className="self-start sm:self-auto"
        >
          <ArrowLeft className="h-4 w-4 mr-2" />
          <span className="hidden sm:inline">Back to Retailers</span>
          <span className="sm:hidden">Back</span>
        </Button>
        <div className="flex-1 min-w-0">
          <h1 className="text-xl sm:text-2xl md:text-3xl font-bold truncate">{retailer.name}</h1>
          <p className="text-xs sm:text-sm text-muted-foreground truncate">
            Retailer Code: {retailer.retailer_code}
          </p>
        </div>
        <Badge variant={getStatusBadgeVariant(retailer.status)} className="self-start sm:self-auto">
          {getStatusLabel(retailer.status)}
        </Badge>
        <div className="flex flex-col sm:flex-row gap-2 w-full sm:w-auto">
          {!isEditing ? (
            <>
              <Button onClick={handleEdit} size="sm" className="w-full sm:w-auto">
                <Edit className="mr-2 h-4 w-4" />
                <span className="hidden sm:inline">Edit Profile</span>
                <span className="sm:hidden">Edit</span>
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" size="sm" className="w-full sm:w-auto">
                    <ShieldOff className="mr-2 h-4 w-4" />
                    <span className="hidden sm:inline">Admin Actions</span>
                    <span className="sm:hidden">Actions</span>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-56">
                  <DropdownMenuLabel>Retailer Controls</DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    onClick={() => handleSuspendRetailer()}
                    className="text-destructive focus:text-destructive"
                  >
                    <Ban className="mr-2 h-4 w-4" />
                    {retailer.status === 'SUSPENDED' ? 'Reactivate Retailer' : 'Suspend Retailer'}
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuLabel>Wallet Controls</DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={() => handleBlockWithdrawals()}>
                    <Lock className="mr-2 h-4 w-4" />
                    Block Winning Withdrawals
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => handleUnblockWithdrawals()}>
                    <Unlock className="mr-2 h-4 w-4" />
                    Unblock Winning Withdrawals
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          ) : (
            <>
              <Button
                onClick={handleSave}
                disabled={updateMutation.isPending}
                size="sm"
                className="w-full sm:w-auto"
              >
                <Save className="mr-2 h-4 w-4" />
                {updateMutation.isPending ? 'Saving...' : 'Save'}
              </Button>
              <Button
                variant="outline"
                onClick={handleCancel}
                size="sm"
                className="w-full sm:w-auto"
              >
                <X className="mr-2 h-4 w-4" />
                Cancel
              </Button>
            </>
          )}
        </div>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="overview" className="space-y-3 sm:space-y-4 md:space-y-6">
        <TabsList className="grid grid-cols-3 sm:grid-cols-5 w-full gap-1 text-xs sm:text-sm">
          <TabsTrigger value="overview" className="px-2 sm:px-3">
            Overview
          </TabsTrigger>
          <TabsTrigger value="wallets" className="px-2 sm:px-3">
            Wallets
          </TabsTrigger>
          <TabsTrigger value="pos-terminals" className="px-2 sm:px-3">
            <span className="hidden sm:inline">POS Terminals</span>
            <span className="sm:hidden">POS</span>
          </TabsTrigger>
          <TabsTrigger value="transactions" className="px-2 sm:px-3">
            <span className="hidden sm:inline">Transactions</span>
            <span className="sm:hidden">Txns</span>
          </TabsTrigger>
          <TabsTrigger value="activity" className="px-2 sm:px-3">
            Activity
          </TabsTrigger>
        </TabsList>

        {/* Overview Tab */}
        <TabsContent value="overview" className="space-y-3 sm:space-y-4 md:space-y-6">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 sm:gap-4 md:gap-6">
            {/* Basic Information */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Store className="h-5 w-5" />
                  Business Information
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <Label htmlFor="name">Retailer Name</Label>
                  {isEditing ? (
                    <Input
                      id="name"
                      value={editForm.name}
                      onChange={e => setEditForm(prev => ({ ...prev, name: e.target.value }))}
                    />
                  ) : (
                    <p className="text-sm">{retailer.name}</p>
                  )}
                </div>
                <div>
                  <Label htmlFor="retailer-code">Retailer Code</Label>
                  <p className="text-sm font-mono">{retailer.retailer_code}</p>
                </div>
                <div>
                  <Label htmlFor="address">Address</Label>
                  {isEditing ? (
                    <Textarea
                      id="address"
                      value={editForm.address}
                      onChange={e => setEditForm(prev => ({ ...prev, address: e.target.value }))}
                    />
                  ) : (
                    <p className="text-sm">{retailer.address}</p>
                  )}
                </div>
              </CardContent>
            </Card>

            {/* Contact Information */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <User className="h-5 w-5" />
                  Contact Information
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <Label htmlFor="email" className="flex items-center gap-2">
                    <Mail className="h-4 w-4" />
                    Email
                  </Label>
                  {isEditing ? (
                    <Input
                      id="email"
                      type="email"
                      value={editForm.email}
                      onChange={e => setEditForm(prev => ({ ...prev, email: e.target.value }))}
                    />
                  ) : (
                    <p className="text-sm">{retailer.email}</p>
                  )}
                </div>
                <div>
                  <Label htmlFor="phone" className="flex items-center gap-2">
                    <Phone className="h-4 w-4" />
                    Phone Number
                  </Label>
                  {isEditing ? (
                    <Input
                      id="phone"
                      value={editForm.phone_number}
                      onChange={e =>
                        setEditForm(prev => ({ ...prev, phone_number: e.target.value }))
                      }
                    />
                  ) : (
                    <p className="text-sm">{retailer.phone_number}</p>
                  )}
                </div>
              </CardContent>
            </Card>

            {/* Agent Information */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Building className="h-5 w-5" />
                  Managing Agent
                </CardTitle>
              </CardHeader>
              <CardContent>
                {retailer.agent_id ? (
                  <div className="space-y-2">
                    <p className="text-sm font-medium">Agent ID: {retailer.agent_id}</p>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => navigate({ to: `/admin/agents/${retailer.agent_id}` })}
                    >
                      View Agent Profile
                    </Button>
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    Independent Retailer (No managing agent)
                  </p>
                )}
              </CardContent>
            </Card>

            {/* Registration Details */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Calendar className="h-5 w-5" />
                  Registration Details
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <div>
                  <Label>Created At</Label>
                  <p className="text-sm">{new Date(retailer.created_at).toLocaleString()}</p>
                </div>
                <div>
                  <Label>Last Updated</Label>
                  <p className="text-sm">{new Date(retailer.updated_at).toLocaleString()}</p>
                </div>
                <div>
                  <Label>Created By</Label>
                  <p className="text-sm">{retailer.created_by}</p>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        {/* Wallets Tab */}
        <TabsContent value="wallets" className="space-y-3 sm:space-y-4 md:space-y-6">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 sm:gap-4 md:gap-6">
            {/* Stake Wallet */}
            <RetailerWalletCard
              retailerId={retailer.id}
              retailerName={retailer.name}
              walletType="stake"
              balance={stakeBalance?.balance || 0}
              pendingBalance={stakeBalance?.pending_balance}
              onViewTransactions={() => {
                setSelectedWalletType('stake')
                // Navigate to transactions tab
                const tabsList = document.querySelector('[role="tablist"]')
                const transactionsTab = tabsList?.querySelector(
                  '[value="transactions"]'
                ) as HTMLButtonElement
                transactionsTab?.click()
              }}
            />

            {/* Winning Wallet */}
            <RetailerWalletCard
              retailerId={retailer.id}
              retailerName={retailer.name}
              walletType="winning"
              balance={winningBalance?.balance || 0}
              pendingBalance={winningBalance?.pending_balance}
              onViewTransactions={() => {
                setSelectedWalletType('winning')
                // Navigate to transactions tab
                const tabsList = document.querySelector('[role="tablist"]')
                const transactionsTab = tabsList?.querySelector(
                  '[value="transactions"]'
                ) as HTMLButtonElement
                transactionsTab?.click()
              }}
            />
          </div>

          {/* Transaction History Table */}
          <Card className="mt-6">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <FileText className="h-5 w-5" />
                Recent Transactions
              </CardTitle>
              <p className="text-sm text-muted-foreground">
                Transaction history for {selectedWalletType} wallet
              </p>
            </CardHeader>
            <CardContent>
              {transactionHistory && transactionHistory.transactions.length > 0 ? (
                <TransactionHistoryTable
                  // eslint-disable-next-line @typescript-eslint/no-explicit-any
                  transactions={transactionHistory.transactions as any}
                  totalCount={
                    transactionHistory.total_count || transactionHistory.transactions.length
                  }
                  currentPage={1}
                  pageSize={10}
                  walletType="retailer"
                  // eslint-disable-next-line @typescript-eslint/no-unused-vars
                  onPageChange={(_page: number) => {}}
                  // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-explicit-any
                  onFilterChange={(_filters: any) => {}}
                />
              ) : (
                <p className="text-muted-foreground text-center py-8">
                  No transactions available for {selectedWalletType} wallet
                </p>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* POS Terminals Tab */}
        <TabsContent value="pos-terminals" className="space-y-3 sm:space-y-4 md:space-y-6">
          <Card>
            <CardHeader>
              <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <CardTitle className="flex items-center gap-2 text-base sm:text-lg">
                    <Monitor className="h-4 w-4 sm:h-5 sm:w-5 shrink-0" />
                    POS Terminals
                  </CardTitle>
                  <p className="text-xs sm:text-sm text-muted-foreground mt-1">
                    Manage POS terminals assigned to this retailer
                  </p>
                </div>
                <Button size="sm" className="w-full sm:w-auto shrink-0">
                  <Plus className="mr-2 h-4 w-4" />
                  <span className="hidden sm:inline">Assign Terminal</span>
                  <span className="sm:hidden">Assign</span>
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              <div className="space-y-3 sm:space-y-4">
                {/* Sample POS Terminal Cards */}
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 sm:gap-4">
                  <Card>
                    <CardHeader className="pb-2 sm:pb-3">
                      <div className="flex items-center justify-between gap-2">
                        <div className="flex items-center gap-2 min-w-0">
                          <Monitor className="h-4 w-4 text-muted-foreground shrink-0" />
                          <span className="text-sm sm:text-base font-semibold truncate">
                            POS-2025-0001
                          </span>
                        </div>
                        <Badge
                          variant="default"
                          className="flex items-center gap-1 shrink-0 text-xs"
                        >
                          <Activity className="h-3 w-3" />
                          Active
                        </Badge>
                      </div>
                    </CardHeader>
                    <CardContent className="space-y-2">
                      <div className="text-xs sm:text-sm">
                        <p className="text-muted-foreground">Serial Number</p>
                        <p className="font-medium">SN-123456789</p>
                      </div>
                      <div className="text-xs sm:text-sm">
                        <p className="text-muted-foreground">Last Activity</p>
                        <p className="font-medium">2 hours ago</p>
                      </div>
                      <div className="flex gap-2 pt-2">
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              variant="outline"
                              size="sm"
                              className="w-full text-xs sm:text-sm"
                            >
                              <span className="hidden sm:inline">Manage Terminal</span>
                              <span className="sm:hidden">Manage</span>
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem>View Details</DropdownMenuItem>
                            <DropdownMenuItem>View Transactions</DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              onClick={() => handleSuspendTerminal('POS-2025-0001', 'active')}
                              className="text-destructive focus:text-destructive"
                            >
                              <Ban className="mr-2 h-4 w-4" />
                              Suspend Terminal
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </CardContent>
                  </Card>

                  <Card>
                    <CardHeader className="pb-2 sm:pb-3">
                      <div className="flex items-center justify-between gap-2">
                        <div className="flex items-center gap-2 min-w-0">
                          <Monitor className="h-4 w-4 text-muted-foreground shrink-0" />
                          <span className="text-sm sm:text-base font-semibold truncate">
                            POS-2025-0002
                          </span>
                        </div>
                        <Badge
                          variant="secondary"
                          className="flex items-center gap-1 shrink-0 text-xs"
                        >
                          <Power className="h-3 w-3" />
                          Offline
                        </Badge>
                      </div>
                    </CardHeader>
                    <CardContent className="space-y-2">
                      <div className="text-xs sm:text-sm">
                        <p className="text-muted-foreground">Serial Number</p>
                        <p className="font-medium">SN-987654321</p>
                      </div>
                      <div className="text-xs sm:text-sm">
                        <p className="text-muted-foreground">Last Activity</p>
                        <p className="font-medium">3 days ago</p>
                      </div>
                      <div className="flex gap-2 pt-2">
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              variant="outline"
                              size="sm"
                              className="w-full text-xs sm:text-sm"
                            >
                              <span className="hidden sm:inline">Manage Terminal</span>
                              <span className="sm:hidden">Manage</span>
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem>View Details</DropdownMenuItem>
                            <DropdownMenuItem>View Transactions</DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              onClick={() => handleSuspendTerminal('POS-2025-0002', 'offline')}
                              className="text-success focus:text-success"
                            >
                              <Power className="mr-2 h-4 w-4" />
                              Reactivate Terminal
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </CardContent>
                  </Card>
                </div>

                {/* Empty State (uncomment when implementing actual data fetching) */}
                {/* <div className="text-center py-8">
                  <Monitor className="mx-auto h-12 w-12 text-muted-foreground" />
                  <p className="mt-2 text-sm text-muted-foreground">No POS terminals assigned</p>
                  <Button className="mt-4">
                    <Plus className="mr-2 h-4 w-4" />
                    Assign First Terminal
                  </Button>
                </div> */}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Transactions Tab */}
        <TabsContent value="transactions" className="space-y-3 sm:space-y-4 md:space-y-6">
          <Card>
            <CardHeader>
              <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
                <CardTitle className="flex items-center gap-2 text-base sm:text-lg">
                  <CreditCard className="h-4 w-4 sm:h-5 sm:w-5 shrink-0" />
                  Transaction History
                </CardTitle>
                <Tabs
                  value={selectedWalletType}
                  onValueChange={v => setSelectedWalletType(v as 'stake' | 'winning')}
                >
                  <TabsList className="grid grid-cols-2 w-full sm:w-auto text-xs sm:text-sm">
                    <TabsTrigger value="stake" className="px-2 sm:px-3">
                      <span className="hidden sm:inline">Stake Wallet</span>
                      <span className="sm:hidden">Stake</span>
                    </TabsTrigger>
                    <TabsTrigger value="winning" className="px-2 sm:px-3">
                      <span className="hidden sm:inline">Winning Wallet</span>
                      <span className="sm:hidden">Winning</span>
                    </TabsTrigger>
                  </TabsList>
                </Tabs>
              </div>
            </CardHeader>
            <CardContent>
              <TransactionHistoryTable
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                transactions={(transactionHistory?.transactions || []) as any}
                totalCount={transactionHistory?.total_count || 0}
                currentPage={1}
                pageSize={10}
                isLoading={false}
                walletType="retailer"
                // eslint-disable-next-line @typescript-eslint/no-unused-vars
                onPageChange={(_page: number) => {}}
                // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-explicit-any
                onFilterChange={(_filters: any) => {}}
              />
            </CardContent>
          </Card>
        </TabsContent>

        {/* Activity Tab */}
        <TabsContent value="activity" className="space-y-3 sm:space-y-4 md:space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base sm:text-lg">
                <FileText className="h-4 w-4 sm:h-5 sm:w-5 shrink-0" />
                Recent Activity
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-xs sm:text-sm text-muted-foreground">
                Activity tracking will be implemented soon
              </p>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Credit Wallet Dialog */}
      <CreditWalletDialog
        open={creditDialogOpen}
        onOpenChange={setCreditDialogOpen}
        walletType="retailer"
        ownerName={retailer.name}
        ownerId={retailer.id}
        currentBalance={0}
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        onConfirm={handleCreditWallet as any}
      />
    </div>
  )
}
