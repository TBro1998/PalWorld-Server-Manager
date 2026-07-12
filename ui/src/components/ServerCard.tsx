'use client'

import React from 'react'
import { Server } from '@/types/server'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from './ui/card'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Play, Square, RotateCw, Trash2, Download } from 'lucide-react'
import { useTranslations } from '@/contexts/LanguageContext'

interface ServerCardProps {
  server: Server
  onInstall: (id: number) => void
  onStart: (id: number) => void
  onStop: (id: number) => void
  onRestart: (id: number) => void
  onDelete: (id: number) => void
}

const statusConfig = {
  stopped: { variant: 'secondary' as const, key: 'statusStopped' },
  running: { variant: 'default' as const, key: 'statusRunning' },
  installing: { variant: 'outline' as const, key: 'statusInstalling' },
  error: { variant: 'destructive' as const, key: 'statusError' },
}

export function ServerCard({
  server,
  onInstall,
  onStart,
  onStop,
  onRestart,
  onDelete,
}: ServerCardProps) {
  const t = useTranslations('servers')
  const status = statusConfig[server.status]

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-lg">{server.name}</CardTitle>
          <Badge variant={status.variant}>{t(status.key)}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-2 text-sm text-gray-600 dark:text-gray-400">
          <div>
            <span className="font-medium">{t('path')}:</span> {server.install_path}
          </div>
          <div>
            <span className="font-medium">{t('port')}:</span> {server.port}
          </div>
        </div>
      </CardContent>
      <CardFooter>
        <div className="flex gap-2 flex-wrap">
          {server.status === 'stopped' && (
            <>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => onInstall(server.id)}
              >
                <Download size={16} className="mr-1" />
                {t('install')}
              </Button>
              <Button
                size="sm"
                variant="default"
                onClick={() => onStart(server.id)}
              >
                <Play size={16} className="mr-1" />
                {t('start')}
              </Button>
            </>
          )}
          {server.status === 'running' && (
            <>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => onStop(server.id)}
              >
                <Square size={16} className="mr-1" />
                {t('stop')}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => onRestart(server.id)}
              >
                <RotateCw size={16} className="mr-1" />
                {t('restart')}
              </Button>
            </>
          )}
          {server.status === 'installing' && (
            <div className="text-sm text-gray-500 dark:text-gray-400 flex items-center">
              <div className="animate-spin mr-2">⏳</div>
              {t('installing')}
            </div>
          )}
          <Button
            size="sm"
            variant="destructive"
            onClick={() => onDelete(server.id)}
            disabled={server.status === 'running' || server.status === 'installing'}
          >
            <Trash2 size={16} className="mr-1" />
            {t('delete')}
          </Button>
        </div>
      </CardFooter>
    </Card>
  )
}
