'use client'

import React from 'react'
import { Server } from '@/types/server'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from './ui/card'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Play, Square, RotateCw, Trash2, Download, Settings, ScrollText, Terminal } from 'lucide-react'
import { useTranslations } from '@/contexts/LanguageContext'

interface ServerCardProps {
  server: Server
  onInstall: (id: number) => void
  onStart: (id: number) => void
  onStop: (id: number) => void
  onRestart: (id: number) => void
  onDelete: (id: number) => void
  onSettings: (server: Server) => void
  onLogs: (server: Server) => void
  onInstallLogs: (server: Server) => void
}

// Parse the game port from the server's launch_args JSON; fall back to 8211.
function parsePort(launchArgs: string): number {
  try {
    const parsed = JSON.parse(launchArgs) as { port?: number }
    if (typeof parsed.port === 'number') return parsed.port
  } catch {
    // ignore malformed launch_args, fall through to default
  }
  return 8211
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
  onSettings,
  onLogs,
  onInstallLogs,
}: ServerCardProps) {
  const t = useTranslations('servers')
  const status = statusConfig[server.status]
  const idle = server.status === 'stopped' || server.status === 'error'
  const needsInstall =
    !server.installed &&
    server.status !== 'installing' &&
    server.status !== 'running'
  const hasError = server.status === 'error' && !!server.last_error

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
            <span className="font-medium">{t('port')}:</span> {parsePort(server.launch_args)}
          </div>
          {needsInstall && (
            <div className="text-amber-600 dark:text-amber-500 font-medium">
              {t('needsInstall')}
            </div>
          )}
          {hasError && (
            <div
              className="text-red-600 dark:text-red-500 break-all"
              title={server.last_error}
            >
              <span className="font-medium">{t('lastErrorLabel')}:</span>{' '}
              {server.last_error}
            </div>
          )}
        </div>
      </CardContent>
      <CardFooter>
        <div className="flex gap-2 flex-wrap">
          {idle && (
            <>
              <Button
                size="sm"
                variant={needsInstall ? 'default' : 'secondary'}
                onClick={() => onInstall(server.id)}
              >
                <Download size={16} className="mr-1" />
                {t('installUpdate')}
              </Button>
              <Button
                size="sm"
                variant="default"
                onClick={() => onStart(server.id)}
                disabled={!server.installed}
              >
                <Play size={16} className="mr-1" />
                {t('start')}
              </Button>
              <Button size="sm" variant="secondary" onClick={() => onSettings(server)}>
                <Settings size={16} className="mr-1" />
                {t('settings')}
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
            <>
              <div className="text-sm text-gray-500 dark:text-gray-400 flex items-center">
                <div className="animate-spin mr-2">⏳</div>
                {t('installing')}
              </div>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => onInstallLogs(server)}
              >
                <Terminal size={16} className="mr-1" />
                {t('installLogs')}
              </Button>
            </>
          )}
          <Button size="sm" variant="secondary" onClick={() => onLogs(server)}>
            <ScrollText size={16} className="mr-1" />
            {t('logs')}
          </Button>
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
