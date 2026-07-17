'use client'

import React, { useState } from 'react'
import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import { Server } from '@/types/server'
import { Card, CardContent, CardFooter, CardHeader } from './ui/card'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Play, Square, RotateCw, Trash2, Download, SlidersHorizontal, Terminal, Eye, EyeOff, Server as ServerIcon, Plug } from 'lucide-react'
import { serversApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'

interface ServerCardProps {
  server: Server
  onInstall: (id: number) => void
  onStart: (id: number) => void
  onStop: (id: number) => void
  onRestart: (id: number) => void
  onDelete: (id: number) => void
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
  running: { variant: 'success' as const, key: 'statusRunning', dot: 'bg-success animate-pulse' },
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

// StatTile surfaces a single high-value fact (port, REST API) as a compact,
// scannable card so the essentials pop above the detail rows.
function StatTile({
  icon,
  label,
  children,
}: {
  icon: React.ReactNode
  label: string
  children: React.ReactNode
}) {
  return (
    <div className="flex min-w-0 items-center gap-2 rounded-xl border border-border/60 bg-background/60 px-3 py-2">
      <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
        {icon}
      </span>
      <div className="min-w-0">
        <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          {label}
        </p>
        <p className="truncate text-sm font-bold leading-tight text-foreground">{children}</p>
      </div>
    </div>
  )
}

// One label/value line in the detail block.
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
  const restEnabled = settings.RESTAPIEnabled === 'True'

  return (
    <Card className="flex flex-col overflow-hidden rounded-2xl border-2 shadow-pal transition-all duration-200 hover:-translate-y-1 hover:shadow-pal-lg">
      {/* Accent header strip keeps the status colour reading at a glance. */}
      <CardHeader className="gap-0 border-b border-border/60 bg-gradient-to-br from-secondary/40 to-transparent pb-4">
        <div className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <ServerIcon className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <h3 className="truncate text-lg font-bold leading-tight text-foreground">
                {server.name}
              </h3>
              <p
                className="truncate font-mono text-xs text-muted-foreground"
                title={server.install_path}
              >
                {server.install_path}
              </p>
            </div>
          </div>
          <Badge variant={status.variant} className="shrink-0 gap-1.5">
            <span className={`h-1.5 w-1.5 rounded-full ${status.dot}`} />
            {t(status.key)}
          </Badge>
        </div>
      </CardHeader>

      <CardContent className="flex-1 pt-4">
        {/* Key facts as scannable tiles; collapse to one column when cramped. */}
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <StatTile icon={<Plug className="h-4 w-4" />} label={t('port')}>
            {parsePort(server.launch_args)}
          </StatTile>
          <StatTile icon={<Terminal className="h-4 w-4" />} label="REST API">
            {config
              ? restEnabled
                ? `${t('enabled')} · ${settings.RESTAPIPort || '8212'}`
                : t('disabled')
              : '—'}
          </StatTile>
        </div>

        {config && (
          <div className="mt-3 space-y-2 rounded-xl bg-muted/40 p-3 text-sm">
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
          </div>
        )}

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

      <CardFooter className="flex-wrap items-center gap-2 border-t border-border bg-secondary/30 pt-4">
        {/* Primary lifecycle action — a single prominent button per state. */}
        {needsInstall && (
          <Button size="sm" variant="default" onClick={() => onInstall(server.id)}>
            <Download size={16} className="mr-1" />
            {t('installUpdate')}
          </Button>
        )}
        {idle && !needsInstall && (
          <Button size="sm" variant="default" onClick={() => onStart(server.id)}>
            <Play size={16} className="mr-1" />
            {t('start')}
          </Button>
        )}
        {server.status === 'running' && (
          <>
            <Button size="sm" variant="default" onClick={() => onStop(server.id)}>
              <Square size={16} className="mr-1" />
              {t('stop')}
            </Button>
            <Button size="sm" variant="outline" onClick={() => onRestart(server.id)}>
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
            <Button size="sm" variant="outline" onClick={() => onInstallLogs(server)}>
              <Terminal size={16} className="mr-1" />
              {t('installLogs')}
            </Button>
          </>
        )}

        {/* Secondary: re-install/update stays available for idle installed
            servers, de-emphasised next to the primary Start. */}
        {idle && !needsInstall && (
          <Button size="sm" variant="outline" onClick={() => onInstall(server.id)}>
            <Download size={16} className="mr-1" />
            {t('installUpdate')}
          </Button>
        )}

        {/* Manage is the gateway to settings, logs and everything else. */}
        <Link href={`/servers/manage?id=${server.id}`} prefetch={false} className="ml-auto">
          <Button size="sm" variant="secondary">
            <SlidersHorizontal size={16} className="mr-1" />
            {t('manage')}
          </Button>
        </Link>
        <Button
          size="sm"
          variant="ghost"
          className="text-destructive hover:bg-destructive/10 hover:text-destructive"
          onClick={() => onDelete(server.id)}
          disabled={server.status === 'running' || server.status === 'installing'}
          aria-label={t('delete')}
        >
          <Trash2 size={16} />
        </Button>
      </CardFooter>
    </Card>
  )
}
