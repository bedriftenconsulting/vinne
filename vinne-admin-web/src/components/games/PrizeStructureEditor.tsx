/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState, useEffect } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useToast } from '@/hooks/use-toast'
import { Plus, Trash2, Save, Trophy, AlertCircle, Edit, Loader2 } from 'lucide-react'
import { gameService, type Game, type PrizeStructure, type PrizeTier } from '@/services/games'

interface PrizeStructureEditorProps {
  game: Game
  isOpen: boolean
  onClose: () => void
}

interface PrizeTierForm {
  prize_structure_id: string
  tier: number
  matches_required: number
  bonus_matches_required?: number
  prize_type: 'Fixed' | 'Percentage' | 'PariMutuel'
  fixed_amount?: number
  percentage?: number
  estimated_value?: number
}

export function PrizeStructureEditor({ game, isOpen, onClose }: PrizeStructureEditorProps) {
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const [isEditingStructure, setIsEditingStructure] = useState(false)
  const [isAddingTier, setIsAddingTier] = useState(false)
  const [editingTier, setEditingTier] = useState<PrizeTier | null>(null)

  // Prize structure form state
  const [structureForm, setStructureForm] = useState({
    prize_pool_percentage: 50,
    rollover_enabled: false,
    rollover_percentage: 10,
    jackpot_cap: 1000000,
    guaranteed_minimum: 10000,
  })

  // Prize tier form state
  const [tierForm, setTierForm] = useState<PrizeTierForm>({
    prize_structure_id: '',
    tier: 1,
    matches_required: 5,
    prize_type: 'Percentage',
    percentage: 40,
  })

  // Fetch prize structure
  const { data: prizeStructure } = useQuery({
    queryKey: ['prize-structure', game.id],
    queryFn: () => gameService.getPrizeStructure(game.id),
    enabled: isOpen,
  })

  // Use tiers from prizeStructure response
  const prizeTiers = prizeStructure?.tiers || []
  const isLoadingTiers = false

  // Update form when prizeStructure loads
  useEffect(() => {
    if (prizeStructure?.structure) {
      setStructureForm({
        prize_pool_percentage: prizeStructure.structure.prize_pool_percentage,
        rollover_enabled: prizeStructure.structure.rollover_enabled,
        rollover_percentage: prizeStructure.structure.rollover_percentage || 10,
        jackpot_cap: prizeStructure.structure.jackpot_cap || 1000000,
        guaranteed_minimum: prizeStructure.structure.guaranteed_minimum || 10000,
      })
    }
  }, [prizeStructure])

  // Mutations
  const updateStructureMutation = useMutation({
    mutationFn: (data: Partial<PrizeStructure>) =>
      gameService.updatePrizeStructure(prizeStructure!.structure.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prize-structure', game.id] })
      toast({ title: 'Success', description: 'Prize structure updated successfully' })
      setIsEditingStructure(false)
    },
    onError: (error: any) => {
      toast({
        title: 'Error',
        description: error.response?.data?.message || 'Failed to update prize structure',
        variant: 'destructive',
      })
    },
  })

  const createTierMutation = useMutation({
    mutationFn: (data: PrizeTierForm) =>
      gameService.createPrizeTier(prizeStructure!.structure.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prize-tiers', prizeStructure?.structure?.id] })
      toast({ title: 'Success', description: 'Prize tier added successfully' })
      setIsAddingTier(false)
      resetTierForm()
    },
    onError: (error: any) => {
      toast({
        title: 'Error',
        description: error.response?.data?.message || 'Failed to add prize tier',
        variant: 'destructive',
      })
    },
  })

  const updateTierMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<PrizeTier> }) =>
      gameService.updatePrizeTier(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prize-tiers', prizeStructure?.structure?.id] })
      toast({ title: 'Success', description: 'Prize tier updated successfully' })
      setEditingTier(null)
    },
    onError: (error: any) => {
      toast({
        title: 'Error',
        description: error.response?.data?.message || 'Failed to update prize tier',
        variant: 'destructive',
      })
    },
  })

  const deleteTierMutation = useMutation({
    mutationFn: (tierId: string) => gameService.deletePrizeTier(tierId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prize-tiers', prizeStructure?.structure?.id] })
      toast({ title: 'Success', description: 'Prize tier deleted successfully' })
    },
    onError: (error: any) => {
      toast({
        title: 'Error',
        description: error.response?.data?.message || 'Failed to delete prize tier',
        variant: 'destructive',
      })
    },
  })

  const resetTierForm = () => {
    setTierForm({
      prize_structure_id: prizeStructure?.structure?.id || '',
      tier: (prizeTiers?.length || 0) + 1,
      matches_required: 5,
      prize_type: 'Percentage',
      percentage: 40,
    })
  }

  const handleSaveStructure = () => {
    updateStructureMutation.mutate(structureForm)
  }

  const handleSaveTier = () => {
    if (editingTier) {
      updateTierMutation.mutate({ id: editingTier.id, data: tierForm })
    } else {
      createTierMutation.mutate(tierForm)
    }
  }

  const calculateTotalPercentage = () => {
    if (!prizeTiers) return 0
    return prizeTiers
      .filter(tier => tier.prize_type === 'Percentage')
      .reduce((sum, tier) => sum + (tier.percentage || 0), 0)
  }

  const calculateEstimatedPrize = (tier: PrizeTier, poolAmount: number = 1000000) => {
    const prizePoolAmount =
      (poolAmount * (prizeStructure?.structure.prize_pool_percentage || 50)) / 100

    switch (tier.prize_type) {
      case 'Fixed':
        return tier.fixed_amount || 0
      case 'Percentage':
        return (prizePoolAmount * (tier.percentage || 0)) / 100
      case 'PariMutuel':
        return tier.estimated_value || 0
      default:
        return 0
    }
  }

  if (!prizeStructure) {
    return (
      <Dialog open={isOpen} onOpenChange={onClose}>
        <DialogContent className="max-w-4xl">
          <DialogHeader>
            <DialogTitle>Prize Structure - {game.name}</DialogTitle>
            <DialogDescription>No prize structure configured yet</DialogDescription>
          </DialogHeader>
          <div className="flex items-center justify-center py-8">
            <Button
              onClick={() => {
                /* Create prize structure */
              }}
            >
              <Plus className="mr-2 h-4 w-4" />
              Create Prize Structure
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    )
  }

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-6xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Prize Structure - {game.name}</DialogTitle>
          <DialogDescription>
            Configure prize pools, tiers, and payout rules for this game
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-6">
          {/* Prize Pool Configuration */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Prize Pool Configuration</CardTitle>
                  <CardDescription>
                    Define how the prize pool is calculated and distributed
                  </CardDescription>
                </div>
                {!isEditingStructure ? (
                  <Button variant="outline" size="sm" onClick={() => setIsEditingStructure(true)}>
                    <Edit className="mr-2 h-4 w-4" />
                    Edit
                  </Button>
                ) : (
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setIsEditingStructure(false)}
                    >
                      Cancel
                    </Button>
                    <Button size="sm" onClick={handleSaveStructure}>
                      <Save className="mr-2 h-4 w-4" />
                      Save
                    </Button>
                  </div>
                )}
              </div>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label>Prize Pool Percentage</Label>
                  <div className="flex items-center gap-2">
                    <Input
                      type="number"
                      value={structureForm.prize_pool_percentage}
                      onChange={e =>
                        setStructureForm({
                          ...structureForm,
                          prize_pool_percentage: parseInt(e.target.value),
                        })
                      }
                      disabled={!isEditingStructure}
                      className="w-24"
                    />
                    <span className="text-muted-foreground">%</span>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Percentage of total sales allocated to prizes
                  </p>
                </div>

                <div className="space-y-2">
                  <Label>Guaranteed Minimum Jackpot</Label>
                  <div className="flex items-center gap-2">
                    <span className="text-muted-foreground">₵</span>
                    <Input
                      type="number"
                      value={structureForm.guaranteed_minimum}
                      onChange={e =>
                        setStructureForm({
                          ...structureForm,
                          guaranteed_minimum: parseInt(e.target.value),
                        })
                      }
                      disabled={!isEditingStructure}
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <Label>Enable Rollover</Label>
                    <Switch
                      checked={structureForm.rollover_enabled}
                      onCheckedChange={checked =>
                        setStructureForm({ ...structureForm, rollover_enabled: checked })
                      }
                      disabled={!isEditingStructure}
                    />
                  </div>
                  {structureForm.rollover_enabled && (
                    <div className="flex items-center gap-2">
                      <Input
                        type="number"
                        value={structureForm.rollover_percentage}
                        onChange={e =>
                          setStructureForm({
                            ...structureForm,
                            rollover_percentage: parseInt(e.target.value),
                          })
                        }
                        disabled={!isEditingStructure}
                        className="w-24"
                      />
                      <span className="text-muted-foreground">%</span>
                    </div>
                  )}
                </div>

                <div className="space-y-2">
                  <Label>Jackpot Cap</Label>
                  <div className="flex items-center gap-2">
                    <span className="text-muted-foreground">₵</span>
                    <Input
                      type="number"
                      value={structureForm.jackpot_cap}
                      onChange={e =>
                        setStructureForm({
                          ...structureForm,
                          jackpot_cap: parseInt(e.target.value),
                        })
                      }
                      disabled={!isEditingStructure}
                    />
                  </div>
                </div>
              </div>

              {/* Prize Pool Summary */}
              <div className="mt-6 rounded-lg bg-muted p-4">
                <div className="grid grid-cols-3 gap-4 text-sm">
                  <div>
                    <p className="text-muted-foreground">Total Prize Pool</p>
                    <p className="text-lg font-semibold">
                      {prizeStructure.structure.prize_pool_percentage}% of sales
                    </p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Allocated to Tiers</p>
                    <p className="text-lg font-semibold">{calculateTotalPercentage()}%</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Reserve/Rollover</p>
                    <p className="text-lg font-semibold">
                      {Math.max(0, 100 - calculateTotalPercentage())}%
                    </p>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Prize Tiers */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Prize Tiers</CardTitle>
                  <CardDescription>
                    Define prize amounts for different winning combinations
                  </CardDescription>
                </div>
                <Button size="sm" onClick={() => setIsAddingTier(true)}>
                  <Plus className="mr-2 h-4 w-4" />
                  Add Tier
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {isLoadingTiers ? (
                <div className="flex items-center justify-center py-8">
                  <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                </div>
              ) : !prizeTiers || prizeTiers.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-8">
                  <Trophy className="h-12 w-12 text-muted-foreground mb-4" />
                  <p className="text-muted-foreground">No prize tiers configured yet</p>
                  <Button variant="outline" className="mt-4" onClick={() => setIsAddingTier(true)}>
                    <Plus className="mr-2 h-4 w-4" />
                    Add First Tier
                  </Button>
                </div>
              ) : (
                <div className="rounded-md border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-20">Tier</TableHead>
                        <TableHead>Matches Required</TableHead>
                        <TableHead>Bonus Matches</TableHead>
                        <TableHead>Prize Type</TableHead>
                        <TableHead>Amount/Percentage</TableHead>
                        <TableHead>Estimated Prize</TableHead>
                        <TableHead className="text-right">Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {prizeTiers.map(tier => (
                        <TableRow key={tier.id}>
                          <TableCell>
                            <Badge variant="outline">Tier {tier.tier}</Badge>
                          </TableCell>
                          <TableCell>{tier.matches_required}</TableCell>
                          <TableCell>{tier.bonus_matches_required || '-'}</TableCell>
                          <TableCell>
                            <Badge
                              variant={
                                tier.prize_type === 'Fixed'
                                  ? 'default'
                                  : tier.prize_type === 'Percentage'
                                    ? 'secondary'
                                    : 'outline'
                              }
                            >
                              {tier.prize_type}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            {tier.prize_type === 'Fixed' && `₵${tier.fixed_amount}`}
                            {tier.prize_type === 'Percentage' && `${tier.percentage}%`}
                            {tier.prize_type === 'PariMutuel' && 'Variable'}
                          </TableCell>
                          <TableCell>₵{calculateEstimatedPrize(tier).toLocaleString()}</TableCell>
                          <TableCell>
                            <div className="flex items-center justify-end gap-2">
                              <Button
                                variant="ghost"
                                size="icon"
                                onClick={() => {
                                  setEditingTier(tier)
                                  setTierForm(tier as PrizeTierForm)
                                }}
                              >
                                <Edit className="h-4 w-4" />
                              </Button>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="text-destructive hover:text-destructive"
                                onClick={() => deleteTierMutation.mutate(tier.id)}
                              >
                                <Trash2 className="h-4 w-4" />
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

          {/* Warning for unallocated percentage */}
          {calculateTotalPercentage() > 100 && (
            <div className="flex items-center gap-2 rounded-lg border border-destructive bg-destructive/10 p-4">
              <AlertCircle className="h-5 w-5 text-destructive" />
              <p className="text-sm text-destructive">
                Total percentage allocated to tiers ({calculateTotalPercentage()}%) exceeds 100%.
                Please adjust the tier percentages.
              </p>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        </DialogFooter>
      </DialogContent>

      {/* Add/Edit Tier Dialog */}
      <Dialog
        open={isAddingTier || !!editingTier}
        onOpenChange={() => {
          setIsAddingTier(false)
          setEditingTier(null)
          resetTierForm()
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingTier ? 'Edit Prize Tier' : 'Add Prize Tier'}</DialogTitle>
            <DialogDescription>Configure the prize for this winning tier</DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Tier Number</Label>
                <Input
                  type="number"
                  value={tierForm.tier}
                  onChange={e => setTierForm({ ...tierForm, tier: parseInt(e.target.value) })}
                />
              </div>

              <div className="space-y-2">
                <Label>Matches Required</Label>
                <Input
                  type="number"
                  value={tierForm.matches_required}
                  onChange={e =>
                    setTierForm({ ...tierForm, matches_required: parseInt(e.target.value) })
                  }
                />
              </div>
            </div>

            {/* Bonus Matches field disabled for now - uncomment if needed:
              <div className="space-y-2">
                <Label>Bonus Matches Required (Optional)</Label>
                <Input
                  type="number"
                  value={tierForm.bonus_matches_required || ''}
                  onChange={(e) =>
                    setTierForm({
                      ...tierForm,
                      bonus_matches_required: e.target.value ? parseInt(e.target.value) : undefined,
                    })
                  }
                />
              </div>
            */}

            <div className="space-y-2">
              <Label>Prize Type</Label>
              <Select
                value={tierForm.prize_type}
                onValueChange={(value: 'Fixed' | 'Percentage' | 'PariMutuel') =>
                  setTierForm({ ...tierForm, prize_type: value })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="Fixed">Fixed Amount</SelectItem>
                  <SelectItem value="Percentage">Percentage of Pool</SelectItem>
                  <SelectItem value="PariMutuel">Pari-Mutuel (Variable)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {tierForm.prize_type === 'Fixed' && (
              <div className="space-y-2">
                <Label>Fixed Prize Amount (₵)</Label>
                <Input
                  type="number"
                  value={tierForm.fixed_amount || ''}
                  onChange={e =>
                    setTierForm({ ...tierForm, fixed_amount: parseFloat(e.target.value) })
                  }
                />
              </div>
            )}

            {tierForm.prize_type === 'Percentage' && (
              <div className="space-y-2">
                <Label>Percentage of Prize Pool</Label>
                <div className="flex items-center gap-2">
                  <Input
                    type="number"
                    value={tierForm.percentage || ''}
                    onChange={e =>
                      setTierForm({ ...tierForm, percentage: parseFloat(e.target.value) })
                    }
                    className="flex-1"
                  />
                  <span className="text-muted-foreground">%</span>
                </div>
              </div>
            )}

            {tierForm.prize_type === 'PariMutuel' && (
              <div className="space-y-2">
                <Label>Estimated Prize Value (₵)</Label>
                <Input
                  type="number"
                  value={tierForm.estimated_value || ''}
                  onChange={e =>
                    setTierForm({ ...tierForm, estimated_value: parseFloat(e.target.value) })
                  }
                />
                <p className="text-xs text-muted-foreground">
                  This is an estimate. Actual prize will be calculated based on total winners.
                </p>
              </div>
            )}
          </div>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setIsAddingTier(false)
                setEditingTier(null)
                resetTierForm()
              }}
            >
              Cancel
            </Button>
            <Button onClick={handleSaveTier}>{editingTier ? 'Update' : 'Add'} Tier</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Dialog>
  )
}
