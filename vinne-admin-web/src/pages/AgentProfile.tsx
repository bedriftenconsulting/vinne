import { useState } from 'react'
import { useParams } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Separator } from '@/components/ui/separator'
import { agentService, type Retailer } from '@/services/agents'
import { walletService } from '@/services/wallet'
import { AgentWalletCard } from '@/components/wallet/AgentWalletCard'
import { CreditWalletDialog } from '@/components/wallet/CreditWalletDialog'
import { TransactionHistoryTable } from '@/components/wallet/TransactionHistoryTable'
import { EditAgentDialog } from '@/components/agents/EditAgentDialog'
import { useToast } from '@/hooks/use-toast'
import {
  ArrowLeft,
  Edit,
  Users,
  Wallet,
  BarChart3,
  Settings,
  Shield,
  Calculator,
  MapPin,
  Phone,
  Mail,
  Calendar,
  Building2,
  CreditCard,
  Activity,
} from 'lucide-react'

export default function AgentProfile() {
  const { agentId } = useParams({ from: '/admin/agent/$agentId' })
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const [creditDialogOpen, setCreditDialogOpen] = useState(false)
  const [editDialogOpen, setEditDialogOpen] = useState(false)

  const { data: agent, isLoading: agentLoading } = useQuery({
    queryKey: ['agent', agentId],
    queryFn: () => agentService.getAgent(agentId),
  })

  const { data: retailersData } = useQuery({
    queryKey: ['agent-retailers', agentId],
    queryFn: () => agentService.getRetailers(1, 100, { agent_id: agentId }),
    enabled: !!agentId,
  })

  // Wallet queries
  const { data: walletBalance } = useQuery({
    queryKey: ['agent-wallet-balance', agentId],
    queryFn: () => walletService.getAgentWalletBalance(agentId),
    enabled: !!agentId,
    refetchInterval: 30000, // Refresh every 30 seconds
  })

  const { data: transactionHistory } = useQuery({
    queryKey: ['agent-transactions', agentId],
    queryFn: () => walletService.getTransactionHistory(agentId, 'agent'),
    enabled: !!agentId,
  })

  // Credit wallet mutation
  const creditWalletMutation = useMutation({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    mutationFn: (data: any) => walletService.creditAgentWallet(agentId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agent-wallet-balance', agentId] })
      queryClient.invalidateQueries({ queryKey: ['agent-transactions', agentId] })
      setCreditDialogOpen(false)
      toast({
        title: 'Success',
        description: 'Agent wallet credited successfully',
      })
    },
    onError: (error: Error) => {
      toast({
        title: 'Error',
        description: error.message || 'Failed to credit wallet',
        variant: 'destructive',
      })
    },
  })

  // Get commission rate
  const { data: commissionRate } = useQuery({
    queryKey: ['agent-commission-rate', agentId],
    queryFn: () => walletService.getCommissionRate(agentId),
    enabled: !!agentId,
  })

  // Helper functions
  const lastTransaction = transactionHistory?.transactions?.[0]
    ? {
        amount: transactionHistory.transactions[0].amount,
        type: (transactionHistory.transactions[0].type?.toLowerCase() || 'transfer') as
          | 'credit'
          | 'debit'
          | 'transfer',
        date: transactionHistory.transactions[0].created_at,
      }
    : undefined

  const handleExportTransactions = () => {
    // TODO: Implement CSV export
    toast({
      title: 'Export started',
      description: 'Transaction history will be downloaded shortly',
    })
  }

  const handleCreditWallet = (amount: number, paymentMethod: string, notes?: string) => {
    creditWalletMutation.mutate({
      amount,
      payment_method: paymentMethod,
      notes,
    })
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

  if (agentLoading) {
    return <div className="p-6">Loading agent profile...</div>
  }

  if (!agent) {
    return <div className="p-6">Agent not found</div>
  }

  return (
    <>
      <div className="p-3 sm:p-4 md:p-6 space-y-3 sm:space-y-4 md:space-y-6">
        <div className="flex flex-col sm:flex-row items-start sm:items-center gap-3">
          <Button
            variant="outline"
            size="sm"
            onClick={() => window.history.back()}
            className="self-start sm:self-auto"
          >
            <ArrowLeft className="h-4 w-4 mr-2" />
            <span className="hidden sm:inline">Back to Agents</span>
            <span className="sm:hidden">Back</span>
          </Button>
          <div className="flex-1 min-w-0">
            <h1 className="text-xl sm:text-2xl md:text-3xl font-bold truncate">{agent.name}</h1>
            <p className="text-sm sm:text-base text-muted-foreground truncate">
              {agent.agent_code}
            </p>
          </div>
          <div className="flex items-center gap-2 sm:gap-3 self-start sm:self-auto">
            <Badge variant={getStatusBadgeVariant(agent.status)}>
              {getStatusLabel(agent.status)}
            </Badge>
            <Button onClick={() => setEditDialogOpen(true)} size="sm">
              <Edit className="h-4 w-4 mr-2" />
              <span className="hidden sm:inline">Edit Profile</span>
              <span className="sm:hidden">Edit</span>
            </Button>
          </div>
        </div>

        <Tabs defaultValue="overview" className="space-y-3 sm:space-y-4 md:space-y-6">
          <TabsList className="grid grid-cols-3 sm:grid-cols-6 w-full gap-1 text-xs sm:text-sm">
            <TabsTrigger value="overview" className="px-2 sm:px-3">
              Overview
            </TabsTrigger>
            <TabsTrigger value="retailers" className="px-2 sm:px-3">
              Retailers
            </TabsTrigger>
            <TabsTrigger value="wallets" className="px-2 sm:px-3">
              Wallets
            </TabsTrigger>
            <TabsTrigger value="performance" className="px-2 sm:px-3">
              <span className="hidden sm:inline">Performance</span>
              <span className="sm:hidden">Stats</span>
            </TabsTrigger>
            <TabsTrigger value="commission" className="px-2 sm:px-3">
              <span className="hidden sm:inline">Commission</span>
              <span className="sm:hidden">Comm</span>
            </TabsTrigger>
            <TabsTrigger value="security" className="px-2 sm:px-3">
              Security
            </TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="space-y-3 sm:space-y-4 md:space-y-6">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 sm:gap-4 md:gap-6">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Building2 className="h-5 w-5" />
                    Basic Information
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Agent Code</p>
                      <p className="font-medium">{agent.agent_code}</p>
                    </div>
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Status</p>
                      <Badge variant={getStatusBadgeVariant(agent.status)}>
                        {getStatusLabel(agent.status)}
                      </Badge>
                    </div>
                  </div>
                  <Separator />
                  <div className="space-y-3">
                    <div className="flex items-center gap-2">
                      <Mail className="h-4 w-4 text-muted-foreground" />
                      <span>{agent.email}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <Phone className="h-4 w-4 text-muted-foreground" />
                      <span>{agent.phone_number}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <MapPin className="h-4 w-4 text-muted-foreground" />
                      <span>{agent.address}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <Calendar className="h-4 w-4 text-muted-foreground" />
                      <span>Registered: {new Date(agent.created_at).toLocaleDateString()}</span>
                    </div>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Activity className="h-5 w-5" />
                    Quick Stats
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Retailers</p>
                      <p className="text-2xl font-bold">
                        {retailersData?.pagination?.total_count || 0}
                      </p>
                    </div>
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Active Devices</p>
                      <p className="text-2xl font-bold">0</p>
                    </div>
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Commission Rate</p>
                      <p className="font-medium">{agent.commission_percentage || 30}%</p>
                    </div>
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Last Active</p>
                      <p className="font-medium">Never</p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </div>

            <Card>
              <CardHeader>
                <CardTitle>Recent Activity</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground">No recent activity data available</p>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="retailers" className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Users className="h-5 w-5" />
                  Managed Retailers ({retailersData?.pagination?.total_count || 0})
                </CardTitle>
              </CardHeader>
              <CardContent>
                {retailersData?.data && retailersData.data.length > 0 ? (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Retailer Code</TableHead>
                        <TableHead>Retailer Name</TableHead>
                        <TableHead>Contact</TableHead>
                        <TableHead>Status</TableHead>
                        <TableHead>Joined</TableHead>
                        <TableHead>Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {retailersData.data.map((retailer: Retailer) => (
                        <TableRow key={retailer.id}>
                          <TableCell className="font-medium">{retailer.retailer_code}</TableCell>
                          <TableCell>{retailer.name}</TableCell>
                          <TableCell>
                            <div className="text-sm">
                              <p>{retailer.email}</p>
                              <p className="text-muted-foreground">{retailer.phone_number}</p>
                            </div>
                          </TableCell>
                          <TableCell>
                            <Badge variant={getStatusBadgeVariant(retailer.status)}>
                              {getStatusLabel(retailer.status)}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            {new Date(retailer.created_at).toLocaleDateString()}
                          </TableCell>
                          <TableCell>
                            <Button variant="outline" size="sm">
                              View Profile
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                ) : (
                  <p className="text-muted-foreground text-center py-8">
                    No retailers assigned to this agent
                  </p>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="wallets" className="space-y-6">
            <div className="mb-4">
              <p className="text-sm text-muted-foreground">
                Agent stake wallet for collecting ticket sales. Note: Agents cannot withdraw funds,
                only transfer to retailers.
              </p>
            </div>

            <AgentWalletCard
              agentId={agentId}
              agentName={agent.name}
              balance={walletBalance?.balance || 0}
              pendingBalance={walletBalance?.pending_balance || 0}
              lastTransaction={lastTransaction}
              onViewTransactions={() => {}}
            />

            <TransactionHistoryTable
              // eslint-disable-next-line @typescript-eslint/no-explicit-any
              transactions={(transactionHistory?.transactions || []) as any}
              totalCount={transactionHistory?.total_count || 0}
              currentPage={1}
              pageSize={10}
              isLoading={!transactionHistory}
              walletType="agent"
              onPageChange={(page: number) => {
                // TODO: Implement pagination
                console.log('Page changed to:', page)
              }}
              // eslint-disable-next-line @typescript-eslint/no-explicit-any
              onFilterChange={(filters: any) => {
                // TODO: Implement filtering
                console.log('Filters changed:', filters)
              }}
              onExport={handleExportTransactions}
              onRefresh={() =>
                queryClient.invalidateQueries({ queryKey: ['agent-transactions', agentId] })
              }
            />
          </TabsContent>

          <TabsContent value="performance" className="space-y-6">
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
              <Card>
                <CardContent className="p-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Tickets Sold</p>
                      <p className="text-2xl font-bold">0</p>
                    </div>
                    <BarChart3 className="h-8 w-8 text-muted-foreground" />
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="p-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Total Stakes</p>
                      <p className="text-2xl font-bold">GH₵0.00</p>
                    </div>
                    <Wallet className="h-8 w-8 text-muted-foreground" />
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="p-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Total Transfers</p>
                      <p className="text-2xl font-bold">GH₵0.00</p>
                    </div>
                    <CreditCard className="h-8 w-8 text-muted-foreground" />
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="p-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Net Revenue</p>
                      <p className="text-2xl font-bold">GH₵0.00</p>
                    </div>
                    <BarChart3 className="h-8 w-8 text-muted-foreground" />
                  </div>
                </CardContent>
              </Card>
            </div>

            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <BarChart3 className="h-5 w-5" />
                  Performance Analytics
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground text-center py-8">
                  Performance analytics dashboard coming soon
                </p>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="commission" className="space-y-6">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <Card>
                <CardContent className="p-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">
                        Today's Commission
                      </p>
                      <p className="text-2xl font-bold">GH₵0.00</p>
                    </div>
                    <Calculator className="h-8 w-8 text-muted-foreground" />
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="p-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">This Month</p>
                      <p className="text-2xl font-bold">GH₵0.00</p>
                    </div>
                    <Calculator className="h-8 w-8 text-muted-foreground" />
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="p-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Total Earned</p>
                      <p className="text-2xl font-bold">GH₵0.00</p>
                    </div>
                    <Calculator className="h-8 w-8 text-muted-foreground" />
                  </div>
                </CardContent>
              </Card>
            </div>

            <Card>
              <CardHeader>
                <CardTitle>Commission Settings</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">Commission Rate</p>
                    <p className="font-medium">{agent.commission_percentage || 30}%</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">Commission Model</p>
                    <p className="font-medium">Direct Percentage</p>
                  </div>
                </div>
                <div className="text-sm text-muted-foreground">
                  <p>Commission is calculated as a direct percentage of ticket sales revenue.</p>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Recent Commission Transactions</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground text-center py-8">
                  No commission transactions available
                </p>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="security" className="space-y-6">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Shield className="h-5 w-5" />
                    Security Information
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="space-y-3">
                    <div className="flex justify-between">
                      <span className="text-sm">Last Login</span>
                      <span className="text-sm font-medium">Never</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-sm">Two-Factor Auth</span>
                      <Badge variant="outline">Not Configured</Badge>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-sm">Account Created</span>
                      <span className="text-sm font-medium">
                        {new Date(agent.created_at).toLocaleString()}
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-sm">Last Updated</span>
                      <span className="text-sm font-medium">
                        {agent.updated_at ? new Date(agent.updated_at).toLocaleString() : 'Never'}
                      </span>
                    </div>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Account Actions</CardTitle>
                </CardHeader>
                <CardContent className="space-y-3">
                  <Button variant="outline" className="w-full justify-start">
                    <Settings className="h-4 w-4 mr-2" />
                    Reset Password
                  </Button>
                  <Button variant="outline" className="w-full justify-start">
                    <Shield className="h-4 w-4 mr-2" />
                    Configure 2FA
                  </Button>
                  <Button variant="outline" className="w-full justify-start">
                    <Activity className="h-4 w-4 mr-2" />
                    View Activity Log
                  </Button>
                  <Separator />
                  <Button variant="destructive" className="w-full justify-start">
                    Suspend Account
                  </Button>
                </CardContent>
              </Card>
            </div>
          </TabsContent>
        </Tabs>

        {/* Credit Wallet Dialog */}
        <CreditWalletDialog
          open={creditDialogOpen}
          onOpenChange={setCreditDialogOpen}
          walletType="agent"
          ownerName={agent.name}
          ownerId={agent.id}
          currentBalance={walletBalance?.balance || 0}
          commissionRate={commissionRate?.rate || 0.3}
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          onConfirm={handleCreditWallet as any}
        />

        {agent && (
          <EditAgentDialog agent={agent} open={editDialogOpen} onOpenChange={setEditDialogOpen} />
        )}
      </div>
    </>
  )
}
