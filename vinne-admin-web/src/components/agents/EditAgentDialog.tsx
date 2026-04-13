import { useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import * as z from 'zod'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Label } from '@/components/ui/label'
import { useToast } from '@/hooks/use-toast'
import { agentService, type Agent } from '@/services/agents'
import { Loader2, Percent } from 'lucide-react'

// Validation schema
const editAgentSchema = z.object({
  name: z.string().min(1, 'Business name is required'),
  email: z.string().email('Invalid email address'),
  phone_number: z.string().min(10, 'Phone number is required'),
  address: z.string().min(1, 'Address is required'),
  status: z.enum(['ACTIVE', 'SUSPENDED', 'UNDER_REVIEW', 'INACTIVE', 'TERMINATED']),
  commission_percentage: z
    .number()
    .min(0, 'Commission must be at least 0%')
    .max(100, 'Commission cannot exceed 100%'),
})

type EditAgentFormData = z.infer<typeof editAgentSchema>

interface EditAgentDialogProps {
  agent: Agent
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function EditAgentDialog({ agent, open, onOpenChange }: EditAgentDialogProps) {
  const { toast } = useToast()
  const queryClient = useQueryClient()

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    reset,
    formState: { errors },
  } = useForm<EditAgentFormData>({
    resolver: zodResolver(editAgentSchema),
    defaultValues: {
      name: agent.name,
      email: agent.email,
      phone_number: agent.phone_number,
      address: agent.address,
      status: agent.status,
      commission_percentage: agent.commission_percentage || 30,
    },
  })

  // Watch for commission value changes (if needed for future features)
  // const commissionValue = watch('commission_percentage')

  // Reset form when dialog opens with new agent data
  useEffect(() => {
    if (open) {
      reset({
        name: agent.name,
        email: agent.email,
        phone_number: agent.phone_number,
        address: agent.address,
        status: agent.status,
        commission_percentage: agent.commission_percentage || 30,
      })
    }
  }, [open, agent, reset])

  const updateAgentMutation = useMutation({
    mutationFn: (data: Omit<EditAgentFormData, 'status'>) =>
      agentService.updateAgent(agent.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agent', agent.id] })
      queryClient.invalidateQueries({ queryKey: ['agents'] })
      toast({
        title: 'Success',
        description: 'Agent profile updated successfully',
      })
      onOpenChange(false)
    },
    onError: (error: Error) => {
      toast({
        title: 'Error',
        description: error.message || 'Failed to update agent',
        variant: 'destructive',
      })
    },
  })

  const onSubmit = (data: EditAgentFormData) => {
    // Remove status from the update request as it requires a separate endpoint
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { status, ...updateData } = data
    updateAgentMutation.mutate(updateData)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Edit Agent Profile</DialogTitle>
          <DialogDescription>Update agent information and commission settings</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-6">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="name">Business Name</Label>
              <Input
                id="name"
                {...register('name')}
                placeholder="Enter business name"
                className={errors.name ? 'border-red-500' : ''}
              />
              {errors.name && <p className="text-sm text-red-500">{errors.name.message}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                {...register('email')}
                placeholder="email@example.com"
                className={errors.email ? 'border-red-500' : ''}
              />
              {errors.email && <p className="text-sm text-red-500">{errors.email.message}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="phone">Phone Number</Label>
              <Input
                id="phone"
                {...register('phone_number')}
                placeholder="+233501234567"
                className={errors.phone_number ? 'border-red-500' : ''}
              />
              {errors.phone_number && (
                <p className="text-sm text-red-500">{errors.phone_number.message}</p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="status">Status</Label>
              <Select
                value={watch('status')}
                onValueChange={value => setValue('status', value as EditAgentFormData['status'])}
              >
                <SelectTrigger id="status" className={errors.status ? 'border-red-500' : ''}>
                  <SelectValue placeholder="Select status" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="UNDER_REVIEW">Under Review</SelectItem>
                  <SelectItem value="ACTIVE">Active</SelectItem>
                  <SelectItem value="SUSPENDED">Suspended</SelectItem>
                  <SelectItem value="TERMINATED">Terminated</SelectItem>
                </SelectContent>
              </Select>
              {errors.status && <p className="text-sm text-red-500">{errors.status.message}</p>}
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="address">Address</Label>
            <Textarea
              id="address"
              {...register('address')}
              placeholder="Enter full address"
              rows={3}
              className={errors.address ? 'border-red-500' : ''}
            />
            {errors.address && <p className="text-sm text-red-500">{errors.address.message}</p>}
          </div>

          <div className="space-y-2">
            <Label htmlFor="commission">Commission Rate (%)</Label>
            <div className="flex items-center space-x-2">
              <Input
                id="commission"
                type="number"
                min={0}
                max={100}
                {...register('commission_percentage', { valueAsNumber: true })}
                className={`w-24 ${errors.commission_percentage ? 'border-red-500' : ''}`}
                placeholder="0-100"
              />
              <Percent className="h-4 w-4 text-muted-foreground" />
            </div>
            {errors.commission_percentage && (
              <p className="text-sm text-red-500">{errors.commission_percentage.message}</p>
            )}
            <p className="text-sm text-muted-foreground">
              The percentage commission the agent earns on sales (0-100%)
            </p>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={updateAgentMutation.isPending}>
              {updateAgentMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Save Changes
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
