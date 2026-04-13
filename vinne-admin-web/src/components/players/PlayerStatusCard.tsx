import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { playerService, type Player } from '@/services/players'
import { useToast } from '@/hooks/use-toast'
import { ShieldAlert, ShieldCheck, ShieldX } from 'lucide-react'

interface PlayerStatusCardProps {
  player: Player
}

export function PlayerStatusCard({ player }: PlayerStatusCardProps) {
  const [showSuspendDialog, setShowSuspendDialog] = useState(false)
  const [showActivateDialog, setShowActivateDialog] = useState(false)
  const [suspendReason, setSuspendReason] = useState('')
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const suspendMutation = useMutation({
    mutationFn: (reason: string) =>
      playerService.suspendPlayer(player.id, {
        reason,
        suspended_by: 'admin', // TODO: Replace with actual admin ID from auth
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['player', player.id] })
      queryClient.invalidateQueries({ queryKey: ['players'] })
      toast({
        title: 'Player Suspended',
        description: 'The player has been suspended successfully.',
      })
      setShowSuspendDialog(false)
      setSuspendReason('')
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to suspend player. Please try again.',
        variant: 'destructive',
      })
    },
  })

  const activateMutation = useMutation({
    mutationFn: () =>
      playerService.activatePlayer(player.id, {
        activated_by: 'admin', // TODO: Replace with actual admin ID from auth
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['player', player.id] })
      queryClient.invalidateQueries({ queryKey: ['players'] })
      toast({
        title: 'Player Activated',
        description: 'The player has been activated successfully.',
      })
      setShowActivateDialog(false)
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to activate player. Please try again.',
        variant: 'destructive',
      })
    },
  })

  const handleSuspend = () => {
    if (!suspendReason.trim()) {
      toast({
        title: 'Validation Error',
        description: 'Please provide a reason for suspension.',
        variant: 'destructive',
      })
      return
    }
    suspendMutation.mutate(suspendReason)
  }

  const handleActivate = () => {
    activateMutation.mutate()
  }

  const getStatusBadgeVariant = (status: string) => {
    switch (status) {
      case 'ACTIVE':
        return 'default'
      case 'SUSPENDED':
        return 'secondary'
      case 'BANNED':
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
      case 'BANNED':
        return 'Banned'
      default:
        return status
    }
  }

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'ACTIVE':
        return <ShieldCheck className="h-5 w-5 text-green-600" />
      case 'SUSPENDED':
        return <ShieldAlert className="h-5 w-5 text-yellow-600" />
      case 'BANNED':
        return <ShieldX className="h-5 w-5 text-red-600" />
      default:
        return <ShieldAlert className="h-5 w-5" />
    }
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            {getStatusIcon(player.status)}
            Account Status
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="space-y-1">
              <p className="text-sm text-muted-foreground">Current Status</p>
              <Badge variant={getStatusBadgeVariant(player.status)} className="text-sm">
                {getStatusLabel(player.status)}
              </Badge>
            </div>
          </div>

          <div className="pt-4 space-y-2">
            {player.status === 'ACTIVE' && (
              <Button
                variant="outline"
                className="w-full border-yellow-600 text-yellow-600 hover:bg-yellow-50"
                onClick={() => setShowSuspendDialog(true)}
              >
                <ShieldAlert className="h-4 w-4 mr-2" />
                Suspend Player
              </Button>
            )}

            {(player.status === 'SUSPENDED' || player.status === 'BANNED') && (
              <Button
                variant="outline"
                className="w-full border-green-600 text-green-600 hover:bg-green-50"
                onClick={() => setShowActivateDialog(true)}
              >
                <ShieldCheck className="h-4 w-4 mr-2" />
                Activate Player
              </Button>
            )}
          </div>

          {player.status === 'SUSPENDED' && (
            <div className="pt-4 border-t">
              <p className="text-sm text-muted-foreground">Status Information</p>
              <p className="text-sm text-yellow-600">
                This player's account is currently suspended and cannot perform any transactions.
              </p>
            </div>
          )}

          {player.status === 'BANNED' && (
            <div className="pt-4 border-t">
              <p className="text-sm text-muted-foreground">Status Information</p>
              <p className="text-sm text-red-600">
                This player's account has been permanently banned.
              </p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Suspend Dialog */}
      <Dialog open={showSuspendDialog} onOpenChange={setShowSuspendDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Suspend Player</DialogTitle>
            <DialogDescription>
              Please provide a reason for suspending this player's account.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="reason">Suspension Reason</Label>
              <Textarea
                id="reason"
                placeholder="Enter reason for suspension..."
                value={suspendReason}
                onChange={e => setSuspendReason(e.target.value)}
                rows={4}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setShowSuspendDialog(false)
                setSuspendReason('')
              }}
            >
              Cancel
            </Button>
            <Button variant="default" onClick={handleSuspend} disabled={suspendMutation.isPending}>
              {suspendMutation.isPending ? 'Suspending...' : 'Suspend Player'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Activate Dialog */}
      <Dialog open={showActivateDialog} onOpenChange={setShowActivateDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Activate Player</DialogTitle>
            <DialogDescription>
              Are you sure you want to activate this player's account? They will be able to perform
              transactions again.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowActivateDialog(false)}>
              Cancel
            </Button>
            <Button
              variant="default"
              onClick={handleActivate}
              disabled={activateMutation.isPending}
            >
              {activateMutation.isPending ? 'Activating...' : 'Activate Player'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
