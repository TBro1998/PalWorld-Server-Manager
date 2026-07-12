'use client'

import React from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from './ui/dialog'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { Button } from './ui/button'
import { useTranslations } from '@/contexts/LanguageContext'

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
  const t = useTranslations('addServer')
  const {
    register,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm<CreateServerFormData>({
    resolver: zodResolver(createServerSchema),
    defaultValues: {
      installPath: `Servers/${nextServerId}`,
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
          <DialogTitle>{t('title')}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit(handleFormSubmit)} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">{t('nameLabel')}</Label>
            <Input
              id="name"
              placeholder={t('namePlaceholder')}
              {...register('name')}
            />
            {errors.name && (
              <p className="text-sm text-destructive">{t('nameRequired')}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="installPath">{t('pathLabel')}</Label>
            <Input
              id="installPath"
              placeholder={`Servers/${nextServerId}`}
              {...register('installPath')}
            />
            {errors.installPath && (
              <p className="text-sm text-destructive">{errors.installPath.message}</p>
            )}
            <p className="text-sm text-muted-foreground">
              {t('pathHint')}: Servers/{nextServerId}
            </p>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isLoading}
            >
              {t('cancel')}
            </Button>
            <Button type="submit" disabled={isLoading}>
              {isLoading ? t('creating') : t('create')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
