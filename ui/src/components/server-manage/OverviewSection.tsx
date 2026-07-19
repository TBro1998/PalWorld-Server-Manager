'use client'

import { useState } from 'react'
import {
  Gauge, Timer, Signal, Clock, Server as ServerIcon, RefreshCw,
  Megaphone, Save, Power, PowerOff, Play, Square, RotateCw, Download, Terminal,
} from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog'
import { serversApi } from '@/lib/api'
import { getApiErrorMessage } from '@/lib/apiError'
import { useTranslations } from '@/contexts/LanguageContext'
import { useRestStatus } from '@/hooks/useRestStatus'
import { ServerLogsDialog } from '@/components/ServerLogsDialog'
import { SectionShell, PanelCard, Placeholder, useServer, useServerId } from './shared'
import { RestUnavailableNotice } from './RestUnavailableNotice'

type Feedback = { kind: 'success' | 'error'; text: string } | null

// Combined overview + operations section. Shows runtime metrics and server info
// (from the game REST API) alongside process-level lifecycle controls and
// in-game operations (broadcast, save, graceful shutdown, force stop).
export function OverviewSection() {
  const t = useTranslations('serverManage')
  const serverId = useServerId()
  const queryClient = useQueryClient()
  const { status, isAvailable } = useRestStatus(serverId)

  // ── REST metrics ──────────────────────────────────────────────────────────
  const metricsQuery = useQuery({
    queryKey: ['rest-metrics', serverId],
    queryFn: async () => (await serversApi.restMetrics(serverId)).data,
    enabled: isAvailable,
    refetchOnWindowFocus: false,
  })

  const info = status?.info
  const metrics = metricsQuery.data
  const isFetching = metricsQuery.isFetching
  const handleRefresh = () => metricsQuery.refetch()

  const dash = '—'
  const tiles = [
    { icon: Signal, label: t('overview.online'), value: metrics ? `${metrics.currentplayernum}/${metrics.maxplayernum}` : dash },
    { icon: Gauge,  label: t('overview.fps'),    value: metrics ? String(metrics.serverfps) : dash },
    { icon: Clock,  label: t('overview.uptime'), value: metrics ? formatUptime(metrics.uptime) : dash },
    { icon: Timer,  label: t('overview.frametime'), value: metrics ? `${metrics.serverframetime.toFixed(1)} ms` : dash },
  ]

  // ── REST in-game operations state ─────────────────────────────────────────
  const [announceMsg, setAnnounceMsg] = useState('')
  const [waitTime, setWaitTime]       = useState('30')
  const [shutdownMsg, setShutdownMsg] = useState('')
  const [confirm, setConfirm]         = useState<'shutdown' | 'stop' | null>(null)
  const [feedback, setFeedback]       = useState<Feedback>(null)

  const onError = (fallback: string) => (err: unknown) =>
    setFeedback({ kind: 'error', text: getApiErrorMessage(err, fallback) })

  const announceMut = useMutation({
    mutationFn: (message: string) => serversApi.restAnnounce(serverId, { message }),
    onSuccess: () => { setFeedback({ kind: 'success', text: t('operations.feedback.announceOk') }); setAnnounceMsg('') },
    onError: onError(t('operations.feedback.announceFail')),
  })

  const saveMut = useMutation({
    mutationFn: () => serversApi.restSave(serverId),
    onSuccess: () => setFeedback({ kind: 'success', text: t('operations.feedback.saveOk') }),
    onError: onError(t('operations.feedback.saveFail')),
  })

  const shutdownMut = useMutation({
    mutationFn: ({ waittime, message }: { waittime: number; message: string }) =>
      serversApi.restShutdown(serverId, { waittime, message }),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('operations.feedback.shutdownOk') })
      queryClient.invalidateQueries({ queryKey: ['rest-status', serverId] })
    },
    onError: onError(t('operations.feedback.shutdownFail')),
  })

  const stopMut = useMutation({
    mutationFn: () => serversApi.restStop(serverId),
    onSuccess: () => {
      setFeedback({ kind: 'success', text: t('operations.feedback.stopOk') })
      queryClient.invalidateQueries({ queryKey: ['rest-status', serverId] })
    },
    onError: onError(t('operations.feedback.stopFail')),
  })

  const confirmAction = () => {
    if (confirm === 'shutdown') {
      const secs = Number.parseInt(waitTime, 10)
      shutdownMut.mutate({ waittime: Number.isFinite(secs) ? secs : 0, message: shutdownMsg })
    } else if (confirm === 'stop') {
      stopMut.mutate()
    }
    setConfirm(null)
  }

  const restDisabled = !isAvailable

  return (
    <SectionShell title={t('overview.title')} desc={t('overview.desc')} comingSoon={false}>
      {/* Process lifecycle controls — start/stop/restart/update. Independent of
          the REST API (these drive the server process directly). */}
      <LifecycleControls />

      {!isAvailable && <RestUnavailableNotice status={status} />}

      {/* Refresh button + operation feedback on the same row */}
      <div className="flex items-center justify-between gap-3">
        {feedback ? (
          <p className={'text-sm ' + (feedback.kind === 'success' ? 'text-success' : 'text-destructive')}>
            {feedback.text}
          </p>
        ) : <span />}
        <Button
          variant="outline"
          size="sm"
          className="gap-2 rounded-xl border-2 shadow-pal"
          onClick={handleRefresh}
          disabled={!isAvailable || isFetching}
        >
          <RefreshCw className={`h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} />
          {t('overview.refresh')}
        </Button>
      </div>

      {/* Runtime metric tiles */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {tiles.map((m) => {
          const Icon = m.icon
          return (
            <Card key={m.label} className="rounded-2xl border-2 shadow-pal">
              <CardContent className="flex items-center gap-3 p-4">
                <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <Icon className="h-5 w-5" />
                </span>
                <div className="min-w-0">
                  <div className="truncate text-2xl font-extrabold leading-none text-foreground">
                    {m.value}
                  </div>
                  <div className="mt-1 text-xs font-medium text-muted-foreground">{m.label}</div>
                </div>
              </CardContent>
            </Card>
          )
        })}
      </div>

      {/* Static server info from REST /info */}
      <PanelCard icon={<ServerIcon className="h-4 w-4" />} title={t('overview.info')}>
        {info ? (
          <dl className="space-y-2 text-sm">
            <InfoRow label={t('overview.serverName')}  value={info.servername} />
            <InfoRow label={t('overview.version')}     value={info.version} />
            <InfoRow label={t('overview.description')} value={info.description} />
          </dl>
        ) : (
          <Placeholder className="min-h-[140px]">{t('rest.noData')}</Placeholder>
        )}
      </PanelCard>

      {/* In-game REST operations */}
      <div className="grid gap-4 sm:grid-cols-2">
        {/* Broadcast */}
        <PanelCard icon={<Megaphone className="h-4 w-4" />} title={t('operations.broadcast')}>
          <div className="space-y-3">
            <Textarea
              value={announceMsg}
              onChange={(e) => setAnnounceMsg(e.target.value)}
              placeholder={t('operations.broadcastPlaceholder')}
              disabled={restDisabled}
              className="min-h-[72px]"
            />
            <Button
              size="sm"
              className="w-full"
              onClick={() => announceMut.mutate(announceMsg)}
              disabled={restDisabled || !announceMsg.trim() || announceMut.isPending}
            >
              {t('operations.broadcastSend')}
            </Button>
          </div>
        </PanelCard>

        {/* Save world */}
        <PanelCard icon={<Save className="h-4 w-4" />} title={t('operations.save')}>
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">{t('operations.saveDesc')}</p>
            <Button
              size="sm"
              variant="outline"
              className="w-full"
              onClick={() => saveMut.mutate()}
              disabled={restDisabled || saveMut.isPending}
            >
              {t('operations.saveAction')}
            </Button>
          </div>
        </PanelCard>

        {/* Graceful shutdown */}
        <PanelCard icon={<Power className="h-4 w-4" />} title={t('operations.shutdown')}>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="shutdown-wait">{t('operations.shutdownWait')}</Label>
              <Input
                id="shutdown-wait"
                type="number"
                min={0}
                value={waitTime}
                onChange={(e) => setWaitTime(e.target.value)}
                disabled={restDisabled}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="shutdown-msg">{t('operations.shutdownMessage')}</Label>
              <Input
                id="shutdown-msg"
                value={shutdownMsg}
                onChange={(e) => setShutdownMsg(e.target.value)}
                placeholder={t('operations.shutdownMessagePlaceholder')}
                disabled={restDisabled}
              />
            </div>
            <Button
              size="sm"
              variant="outline"
              className="w-full"
              onClick={() => setConfirm('shutdown')}
              disabled={restDisabled || shutdownMut.isPending}
            >
              {t('operations.shutdownAction')}
            </Button>
          </div>
        </PanelCard>

        {/* Immediate stop */}
        <PanelCard icon={<PowerOff className="h-4 w-4" />} title={t('operations.stop')}>
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">{t('operations.stopDesc')}</p>
            <Button
              size="sm"
              variant="destructive"
              className="w-full"
              onClick={() => setConfirm('stop')}
              disabled={restDisabled || stopMut.isPending}
            >
              {t('operations.stopAction')}
            </Button>
          </div>
        </PanelCard>
      </div>

      {/* Confirmation dialog for destructive shutdown / force-stop */}
      <Dialog open={confirm !== null} onOpenChange={(o) => !o && setConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {confirm === 'shutdown'
                ? t('operations.confirm.shutdownTitle')
                : t('operations.confirm.stopTitle')}
            </DialogTitle>
            <DialogDescription>
              {confirm === 'shutdown'
                ? t('operations.confirm.shutdownDesc')
                : t('operations.confirm.stopDesc')}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setConfirm(null)}>
              {t('operations.confirm.cancel')}
            </Button>
            <Button type="button" variant="destructive" onClick={confirmAction}>
              {t('operations.confirm.confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SectionShell>
  )
}

// LifecycleControls drives the server process (start / stop / restart /
// install-update) from the manage page. These are process-level operations
// and do NOT depend on the REST API.
function LifecycleControls() {
  const t  = useTranslations('servers')
  const tm = useTranslations('serverManage')
  const serverId = useServerId()
  const queryClient = useQueryClient()
  const { data: server } = useServer()
  const [installLogsOpen, setInstallLogsOpen] = useState(false)

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['server', serverId] })
    queryClient.invalidateQueries({ queryKey: ['servers'] })
  }

  const startMut   = useMutation({ mutationFn: () => serversApi.start(serverId),   onSuccess: invalidate })
  const stopMut    = useMutation({ mutationFn: () => serversApi.stop(serverId),    onSuccess: invalidate })
  const restartMut = useMutation({ mutationFn: () => serversApi.restart(serverId), onSuccess: invalidate })
  const installMut = useMutation({
    mutationFn: () => serversApi.install(serverId),
    onSuccess: () => { setInstallLogsOpen(true); invalidate() },
  })

  if (!server) return null

  const idle        = server.status === 'stopped' || server.status === 'error'
  const needsInstall = !server.installed && server.status !== 'installing' && server.status !== 'running'
  const busy        = startMut.isPending || stopMut.isPending || restartMut.isPending || installMut.isPending

  return (
    <PanelCard icon={<Power className="h-4 w-4" />} title={tm('operations.control')}>
      <div className="flex flex-wrap gap-2">
        {needsInstall && (
          <Button size="sm" onClick={() => installMut.mutate()} disabled={busy}>
            <Download size={16} className="mr-1" />
            {t('installUpdate')}
          </Button>
        )}
        {idle && !needsInstall && (
          <Button size="sm" onClick={() => startMut.mutate()} disabled={busy}>
            <Play size={16} className="mr-1" />
            {t('start')}
          </Button>
        )}
        {server.status === 'running' && (
          <>
            <Button size="sm" variant="secondary" onClick={() => stopMut.mutate()} disabled={busy}>
              <Square size={16} className="mr-1" />
              {t('stop')}
            </Button>
            <Button size="sm" variant="secondary" onClick={() => restartMut.mutate()} disabled={busy}>
              <RotateCw size={16} className="mr-1" />
              {t('restart')}
            </Button>
          </>
        )}
        {server.status === 'installing' && (
          <div className="flex items-center gap-2">
            <span className="flex items-center text-sm text-muted-foreground">
              <span className="mr-2 animate-spin">⏳</span>
              {t('installing')}
            </span>
            <Button size="sm" variant="outline" onClick={() => setInstallLogsOpen(true)}>
              <Terminal size={16} className="mr-1" />
              {t('installLogs')}
            </Button>
          </div>
        )}
        {/* Update button shown for idle, already-installed servers. */}
        {idle && !needsInstall && (
          <Button size="sm" variant="outline" onClick={() => installMut.mutate()} disabled={busy}>
            <Download size={16} className="mr-1" />
            {t('installUpdate')}
          </Button>
        )}
      </div>

      <ServerLogsDialog
        open={installLogsOpen}
        onOpenChange={setInstallLogsOpen}
        server={server}
        kind="steamcmd"
      />
    </PanelCard>
  )
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <dt className="shrink-0 text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-words text-right font-medium text-foreground">
        {value || '—'}
      </dd>
    </div>
  )
}

// formatUptime turns uptime in seconds into a compact d/h/m/s label.
function formatUptime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}
