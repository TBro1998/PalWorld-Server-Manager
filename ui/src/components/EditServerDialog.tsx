'use client'

import React, { useEffect, useState } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from './ui/dialog'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { Button } from './ui/button'
import { Switch } from './ui/switch'
import { useTranslations } from '@/contexts/LanguageContext'
import type { Server, UpdateServerData } from '@/types/server'

interface EditServerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  server: Server | null
  onSubmit: (data: UpdateServerData) => void
  isLoading?: boolean
}

export function EditServerDialog({
  open,
  onOpenChange,
  server,
  onSubmit,
  isLoading,
}: EditServerDialogProps) {
  const t = useTranslations('editServer')

  const [name, setName] = useState('')
  const [installPath, setInstallPath] = useState('')
  const [port, setPort] = useState(8211)
  const [queryPort, setQueryPort] = useState(27015)
  const [rconPort, setRconPort] = useState(25575)
  const [rconEnabled, setRconEnabled] = useState(false)

  // Prefill from the selected server whenever the dialog opens.
  useEffect(() => {
    if (server && open) {
      setName(server.name)
      setInstallPath(server.install_path)
      setPort(server.port)
      setQueryPort(server.query_port)
      setRconPort(server.rcon_port)
      setRconEnabled(server.rcon_enabled)
    }
  }, [server, open])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit({
      name,
      installPath,
      port,
      queryPort,
      rconPort,
      rconEnabled,
    })
  }

  const pathChanged = server ? installPath !== server.install_path : false

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('title')}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="edit-name">{t('nameLabel')}</Label>
            <Input id="edit-name" value={name} onChange={(e) => setName(e.target.value)} />
          </div>

          <div className="space-y-2">
            <Label htmlFor="edit-path">{t('pathLabel')}</Label>
            <Input
              id="edit-path"
              value={installPath}
              onChange={(e) => setInstallPath(e.target.value)}
            />
            {pathChanged && (
              <p className="text-sm text-amber-600 dark:text-amber-500">{t('pathChangedHint')}</p>
            )}
          </div>

          <div className="grid grid-cols-3 gap-3">
            <div className="space-y-2">
              <Label htmlFor="edit-port">{t('portLabel')}</Label>
              <Input
                id="edit-port"
                type="number"
                value={port}
                onChange={(e) => setPort(Number(e.target.value))}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="edit-query">{t('queryPortLabel')}</Label>
              <Input
                id="edit-query"
                type="number"
                value={queryPort}
                onChange={(e) => setQueryPort(Number(e.target.value))}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="edit-rcon">{t('rconPortLabel')}</Label>
              <Input
                id="edit-rcon"
                type="number"
                value={rconPort}
                onChange={(e) => setRconPort(Number(e.target.value))}
              />
            </div>
          </div>

          <div className="flex items-center justify-between">
            <Label htmlFor="edit-rcon-enabled">{t('rconEnabledLabel')}</Label>
            <Switch id="edit-rcon-enabled" checked={rconEnabled} onCheckedChange={setRconEnabled} />
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
              {isLoading ? t('saving') : t('save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
