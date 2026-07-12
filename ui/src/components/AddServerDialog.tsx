import React from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from './ui/dialog'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { Button } from './ui/button'

const createServerSchema = z.object({
  name: z.string().min(1, 'Server name is required'),
  installPath: z.string().optional(),
})

type CreateServerFormData = z.infer<typeof createServerSchema>

interface AddServerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (data: CreateServerFormData) => void
  isLoading?: boolean
  nextServerId?: number
}

export function AddServerDialog({
  open,
  onOpenChange,
  onSubmit,
  isLoading,
  nextServerId = 1,
}: AddServerDialogProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm<CreateServerFormData>({
    resolver: zodResolver(createServerSchema),
    defaultValues: {
      installPath: `Server/${nextServerId}`,
    },
  })

  const handleFormSubmit = (data: CreateServerFormData) => {
    onSubmit(data)
    reset()
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add New Server</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit(handleFormSubmit)} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Server Name</Label>
            <Input
              id="name"
              placeholder="My Palworld Server"
              {...register('name')}
            />
            {errors.name && (
              <p className="text-sm text-destructive">{errors.name.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="installPath">Install Path</Label>
            <Input
              id="installPath"
              placeholder={`Server/${nextServerId}`}
              {...register('installPath')}
            />
            {errors.installPath && (
              <p className="text-sm text-destructive">{errors.installPath.message}</p>
            )}
            <p className="text-sm text-muted-foreground">
              Leave install path empty to use default: Server/{nextServerId}
            </p>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isLoading}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isLoading}>
              {isLoading ? 'Creating...' : 'Create Server'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
