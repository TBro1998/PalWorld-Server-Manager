'use client'

import React, { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Server } from '@/types/server'
import { Card, CardContent, CardFooter, CardHeader } from './ui/card'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Play, Square, RotateCw, Trash2, Download, Settings, ScrollText, Terminal, Eye, EyeOff, Server as ServerIcon } from 'lucide-react'
import { serversApi } from '@/lib/api'
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

// Status → semantic badge variant + status-dot color for the Palworld palette.
const statusConfig = {
  stopped: { variant: 'secondary' as const, key: 'statusStopped', dot: 'bg-muted-foreground' },
  running: { variant: 'success' as const, key: 'statusRunning', dot: 'bg-success' },
  installing: { variant: 'info' as const, key: 'statusInstalling', dot: 'bg-info animate-pulse' },
  error: { variant: 'destructive' as const, key: 'statusError', dot: 'bg-destructive' },
}

// SecretValue renders a password masked by default with a show/hide toggle.
// Shows the `empty` placeholder when no value is set.
function SecretValue({ value, empty }: { value: string; empty: string }) {
  const [show, setShow] = useState(false)
  if (!value) return <span className="text-muted-foreground">{empty}</span>
  return (
    <span className="inline-flex items-center gap-1">
      <span className="font-mono">{show ? value : '••••••'}</span>
      <button
        type="button"
        onClick={() => setShow((s) => !s)}
        className="text-muted-foreground transition-colors hover:text-foreground"
      >
        {show ? <EyeOff size={14} /> : <Eye size={14} />}
      </button>
    </span>
  )
}

// One label/value line in the info block.
function InfoRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <span className="shrink-0 text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate text-right font-medium text-foreground">{children}</span>
    </div>
  )
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
  const tc = useTranslations('serverConfig')
  const status = statusConfig[server.status]

  // Pull the INI config so the card can surface the basics-tab fields. Only
  // meaningful once installed; backend returns registry defaults otherwise.
  const { data: config } = useQuery({
    queryKey: ['serverConfig', server.id],
    queryFn: async () => (await serversApi.getConfig(server.id)).data,
    enabled: server.installed,
    staleTime: 30_000,
  })
  const settings = config?.settings ?? {}
  const idle = server.status === 'stopped' || server.status === 'error'
  const needsInstall =
    !server.installed &&
    server.status !== 'installing' &&
    server.status !== 'running'
  const hasError = server.status === 'error' && !!server.last_error

  return (
    <Card className="flex flex-col overflow-hidden rounded-2xl border-2 shadow-pal transition-transform hover:-translate-y-1">
      <CardHeader className="gap-0 pb-4">
        <div className="flex items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2.5">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <ServerIcon className="h-5 w-5" />
            </div>
            <h3 className="truncate text-lg font-bold leading-tight text-foreground">
              {server.name}
            </h3>
          </div>
          <Badge variant={status.variant} className="shrink-0 gap-1.5">
            <span className={`h-1.5 w-1.5 rounded-full ${status.dot}`} />
            {t(status.key)}
          </Badge>
        </div>
      </CardHeader>

      <CardContent className="flex-1">
        <div className="space-y-2 rounded-xl bg-muted/40 p-3 text-sm">
          <InfoRow label={t('path')}>{server.install_path}</InfoRow>
          <InfoRow label={t('port')}>{parsePort(server.launch_args)}</InfoRow>
          {config && (
            <>
              <InfoRow label={tc('params.ServerDescription.label')}>
                {settings.ServerDescription || (
                  <span className="text-muted-foreground">{t('notSet')}</span>
                )}
              </InfoRow>
              <InfoRow label={tc('params.ServerPassword.label')}>
                <SecretValue value={settings.ServerPassword ?? ''} empty={t('notSet')} />
              </InfoRow>
              <InfoRow label={tc('params.AdminPassword.label')}>
                <SecretValue value={settings.AdminPassword ?? ''} empty={t('notSet')} />
              </InfoRow>
              <InfoRow label="REST API">
                {settings.RESTAPIEnabled === 'True'
                  ? `${t('enabled')} · ${t('port')} ${settings.RESTAPIPort || '8212'}`
                  : t('disabled')}
              </InfoRow>
            </>
          )}
        </div>

        {needsInstall && (
          <div className="mt-3 rounded-lg bg-warning/15 px-3 py-2 text-sm font-medium text-warning">
            {t('needsInstall')}
          </div>
        )}
        {hasError && (
          <div
            className="mt-3 break-all rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive"
            title={server.last_error}
          >
            <span className="font-semibold">{t('lastErrorLabel')}:</span>{' '}
            {server.last_error}
          </div>
        )}
      </CardContent>

      <CardFooter className="flex-wrap gap-2 border-t border-border bg-secondary/30 pt-4">
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
            <Button size="sm" variant="secondary" onClick={() => onStop(server.id)}>
              <Square size={16} className="mr-1" />
              {t('stop')}
            </Button>
            <Button size="sm" variant="secondary" onClick={() => onRestart(server.id)}>
              <RotateCw size={16} className="mr-1" />
              {t('restart')}
            </Button>
          </>
        )}
        {server.status === 'installing' && (
          <>
            <div className="flex items-center text-sm text-muted-foreground">
              <div className="mr-2 animate-spin">⏳</div>
              {t('installing')}
            </div>
            <Button size="sm" variant="secondary" onClick={() => onInstallLogs(server)}>
              <Terminal size={16} className="mr-1" />
              {t('installLogs')}
            </Button>
          </>
        )}
        <Button size="sm" variant="secondary" onClick={() => onLogs(server)}>
          <ScrollText size={16} className="mr-1" />
          {t('logs')}
        </Button>
        <div className="ml-auto">
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
