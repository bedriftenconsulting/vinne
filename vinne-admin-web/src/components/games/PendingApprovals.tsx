import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
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
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { CheckCircle, XCircle, Info, Clock } from 'lucide-react'
import { gameService, type GameApproval } from '@/services/games'
import { formatInGhanaTime } from '@/lib/date-utils'
import { toast } from '@/hooks/use-toast'

export function PendingApprovals() {
  const [selectedApproval, setSelectedApproval] = useState<GameApproval | null>(null)
  const [approvalDialog, setApprovalDialog] = useState<{
    open: boolean
    type: 'approve' | 'reject' | null
    notes: string
  }>({ open: false, type: null, notes: '' })

  const queryClient = useQueryClient()

  const { data: approvalsData, isLoading } = useQuery({
    queryKey: ['pending-approvals'],
    queryFn: () => gameService.getPendingApprovals(),
    refetchInterval: 30000, // Refresh every 30 seconds
  })

  const approvals = approvalsData?.approvals || []

  const approveGameMutation = useMutation({
    mutationFn: ({ gameId, notes }: { gameId: string; notes?: string }) =>
      gameService.approveGame(gameId, notes),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['pending-approvals'] })
      queryClient.invalidateQueries({ queryKey: ['games'] })
      toast({
        title: 'Success',
        description: 'Game approved successfully',
      })
      setApprovalDialog({ open: false, type: null, notes: '' })
      setSelectedApproval(null)
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to approve game',
        variant: 'destructive',
      })
    },
  })

  const rejectGameMutation = useMutation({
    mutationFn: ({ gameId, reason }: { gameId: string; reason: string }) =>
      gameService.rejectGame(gameId, reason),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['pending-approvals'] })
      queryClient.invalidateQueries({ queryKey: ['games'] })
      toast({
        title: 'Success',
        description: 'Game rejected',
      })
      setApprovalDialog({ open: false, type: null, notes: '' })
      setSelectedApproval(null)
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to reject game',
        variant: 'destructive',
      })
    },
  })

  const handleApprove = () => {
    if (selectedApproval) {
      approveGameMutation.mutate({
        gameId: selectedApproval.game_id,
        notes: approvalDialog.notes,
      })
    }
  }

  const handleReject = () => {
    if (selectedApproval && approvalDialog.notes) {
      rejectGameMutation.mutate({
        gameId: selectedApproval.game_id,
        reason: approvalDialog.notes,
      })
    }
  }

  const getApprovalStageBadge = (stage: string) => {
    const stageMap: {
      [key: string]: {
        variant: 'default' | 'secondary' | 'outline' | 'destructive'
        label: string
        icon: React.ComponentType<{ className?: string }>
      }
    } = {
      SUBMITTED: { variant: 'outline', label: 'Awaiting First Approval', icon: Clock },
      FIRST_APPROVED: { variant: 'outline', label: 'Awaiting Second Approval', icon: Clock },
      APPROVED: { variant: 'default', label: 'Fully Approved', icon: CheckCircle },
      REJECTED: { variant: 'destructive', label: 'Rejected', icon: XCircle },
    }

    const config = stageMap[stage] || { variant: 'secondary', label: stage, icon: Info }
    const Icon = config.icon

    return (
      <Badge variant={config.variant} className="flex items-center gap-1">
        <Icon className="h-3 w-3" />
        {config.label}
      </Badge>
    )
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>Pending Game Approvals</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <div className="text-muted-foreground">Loading approvals...</div>
            </div>
          ) : approvals.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-8">
              <CheckCircle className="h-12 w-12 text-muted-foreground mb-4" />
              <div className="text-muted-foreground">No games pending approval</div>
            </div>
          ) : (
            <div className="rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Game Name</TableHead>
                    <TableHead>Game Code</TableHead>
                    <TableHead>Approval Stage</TableHead>
                    <TableHead>Submitted By</TableHead>
                    <TableHead>Submission Date</TableHead>
                    <TableHead>Notes</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {approvals.map(approval => (
                    <TableRow key={approval.id}>
                      <TableCell className="font-medium">
                        {approval.game?.name || 'Unknown Game'}
                      </TableCell>
                      <TableCell>{approval.game?.code || '-'}</TableCell>
                      <TableCell>{getApprovalStageBadge(approval.approval_stage)}</TableCell>
                      <TableCell>{approval.approver_name || approval.approved_by || '-'}</TableCell>
                      <TableCell>
                        {approval.created_at
                          ? formatInGhanaTime(new Date(approval.created_at), 'MMM dd, yyyy')
                          : '-'}
                      </TableCell>
                      <TableCell className="max-w-xs truncate">{approval.notes || '-'}</TableCell>
                      <TableCell>
                        <div className="flex items-center justify-end gap-2">
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => {
                              setSelectedApproval(approval)
                              setApprovalDialog({ open: true, type: 'approve', notes: '' })
                            }}
                            title="Approve"
                            className="text-green-600 hover:text-green-700"
                          >
                            <CheckCircle className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => {
                              setSelectedApproval(approval)
                              setApprovalDialog({ open: true, type: 'reject', notes: '' })
                            }}
                            title="Reject"
                            className="text-red-600 hover:text-red-700"
                          >
                            <XCircle className="h-4 w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Approval/Rejection Dialog */}
      <Dialog
        open={approvalDialog.open}
        onOpenChange={open => setApprovalDialog({ ...approvalDialog, open })}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {approvalDialog.type === 'approve' ? 'Approve Game' : 'Reject Game'}
            </DialogTitle>
            <DialogDescription>
              {approvalDialog.type === 'approve'
                ? `You are about to approve "${selectedApproval?.game?.name}". ${
                    selectedApproval?.approval_stage === 'SUBMITTED'
                      ? 'This will be the first approval.'
                      : 'This will be the final approval.'
                  }`
                : `You are about to reject "${selectedApproval?.game?.name}". Please provide a reason for rejection.`}
            </DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <Textarea
              placeholder={
                approvalDialog.type === 'approve'
                  ? 'Optional notes for approval...'
                  : 'Reason for rejection (required)...'
              }
              value={approvalDialog.notes}
              onChange={e => setApprovalDialog({ ...approvalDialog, notes: e.target.value })}
              className="min-h-[100px]"
            />
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setApprovalDialog({ open: false, type: null, notes: '' })}
            >
              Cancel
            </Button>
            <Button
              variant={approvalDialog.type === 'approve' ? 'default' : 'destructive'}
              onClick={approvalDialog.type === 'approve' ? handleApprove : handleReject}
              disabled={approvalDialog.type === 'reject' && !approvalDialog.notes.trim()}
            >
              {approvalDialog.type === 'approve' ? 'Approve' : 'Reject'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
